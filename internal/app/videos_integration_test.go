//go:build integration

package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func videoIntegrationFixture(t *testing.T, base string) (*Service, *pgxpool.Pool, string, keyContext, channel) {
	t.Helper()
	s, db, secret := billingIntegrationFixture(t, base)
	key, err := s.loadAPIKeyContext(context.Background(), secret)
	if err != nil {
		t.Fatal(err)
	}
	ch := channel{baseURL: base, upstreamModel: "upstream-video", apiKey: "fake-video-original"}
	if err := db.QueryRow(context.Background(), `select id from channels where name='billing'`).Scan(&ch.id); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(context.Background(), `insert into channel_api_keys(channel_id,name,key_encrypted) values($1,'original','fake-video-original') returning id`, ch.id).Scan(&ch.keyID); err != nil {
		t.Fatal(err)
	}
	return s, db, secret, key, ch
}

func videoIntegrationGet(s *Service, secret, path, byteRange string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if secret != "" {
		r.Header.Set("Authorization", "Bearer "+secret)
	}
	if byteRange != "" {
		r.Header.Set("Range", byteRange)
		r.Header.Set("If-Range", `"video-etag"`)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestIntegrationVideoPersistenceOwnershipAndContent(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fake-video-original" {
			t.Errorf("unexpected upstream credential")
		}
		switch r.URL.Path {
		case "/v1/videos/video_durable":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"video_durable","status":"completed","progress":100}`)
		case "/v1/videos/video_durable/content":
			if r.Header.Get("Range") != "bytes=2-5" || r.Header.Get("If-Range") != `"video-etag"` || r.URL.RawQuery != "variant=video" {
				t.Errorf("incorrect range/query forwarding: %s %s", r.Header.Get("Range"), r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Range", "bytes 2-5/8")
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Disposition", `attachment; filename="clip.mp4"`)
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0, 1, 254, 255})
		default:
			t.Errorf("unexpected path=%s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	s, db, secret, key, ch := videoIntegrationFixture(t, upstream.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err := s.registerVideoTask(ctx, key, ch, "video-model", []byte(`{"id":"video_durable","status":"queued"}`))
	if err != nil {
		t.Fatalf("detached persistence: %v", err)
	}
	localID, _, err := parseVideoTask(response)
	if err != nil || localID == "video_durable" || !strings.HasPrefix(localID, "video_") {
		t.Fatalf("local ID response=%s err=%v", response, err)
	}

	var model, upstreamModel, channelKey, workspace string
	if err := db.QueryRow(context.Background(), `select model,upstream_model,channel_key_id::text,workspace_id::text from video_tasks where upstream_id='video_durable'`).Scan(&model, &upstreamModel, &channelKey, &workspace); err != nil || model != "video-model" || upstreamModel != "upstream-video" || channelKey != ch.keyID || workspace != key.workspaceID {
		t.Fatalf("persisted fields model=%s upstream=%s key=%s workspace=%s err=%v", model, upstreamModel, channelKey, workspace, err)
	}
	if _, err := s.registerVideoTask(context.Background(), key, ch, "video-model", []byte(`{"id":"video_durable","status":"queued"}`)); err == nil {
		t.Fatal("duplicate task id accepted")
	}
	otherSecret := "sk-xh-video-other-key"
	integrationKey(t, db, key.userID, otherSecret)
	otherUser, _ := integrationUser(t, db, "video-other@example.com", 10)
	foreignSecret := "sk-xh-video-other-user"
	integrationKey(t, db, otherUser, foreignSecret)
	for _, test := range []struct {
		secret, path string
		status       int
	}{
		{"", "/v1/videos/" + localID, 401},
		{otherSecret, "/v1/videos/" + localID, 404},
		{foreignSecret, "/v1/videos/" + localID + "/content", 404},
		{secret, "/v1/videos/unknown", 404},
		{secret, "/v1/videos/" + localID + "/content?url=https://example.test", 400},
		{secret, "/v1/videos/" + localID + "/content?variant=video&variant=thumbnail", 400},
		{secret, "/v1/videos/" + localID + "?variant=video", 400},
	} {
		if got := videoIntegrationGet(s, test.secret, test.path, ""); got.Code != test.status {
			t.Fatalf("path=%s status=%d body=%s", test.path, got.Code, got.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("unauthorized/invalid requests reached upstream")
	}
	restarted := integrationService(t, db)
	if got := videoIntegrationGet(restarted, secret, "/v1/videos/"+localID, ""); got.Code != 200 || !strings.Contains(got.Body.String(), `"status":"completed"`) || !strings.Contains(got.Body.String(), `"id":"`+localID+`"`) {
		t.Fatalf("restart retrieval status=%d body=%s", got.Code, got.Body.String())
	}
	got := videoIntegrationGet(restarted, secret, "/v1/videos/"+localID+"/content?variant=video", "bytes=2-5")
	if got.Code != 206 || string(got.Body.Bytes()) != string([]byte{0, 1, 254, 255}) || got.Header().Get("Content-Type") != "video/mp4" || got.Header().Get("Content-Range") != "bytes 2-5/8" || got.Header().Get("Accept-Ranges") != "bytes" || got.Header().Get("Content-Disposition") != `attachment; filename="clip.mp4"` || got.Header().Get("Content-Length") != "4" {
		t.Fatalf("content status=%d headers=%v body=%v", got.Code, got.Header(), got.Body.Bytes())
	}
	var usage int
	var balance, reserved string
	if err := db.QueryRow(context.Background(), `select count(*) from usage_records`).Scan(&usage); err != nil || usage != 0 {
		t.Fatalf("GET usage=%d err=%v", usage, err)
	}
	if err := db.QueryRow(context.Background(), `select balance::text,reserved::text from user_wallets where user_id=$1`, key.userID).Scan(&balance, &reserved); err != nil || balance != "100.00000000" || reserved != "0.00000000" {
		t.Fatalf("GET wallet balance=%s reserved=%s err=%v", balance, reserved, err)
	}
	otherKey, err := s.loadAPIKeyContext(context.Background(), otherSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.registerVideoTask(context.Background(), otherKey, ch, "video-model", []byte(`{"id":"video_durable","status":"queued"}`)); err == nil {
		t.Fatal("cross-owner collision accepted")
	}
	otherUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/videos/video_durable" || r.Header.Get("Authorization") != "Bearer fake-other-provider" {
			t.Errorf("wrong second-provider request")
		}
		_, _ = io.WriteString(w, `{"id":"video_durable","status":"completed","provider":"other"}`)
	}))
	defer otherUpstream.Close()
	otherChannel := channel{baseURL: otherUpstream.URL, apiKey: "fake-other-provider"}
	if err := db.QueryRow(context.Background(), `insert into channels(name,base_url,api_key,models,provider,user_id) values('other-video',$1,$2,'["video-model"]','openai',$3) returning id`, otherChannel.baseURL, otherChannel.apiKey, key.userID).Scan(&otherChannel.id); err != nil {
		t.Fatal(err)
	}
	otherBody, err := s.registerVideoTask(context.Background(), key, otherChannel, "video-model", []byte(`{"id":"video_durable","status":"queued"}`))
	if err != nil {
		t.Fatalf("independent provider ID collision: %v", err)
	}
	otherID, _, _ := parseVideoTask(otherBody)
	if otherID == localID {
		t.Fatal("provider IDs not namespaced")
	}
	if got := videoIntegrationGet(restarted, secret, "/v1/videos/"+otherID, ""); got.Code != 200 || !strings.Contains(got.Body.String(), `"provider":"other"`) || !strings.Contains(got.Body.String(), `"id":"`+otherID+`"`) {
		t.Fatalf("second provider status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestIntegrationVideoOriginalCredentialAndChannel(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fake-video-original" {
			t.Errorf("credential=%q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"id":"video_pinned","status":"in_progress"}`)
	}))
	defer upstream.Close()
	s, db, secret, key, ch := videoIntegrationFixture(t, upstream.URL)
	response, err := s.registerVideoTask(context.Background(), key, ch, "video-model", []byte(`{"id":"video_pinned","status":"queued"}`))
	if err != nil {
		t.Fatal(err)
	}
	localID, _, _ := parseVideoTask(response)

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(context.Background(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`insert into channel_api_keys(channel_id,name,key_encrypted) values($1,'alternate','wrong-alternate')`, ch.id)
	if got := videoIntegrationGet(s, secret, "/v1/videos/"+localID, ""); got.Code != 200 {
		t.Fatalf("original status=%d body=%s", got.Code, got.Body.String())
	}
	baseline := calls.Load()
	for _, test := range []struct {
		disable, restore string
		args             []any
	}{
		{`update channel_api_keys set key_encrypted='rotated-original' where id=$1`, `update channel_api_keys set key_encrypted='fake-video-original' where id=$1`, []any{ch.keyID}},
		{`update channel_api_keys set enabled=false where id=$1`, `update channel_api_keys set enabled=true where id=$1`, []any{ch.keyID}},
		{`update channels set enabled=false where id=$1`, `update channels set enabled=true where id=$1`, []any{ch.id}},
		{`update channels set auto_disabled=true where id=$1`, `update channels set auto_disabled=false where id=$1`, []any{ch.id}},
		{`update channels set base_url='http://127.0.0.1:1' where id=$1`, `update channels set base_url=$2 where id=$1`, []any{ch.id}},
	} {
		exec(test.disable, test.args...)
		if got := videoIntegrationGet(s, secret, "/v1/videos/"+localID, ""); got.Code != 503 {
			t.Fatalf("disabled original status=%d body=%s", got.Code, got.Body.String())
		}
		if strings.Contains(test.restore, "$2") {
			exec(test.restore, ch.id, upstream.URL)
		} else {
			exec(test.restore, test.args...)
		}
	}
	exec(`delete from channel_api_keys where id=$1`, ch.keyID)
	got := videoIntegrationGet(s, secret, "/v1/videos/"+localID, "")
	if got.Code != 503 {
		t.Fatalf("deleted credential status=%d body=%s", got.Code, got.Body.String())
	}
	var logged bool
	if err := db.QueryRow(context.Background(), `select exists(select 1 from request_logs where request_id=$1 and error_code='video_channel_unavailable')`, got.Header().Get("X-Request-ID")).Scan(&logged); err != nil || !logged {
		t.Fatalf("deleted credential request logged=%v err=%v", logged, err)
	}
	if calls.Load() != baseline {
		t.Fatal("unavailable original credential rerouted upstream")
	}
}

func TestIntegrationVideoUpstreamValidationAndErrors(t *testing.T) {
	var mode atomic.Int64
	var redirects atomic.Int64
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects.Add(1); w.WriteHeader(200) }))
	defer redirected.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 0:
			_, _ = io.WriteString(w, `{"id":"wrong_id","status":"completed"}`)
		case 1:
			_, _ = io.WriteString(w, `{"id":"video_errors","status":"bogus"}`)
		case 2:
			_, _ = io.WriteString(w, strings.Repeat("x", maxVideoTaskBody+1))
		case 3:
			http.Redirect(w, r, redirected.URL, http.StatusFound)
		case 4:
			w.WriteHeader(500)
			_, _ = io.WriteString(w, "fake-video-original"+strings.Repeat("x", 100<<10))
		case 5:
			_, _ = io.WriteString(w, `{"id":"video_errors","status":"failed","error":{"message":"generation failed"}}`)
		case 6:
			w.Header().Set("Content-Range", "bytes */8")
			w.WriteHeader(416)
		}
	}))
	defer upstream.Close()
	s, _, secret, key, ch := videoIntegrationFixture(t, upstream.URL)
	response, err := s.registerVideoTask(context.Background(), key, ch, "video-model", []byte(`{"id":"video_errors","status":"queued"}`))
	if err != nil {
		t.Fatal(err)
	}
	localID, _, _ := parseVideoTask(response)

	for i, expected := range []int{502, 502, 502, 502, 500, 200, 416} {
		mode.Store(int64(i))
		path := "/v1/videos/" + localID
		if i == 6 {
			path += "/content"
		}
		got := videoIntegrationGet(s, secret, path, "")
		if got.Code != expected {
			t.Fatalf("mode=%d status=%d body=%s", i, got.Code, got.Body.String())
		}
		if len(got.Body.Bytes()) > maxVideoTaskBody || strings.Contains(got.Body.String(), "fake-video-original") {
			t.Fatal("unbounded or secret-bearing error response")
		}
		if i == 6 && got.Header().Get("Content-Range") != "bytes */8" {
			t.Fatal("missing unsatisfied content range")
		}
	}
	if redirects.Load() != 0 {
		t.Fatal("followed redirect with upstream credential")
	}
	for _, body := range []string{`{"id":"bad/id","status":"queued"}`, `{"id":"video_failed","status":"failed"}`} {
		if _, err := s.registerVideoTask(context.Background(), key, ch, "video-model", []byte(body)); err == nil {
			t.Fatalf("invalid task persisted: %s", body)
		}
	}
}
