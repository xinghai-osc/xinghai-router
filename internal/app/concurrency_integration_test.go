//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIntegrationClusterConcurrencyAdmission(t *testing.T) {
	redisURL, _, _ := leaseTestRedis(t)
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "cluster-concurrency@example.com", 100)
	var groupID string
	if err := db.QueryRow(context.Background(), `insert into groups(id,name,max_concurrency) values(gen_random_uuid(),'cluster',1) returning id`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `update users set max_concurrency=2 where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	a, b := integrationService(t, db), integrationService(t, db)
	for _, s := range []*Service{a, b} {
		s.cfg.DeploymentMode = "cluster"
		s.concurrencyLeases = &concurrencyLeaseManager{redisURL: redisURL, ttl: 5 * time.Second}
		s.userConcurrencyCache = newTTLCache[string, int](time.Hour)
		s.userConcurrencyCache.store(userID, 1000)
	}
	key := keyContext{userID: userID, groupID: groupID}
	_, release, scope, err := a.acquireRequestConcurrency(context.Background(), key)
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	_, _, scope, err = b.acquireRequestConcurrency(context.Background(), key)
	if err != nil || scope != "group" {
		t.Fatalf("replica bypassed group limit: %s %v", scope, err)
	}
	if _, err := db.Exec(context.Background(), `update users set max_concurrency=1 where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	_, _, scope, err = b.acquireRequestConcurrency(context.Background(), keyContext{userID: userID})
	if err != nil || scope != "user" {
		t.Fatalf("cached stale limit used: %s %v", scope, err)
	}
	release()
	_, release, scope, err = b.acquireRequestConcurrency(context.Background(), keyContext{userID: userID})
	if err != nil || scope != "" {
		t.Fatalf("no group acquire: %s %v", scope, err)
	}
	release()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := b.acquireRequestConcurrency(cancelled, key); err == nil {
		t.Fatal("failed DB read interpreted as unlimited")
	}
}

func TestIntegrationClusterGatewayRejectionReleasesReservation(t *testing.T) {
	redisURL, _, _ := leaseTestRedis(t)
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "cluster-reservation@example.com", 100)
	keyID := integrationKey(t, db, userID, "sk-xh-cluster-reservation")
	if _, err := db.Exec(context.Background(), `update users set max_concurrency=1 where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	s.cfg.DeploymentMode = "cluster"
	s.concurrencyLeases = &concurrencyLeaseManager{redisURL: redisURL, ttl: 5 * time.Second}
	s.pricingCache.store("cluster-model", pricingRule{found: true, input: 1, output: 1, multiplier: 1, exchangeRate: 1, currency: "CNY"})
	key := keyContext{userID: userID, keyID: keyID}
	_, release, scope, err := s.acquireRequestConcurrency(context.Background(), key)
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{{"limit", 429, "user_concurrency_exceeded"}, {"outage", 503, "concurrency_unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "outage" {
				s.concurrencyLeases = &concurrencyLeaseManager{redisURL: "redis://127.0.0.1:1", ttl: 5 * time.Second}
			}
			requestID, err := randomID()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), contextKey{}, key)
			ctx = context.WithValue(ctx, requestIDKey{}, requestID)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			s.proxyChatCompletions(rec, req, []byte(`{"model":"cluster-model","messages":[]}`), "cluster-model", false, 8, nil, nil, nil, nil, nil, nil)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("response=%d %s", rec.Code, rec.Body)
			}
			var held float64
			if err := db.QueryRow(context.Background(), `select reserved from user_wallets where user_id=$1`, userID).Scan(&held); err != nil {
				t.Fatal(err)
			}
			if held != 0 {
				t.Fatalf("reservation leaked: %v", held)
			}
		})
	}
}

func TestIntegrationChannelConcurrencyAdmission(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "channel-concurrency@example.com", 100)
	keyID := integrationKey(t, db, userID, "sk-xh-channel-concurrency")
	const model = "channel-concurrency-model"

	seen := make(chan string, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-mock","model":"`+model+`","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
	}))
	defer upstream.Close()

	models, err := json.Marshal([]string{model})
	if err != nil {
		t.Fatal(err)
	}
	var primaryID, fallbackID int64
	if err := db.QueryRow(context.Background(), `insert into channels(name,base_url,api_key,models,provider,priority,max_concurrency) values('primary',$1,'primary-key',$2,'openai',200,1) returning id`, upstream.URL, string(models)).Scan(&primaryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(context.Background(), `insert into channels(name,base_url,api_key,models,provider,priority,max_concurrency) values('fallback',$1,'fallback-key',$2,'openai',100,1) returning id`, upstream.URL, string(models)).Scan(&fallbackID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),$1,1,1)`, model); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	s.httpClient = upstream.Client()
	s.streamClient = upstream.Client()

	call := func(t *testing.T) *httptest.ResponseRecorder {
		t.Helper()
		requestID, err := randomID()
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.WithValue(context.Background(), contextKey{}, keyContext{userID: userID, keyID: keyID})
		ctx = context.WithValue(ctx, requestIDKey{}, requestID)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		s.proxyChatCompletions(rec, req, []byte(`{"model":"`+model+`","messages":[]}`), model, false, 8, nil, nil, nil, nil, nil, nil)
		return rec
	}

	_, releasePrimary, busy, err := s.acquireChannelConcurrency(context.Background(), channel{id: primaryID, maxConcurrency: 1})
	if err != nil || busy {
		t.Fatalf("hold primary: busy=%v err=%v", busy, err)
	}
	defer releasePrimary()
	if rec := call(t); rec.Code != http.StatusOK {
		t.Fatalf("fallback request status=%d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case got := <-seen:
		if got != "Bearer fallback-key" {
			t.Fatalf("a full channel served the request: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no upstream request observed")
	}

	_, releaseFallback, busy, err := s.acquireChannelConcurrency(context.Background(), channel{id: fallbackID, maxConcurrency: 1})
	if err != nil || busy {
		t.Fatalf("hold fallback: busy=%v err=%v", busy, err)
	}
	defer releaseFallback()
	rec := call(t)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "channel_concurrency_exceeded") {
		t.Fatalf("all channels full status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(seen) != 0 {
		t.Fatalf("a full channel was still called %d time(s)", len(seen))
	}
	var held float64
	if err := db.QueryRow(context.Background(), `select reserved from user_wallets where user_id=$1`, userID).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Fatalf("reservation leaked: %v", held)
	}
}

func TestIntegrationClusterRedisFailureHTTP(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "cluster-http@example.com", 100)
	secret := "sk-xh-cluster-http-failure"
	integrationKey(t, db, userID, secret)
	s := integrationService(t, db)
	s.cfg.RedisFailurePolicy = "deny"
	client, err := newRedisClient("redis://127.0.0.1:1", 60)
	if err != nil {
		t.Fatal(err)
	}
	s.limiter = &fallbackLimiter{primary: client, backup: newMemoryLimiter(60), policy: "deny"}
	s.redisReadiness = client
	called := false
	handler := s.api(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 503 || called {
		t.Fatalf("failed closed response=%d called=%v", rec.Code, called)
	}
	rec = httptest.NewRecorder()
	s.readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != 503 {
		t.Fatalf("readiness=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("liveness=%d", rec.Code)
	}
}
