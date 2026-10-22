//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegrationQuotaScopeIsolation(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	user, _ := integrationUser(t, db, "quota-scope@example.test", 0)
	other, _ := integrationUser(t, db, "quota-scope-other@example.test", 0)
	first := integrationKey(t, db, user, "sk-quota-scope-first")
	second := integrationKey(t, db, user, "sk-quota-scope-second")
	fresh := integrationKey(t, db, user, "sk-quota-scope-fresh")
	foreign := integrationKey(t, db, other, "sk-quota-scope-foreign")
	for i, entry := range []struct {
		user, key, model string
		tokens           int
		cost             float64
	}{
		{user, first, "model-a", 10, 0.25}, {user, second, "model-a", 20, 0.5},
		{user, first, "model-b", 100, 2}, {other, foreign, "model-a", 1000, 20},
	} {
		queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: fmt.Sprintf("scope-%d", i), userID: entry.user, keyID: entry.key, model: entry.model, status: 200, total: entry.tokens, cost: entry.cost, withUsage: true, created: time.Now()})
	}
	for _, tc := range []struct {
		name                   string
		user, key, model       any
		target                 string
		requests, tokens, cost any
		reject                 bool
	}{
		{"user requests across keys", user, nil, nil, fresh, 3, nil, nil, true},
		{"user tokens across keys", user, nil, nil, fresh, nil, 130, nil, true},
		{"user cost across keys", user, nil, nil, fresh, nil, nil, 2.75, true},
		{"other user excluded", user, nil, nil, fresh, 4, 131, 2.76, false},
		{"key overrides user scope", user, second, nil, second, 2, 21, 0.51, false},
		{"key-only scope", nil, second, nil, second, 1, nil, nil, true},
		{"user model requests", user, nil, "model-a", fresh, 2, nil, nil, true},
		{"user model excludes other models", user, nil, "model-a", fresh, 3, 31, 0.76, false},
		{"key model excludes other models", user, first, "model-a", first, 2, 11, 0.26, false},
		{"key model tokens", user, first, "model-a", first, nil, 10, nil, true},
		{"key model cost", user, first, "model-a", first, nil, nil, 0.25, true},
		{"legacy global is per key", nil, nil, nil, second, 2, 21, 0.51, false},
		{"unrelated model", user, nil, "model-b", fresh, 0, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, `delete from quota_limits`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens,max_cost) values(gen_random_uuid(),$1,$2,$3,'day',$4,$5,$6)`, tc.user, tc.key, tc.model, tc.requests, tc.tokens, tc.cost); err != nil {
				t.Fatal(err)
			}
			err := s.checkQuota(ctx, keyContext{userID: user, keyID: tc.target}, "model-a")
			if tc.reject && !errors.Is(err, errInvalid) || !tc.reject && err != nil {
				t.Fatalf("quota error=%v reject=%v", err, tc.reject)
			}
		})
	}
}

func TestIntegrationQuotaSourceOwnership(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	user, _ := integrationUser(t, db, "quota-owner@example.test", 0)
	key := integrationKey(t, db, user, "sk-quota-owner-key")
	otherKey := integrationKey(t, db, user, "sk-quota-owner-other")
	var workspace string
	if err := db.QueryRow(ctx, `select workspace_id from api_keys where id=$1`, key).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	account := accountContext{userID: user, workspaceID: workspace}
	call := func(handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.SetPathValue("id", key)
		r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, account))
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	for _, scope := range []any{user, nil} {
		for _, window := range []string{"minute", "day", "month", "total"} {
			if _, err := db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,"window",max_requests) values(gen_random_uuid(),$1,$2,$3,2)`, scope, key, window); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, window := range []string{"minute", "day", "month", "total"} {
		w := call(s.keyQuotaDelete, http.MethodDelete, "/quota?window="+window, "")
		want := http.StatusNoContent
		if window == "minute" {
			want = http.StatusBadRequest
		}
		if w.Code != want {
			t.Fatalf("delete %s status=%d body=%s", window, w.Code, w.Body.String())
		}
	}
	s.quotaAbsentCache = newTTLCache[quotaRouteKey, struct{}](time.Minute)
	route := quotaRouteKey{userID: user, keyID: key, model: "model-a"}
	s.quotaAbsentCache.store(route, struct{}{})
	w := call(s.keyQuotaUpsert, http.MethodPut, "/quota", `{"window":"day","max_requests":100}`)
	if w.Code != 200 {
		t.Fatalf("self upsert=%d %s", w.Code, w.Body.String())
	}
	if _, ok := s.quotaAbsentCache.lookup(route); ok {
		t.Fatal("quota absence cache was not invalidated")
	}
	var saved struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	w = call(s.keyQuotaUpsert, http.MethodPut, "/quota", `{"window":"day","max_requests":99}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), saved.ID) {
		t.Fatalf("repeat self upsert=%d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		w = call(s.upsertQuota, http.MethodPost, "/quota", fmt.Sprintf(`{"user_id":%s,"api_key_id":%q,"window":"day","max_requests":2}`, user, key))
		if w.Code != 200 {
			t.Fatalf("admin upsert=%d %s", w.Code, w.Body.String())
		}
	}
	queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: "owner-key", userID: user, keyID: key, model: "model-a", status: 200, total: 5, cost: 0.25, withUsage: true, created: time.Now()})
	queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: "owner-other", userID: user, keyID: otherKey, model: "model-a", status: 200, total: 500, cost: 25, withUsage: true, created: time.Now()})
	usage, err := s.keyQuotaUsage(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 4 {
		t.Fatalf("usage windows=%d want 4", len(usage))
	}
	for _, row := range usage {
		if row["requests"] != int64(1) || row["tokens"] != int64(5) || row["cost"] != float64(0.25) {
			t.Fatalf("key usage includes other key: %#v", row)
		}
	}
	limits, err := s.listKeyQuotaLimits(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 9 {
		t.Fatalf("limits=%d want 9", len(limits))
	}
	var admins, self int
	for _, row := range limits {
		switch row["source"] {
		case "admin":
			admins++
		case "self_service":
			self++
		default:
			t.Fatalf("missing source %#v", row)
		}
	}
	if admins != 8 || self != 1 {
		t.Fatalf("admin=%d self=%d", admins, self)
	}
	queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: "owner-key-second", userID: user, keyID: key, model: "model-a", status: 200, total: 5, created: time.Now()})
	if err := s.checkQuota(ctx, keyContext{userID: user, keyID: key}, "model-a"); !errors.Is(err, errInvalid) {
		t.Fatalf("self quota bypassed admin: %v", err)
	}
	w = call(s.keyQuotaDelete, http.MethodDelete, "/quota?window=day", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete self=%d", w.Code)
	}
	var count int
	if err := db.QueryRow(ctx, `select count(*) from quota_limits where source='admin' and max_requests=2`).Scan(&count); err != nil || count != 8 {
		t.Fatalf("admin rows=%d err=%v", count, err)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- call(s.keyQuotaUpsert, http.MethodPut, "/quota", `{"window":"day","max_requests":10}`).Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concurrent upsert=%d", status)
		}
	}
	if err := db.QueryRow(ctx, `select count(*) from quota_limits where source='self_service'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("self rows=%d err=%v", count, err)
	}
}

func TestIntegrationQuotaMigrationPreservesHistoricalRules(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	user, _ := integrationUser(t, db, "quota-migration@example.test", 0)
	key := integrationKey(t, db, user, "sk-quota-migration-key")
	if _, err := db.Exec(ctx, `alter table quota_limits drop column source cascade`); err != nil {
		t.Fatal(err)
	}
	original, err := migrations.ReadFile("migrations/073_quota_limits_unique_scope.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(original)); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []any{user, nil} {
		if _, err = db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,"window",max_requests) values(gen_random_uuid(),$1,$2,'day',2)`, scope, key); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := migrations.ReadFile("migrations/133_quota_sources.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = db.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(ctx, `select count(*) from quota_limits where source='admin' and max_requests=2`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("historical admin rows=%d err=%v", count, err)
	}
	if _, err = db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,"window",max_requests,source) values(gen_random_uuid(),$1,$2,'day',10,'self_service')`, user, key); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,"window",max_requests) values(gen_random_uuid(),$1,$2,'day',100)`, user, key); err == nil {
		t.Fatal("duplicate admin scope with null model accepted")
	}
	if _, err = db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,"window",max_requests,source) values(gen_random_uuid(),$1,$2,'day',100,'self_service')`, user, key); err == nil {
		t.Fatal("duplicate self-service scope accepted")
	}
}
