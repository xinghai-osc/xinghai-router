//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIntegrationMiniMaxVideoLifecycle(t *testing.T) {
	var creates, queries, deletes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-video-original" {
			t.Errorf("wrong upstream credential")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == minimaxVideoPath:
			n := creates.Add(1)
			body, _ := io.ReadAll(r.Body)
			request, err := validateMiniMaxVideoRequest(body)
			if err != nil || request.Model != "MiniMax-H3" || request.Duration != 5 || !strings.Contains(string(body), `"text":"A cat running"`) {
				t.Errorf("upstream body=%s err=%v", body, err)
			}
			_, _ = fmt.Fprintf(w, `{"task_id":"upstream_%d"}`, n)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/query/video_generation/upstream_"):
			queries.Add(1)
			id := strings.TrimPrefix(r.URL.Path, "/v2/query/video_generation/")
			_, _ = fmt.Fprintf(w, `{"task":{"id":%q,"model":"MiniMax-H3","status":"succeeded","duration":5,"content":{"url":"https://cdn.example.test/video.mp4"},"usage":{"total_seconds":5}}}`, id)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, minimaxVideoPath+"/upstream_"):
			deletes.Add(1)
			id := strings.TrimPrefix(r.URL.Path, minimaxVideoPath+"/")
			_, _ = fmt.Fprintf(w, `{"task_id":%q,"action":"deleted","status":"deleted"}`, id)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	s, db, secret, key, ch := videoIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `update channels set provider='minimax',upstream_format='',models='["public-video"]' where id=$1`, ch.id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into model_routes(channel_id,public_model,upstream_model) values($1,'public-video','MiniMax-H3')`, ch.id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million,dimension_prices) values(gen_random_uuid(),'public-video',0,0,'{"video_per_second":0.1}')`); err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(miniMaxVideoTestBody, "MiniMax-H3", "public-video", 1)
	created := billingIntegrationCall(s, secret, minimaxVideoPath, body)
	if created.Code != 200 {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var result struct {
		TaskID string `json:"task_id"`
	}
	if json.Unmarshal(created.Body.Bytes(), &result) != nil || !strings.HasPrefix(result.TaskID, "video_") {
		t.Fatalf("create=%s", created.Body.String())
	}
	var protocol, original, cost string
	if err := db.QueryRow(ctx, `select upstream_provider,upstream_id from video_tasks where id=$1`, result.TaskID).Scan(&protocol, &original); err != nil || protocol != "minimax" || original != "upstream_1" {
		t.Fatalf("task protocol=%s original=%s err=%v", protocol, original, err)
	}
	if err := db.QueryRow(ctx, `select cost::text from usage_records where request_id=$1`, created.Header().Get("X-Request-ID")).Scan(&cost); err != nil || cost != "0.00000000" {
		t.Fatalf("cost=%s err=%v", cost, err)
	}
	otherSecret := "sk-xh-minimax-other-key"
	integrationKey(t, db, key.userID, otherSecret)
	if response := videoIntegrationGet(s, otherSecret, "/v2/query/video_generation/"+result.TaskID, ""); response.Code != 404 {
		t.Fatalf("other key status=%d", response.Code)
	}
	if response := videoIntegrationGet(s, secret, "/v2/query/video_generation/upstream_1", ""); response.Code != 404 {
		t.Fatalf("raw ID status=%d", response.Code)
	}
	if queries.Load() != 0 {
		t.Fatal("unauthorized lookup reached upstream")
	}
	restarted := integrationService(t, db)
	for _, path := range []string{"/v2/query/video_generation/", "/v1/videos/"} {
		response := videoIntegrationGet(restarted, secret, path+result.TaskID, "")
		if response.Code != 200 || strings.Contains(response.Body.String(), "upstream_1") || !strings.Contains(response.Body.String(), result.TaskID) {
			t.Fatalf("query=%d %s", response.Code, response.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodDelete, minimaxVideoPath+"/"+result.TaskID, nil)
	r.Header.Set("Authorization", "Bearer "+secret)
	deleted := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(deleted, r)
	if deleted.Code != 200 || deletes.Load() != 1 || !strings.Contains(deleted.Body.String(), result.TaskID) {
		t.Fatalf("delete=%d %s", deleted.Code, deleted.Body.String())
	}
	var usages int
	if err := db.QueryRow(ctx, `select count(*) from usage_records`).Scan(&usages); err != nil || usages != 1 {
		t.Fatalf("repeated billing=%d err=%v", usages, err)
	}
	compat := billingIntegrationCall(s, secret, "/v1/videos", `{"model":"public-video","prompt":"A cat running","seconds":"5","size":"1280x720"}`)
	if compat.Code != 200 || creates.Load() != 2 || !strings.Contains(compat.Body.String(), `"status":"queued"`) {
		t.Fatalf("compat=%d %s", compat.Code, compat.Body.String())
	}
	if _, err := db.Exec(ctx, `update channel_api_keys set key_encrypted='rotated-key' where id=$1`, ch.keyID); err != nil {
		t.Fatal(err)
	}
	before := queries.Load()
	if response := videoIntegrationGet(restarted, secret, "/v2/query/video_generation/"+result.TaskID, ""); response.Code != 503 || queries.Load() != before {
		t.Fatalf("credential replacement status=%d", response.Code)
	}
}

func TestIntegrationMiniMaxVideoIsolationAndFailure(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"error":{"type":"insufficient_balance_error","message":"failed"}}`)
	}))
	defer upstream.Close()
	s, db, secret, _, ch := videoIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `update channels set models='["MiniMax-H3"]' where id=$1`, ch.id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,output_per_million,dimension_prices) values(gen_random_uuid(),'MiniMax-H3',0,0,'{"video_per_second":0.1}')`); err != nil {
		t.Fatal(err)
	}
	response := billingIntegrationCall(s, secret, minimaxVideoPath, miniMaxVideoTestBody)
	if response.Code < 400 || calls.Load() != 0 {
		t.Fatalf("native reached non-MiniMax: %d calls=%d", response.Code, calls.Load())
	}
	if _, err := db.Exec(ctx, `update channels set provider='minimax',upstream_format='' where id=$1`, ch.id); err != nil {
		t.Fatal(err)
	}
	s.invalidateChannels()
	response = billingIntegrationCall(s, secret, minimaxVideoPath, miniMaxVideoTestBody)
	if response.Code != 502 || calls.Load() != 1 {
		t.Fatalf("business error accepted/retried: %d calls=%d body=%s", response.Code, calls.Load(), response.Body.String())
	}
	var tasks, usages int
	if err := db.QueryRow(ctx, `select count(*) from video_tasks`).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `select count(*) from usage_records`).Scan(&usages); err != nil {
		t.Fatal(err)
	}
	if tasks != 0 || usages != 0 {
		t.Fatalf("failed request persisted or billed: tasks=%d usage=%d", tasks, usages)
	}
}
