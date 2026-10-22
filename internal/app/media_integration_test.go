//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func mediaIntegrationPost(s *Service, secret, path string, body []byte, ct string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestIntegrationMediaSpeechBinaryAndPricing(t *testing.T) {
	var calls atomic.Int64
	audio := []byte{0, 1, 255, 254, 0, 42}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/audio/speech" || r.Header.Get("Authorization") != "Bearer fake-upstream-key" {
			t.Errorf("bad upstream request path=%s", r.URL.Path)
		}
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			t.Error(err)
		}
		if fields["model"] != "speech-upstream" || fields["input"] != "你好🌊 A" {
			t.Errorf("fields=%v", fields)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write(audio)
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `insert into model_routes(id,channel_id,public_model,upstream_model) select gen_random_uuid(),id,'speech-model','speech-upstream' from channels where name='billing'; insert into pricing_rules(id,model,input_per_million,output_per_million,dimension_prices) values(gen_random_uuid(),'speech-model',0,0,'{"audio_per_million_characters":20}')`); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"model":"speech-model","input":"你好🌊 A","voice":"alloy"}`)
	got := mediaIntegrationPost(s, secret, "/v1/audio/speech", body, "application/json")
	if got.Code != 200 || got.Header().Get("Content-Type") != "audio/mpeg" || !bytes.Equal(got.Body.Bytes(), audio) {
		t.Fatalf("status=%d headers=%v body=%v", got.Code, got.Header(), got.Body.Bytes())
	}
	var cost string
	var facts UsageFacts
	var raw []byte
	if err := db.QueryRow(ctx, `select cost::text,usage_facts from usage_records where request_id=$1`, got.Header().Get("X-Request-ID")).Scan(&cost, &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &facts); err != nil {
		t.Fatal(err)
	}
	if cost != "0.00010000" || facts.AudioCharacters != 5 {
		t.Fatalf("cost=%s facts=%+v", cost, facts)
	}
	if _, err := db.Exec(ctx, `update pricing_rules set dimension_prices='{}' where model='speech-model'`); err != nil {
		t.Fatal(err)
	}
	s.pricingCache.clear()
	got = mediaIntegrationPost(s, secret, "/v1/audio/speech", body, "application/json")
	if got.Code != 402 || calls.Load() != 1 {
		t.Fatalf("unpriced status=%d calls=%d body=%s", got.Code, calls.Load(), got.Body.String())
	}
}

func TestIntegrationMediaTranscriptionReservation(t *testing.T) {
	var calls atomic.Int64
	var excessive atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/audio/transcriptions" && r.URL.Path != "/v1/audio/translations" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("model") != "audio-upstream" || r.FormValue("max_audio_seconds") != "" {
			t.Errorf("unexpected fields=%v", r.MultipartForm.Value)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if len(data) == 0 {
			t.Error("empty audio forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		if excessive.Load() {
			_, _ = io.WriteString(w, `{"text":"too long","duration":61}`)
			return
		}
		_, _ = io.WriteString(w, `{"text":"hello","duration":2.5}`)
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `insert into model_routes(id,channel_id,public_model,upstream_model) select gen_random_uuid(),id,'audio-model','audio-upstream' from channels where name='billing'; insert into pricing_rules(id,model,input_per_million,output_per_million,dimension_prices) values(gen_random_uuid(),'audio-model',0,0,'{"audio_per_second":0.1}'); update user_wallets set balance=1`); err != nil {
		t.Fatal(err)
	}
	body, ct := mediaTestMultipart(t, [][2]string{{"model", "audio-model"}, {"response_format", "verbose_json"}, {"max_audio_seconds", "60"}}, "file")
	ct = strings.Replace(ct, "multipart/form-data", "Multipart/Form-Data", 1)
	got := mediaIntegrationPost(s, secret, "/v1/audio/transcriptions", body, ct)
	if got.Code != 402 || calls.Load() != 0 {
		t.Fatalf("low-balance status=%d calls=%d body=%s", got.Code, calls.Load(), got.Body.String())
	}
	if _, err := db.Exec(ctx, `update user_wallets set balance=100`); err != nil {
		t.Fatal(err)
	}
	got = mediaIntegrationPost(s, secret, "/v1/audio/transcriptions", body, ct)
	if got.Code != 200 {
		t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
	}
	var cost, reserved string
	if err := db.QueryRow(ctx, `select cost::text from usage_records where request_id=$1`, got.Header().Get("X-Request-ID")).Scan(&cost); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `select reserved::text from user_wallets where user_id=(select user_id from api_keys where secret_hash=$1)`, hashSecret(secret)).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if cost != "0.25000000" || reserved != cost {
		t.Fatalf("cost=%s reserved=%s", cost, reserved)
	}
	got = mediaIntegrationPost(s, secret, "/v1/audio/translations", body, ct)
	if got.Code != 200 {
		t.Fatalf("translation status=%d body=%s", got.Code, got.Body.String())
	}
	excessive.Store(true)
	got = mediaIntegrationPost(s, secret, "/v1/audio/transcriptions", body, ct)
	if got.Code != 502 {
		t.Fatalf("excessive status=%d body=%s", got.Code, got.Body.String())
	}
	if err := db.QueryRow(ctx, `select reserved::text from user_wallets where user_id=(select user_id from api_keys where secret_hash=$1)`, hashSecret(secret)).Scan(&reserved); err != nil || reserved != "0.50000000" {
		t.Fatalf("failed request hold reserved=%s err=%v", reserved, err)
	}
}

func TestIntegrationMediaVideoCreateAndSingleCharge(t *testing.T) {
	var posts atomic.Int64
	var fail atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts.Add(1)
			if fail.Load() {
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"error":"unavailable"}`)
				return
			}
			var fields map[string]any
			if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
				t.Error(err)
			}
			if r.URL.Path != "/v1/videos" || fields["seconds"] != "4" {
				t.Errorf("path=%s fields=%v", r.URL.Path, fields)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"id":"upstream_task","status":"queued","seconds":"4"}`)
			return
		}
		if r.URL.Path != "/v1/videos/upstream_task" {
			t.Errorf("get path=%s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"upstream_task","status":"completed","seconds":"4"}`)
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `insert into model_routes(id,channel_id,public_model,upstream_model) select gen_random_uuid(),id,'video-model','video-upstream' from channels where name='billing'; insert into pricing_rules(id,model,input_per_million,output_per_million,dimension_prices) values(gen_random_uuid(),'video-model',0,0,'{"video_per_second":0.2}')`); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"model":"video-model","prompt":"ocean waves"}`)
	got := mediaIntegrationPost(s, secret, "/v1/videos", body, "application/json")
	if got.Code != 202 {
		t.Fatalf("create status=%d body=%s", got.Code, got.Body.String())
	}
	var task struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(task.ID, "video_") || task.ID == "upstream_task" {
		t.Fatalf("id=%s", task.ID)
	}
	for i := 0; i < 2; i++ {
		got := videoIntegrationGet(s, secret, "/v1/videos/"+task.ID, "")
		if got.Code != 200 || !strings.Contains(got.Body.String(), task.ID) {
			t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
		}
	}
	var count int
	var cost string
	if err := db.QueryRow(ctx, `select count(*),coalesce(sum(cost),0)::numeric(20,8)::text from usage_records`).Scan(&count, &cost); err != nil {
		t.Fatal(err)
	}
	if count != 1 || cost != "0.80000000" {
		t.Fatalf("count=%d cost=%s", count, cost)
	}
	fail.Store(true)
	got = mediaIntegrationPost(s, secret, "/v1/videos", body, "application/json")
	if got.Code != 503 || posts.Load() != 2 {
		t.Fatalf("failed create status=%d posts=%d", got.Code, posts.Load())
	}
	var reserved string
	if err := db.QueryRow(ctx, `select reserved::text from user_wallets where user_id=(select user_id from api_keys where secret_hash=$1)`, hashSecret(secret)).Scan(&reserved); err != nil || reserved != "0.80000000" {
		t.Fatalf("reserved=%s err=%v", reserved, err)
	}
}
