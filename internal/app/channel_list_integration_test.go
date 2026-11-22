//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIntegrationListChannelsReturnsPerChannelAggregates(t *testing.T) {
	db, dsn := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, userName := integrationUser(t, db, "channel-list@example.com", 0)

	insertChannel := func(name string, priority int, owner any) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(ctx, `insert into channels(name,user_id,base_url,api_key,models,priority,key_type) values($1,$2,'https://example.com','legacy','["model-a","model-b"]',$3,'multi') returning id`, name, owner, priority).Scan(&id); err != nil {
			t.Fatalf("insert channel %s: %v", name, err)
		}
		return id
	}
	insertKey := func(channelID int64, priority int, enabled bool) string {
		t.Helper()
		var id string
		if err := db.QueryRow(ctx, `insert into channel_api_keys(channel_id,key_encrypted,priority,enabled) values($1,$2,$3,$4) returning id`, channelID, "sk-plaintext", priority, enabled).Scan(&id); err != nil {
			t.Fatalf("insert channel key: %v", err)
		}
		return id
	}
	insertLog := func(channelID int64, duration int, firstToken any, tokens int) {
		t.Helper()
		if _, err := db.Exec(ctx, `insert into request_logs(id,request_id,channel_id,model,status_code,duration_ms,first_token_ms,total_tokens) values(gen_random_uuid(),$1,$2,'model-a',200,$3,$4,$5)`, randomIDString(), channelID, duration, firstToken, tokens); err != nil {
			t.Fatalf("insert request log: %v", err)
		}
	}

	first := insertChannel("list-first", 300, userID)
	second := insertChannel("list-second", 200, nil)
	empty := insertChannel("list-empty", 100, nil)

	var groupA, groupB string
	if err := db.QueryRow(ctx, `insert into groups(id,name) values(gen_random_uuid(),'list-group-a') returning id`).Scan(&groupA); err != nil {
		t.Fatalf("insert group a: %v", err)
	}
	if err := db.QueryRow(ctx, `insert into groups(id,name) values(gen_random_uuid(),'list-group-b') returning id`).Scan(&groupB); err != nil {
		t.Fatalf("insert group b: %v", err)
	}
	for _, groupID := range []string{groupB, groupA} {
		if _, err := db.Exec(ctx, `insert into channel_groups(channel_id,group_id) values($1,$2)`, first, groupID); err != nil {
			t.Fatalf("insert channel group: %v", err)
		}
	}

	lowKey := insertKey(first, 10, true)
	highKey := insertKey(first, 90, true)
	disabledKey := insertKey(first, 500, false)
	insertKey(second, 50, true)
	if _, err := db.Exec(ctx, `insert into channel_balances(channel_id,key_id,balance,used,total,currency,supported,error,fetched_at) values($1,$2,5,1,6,'USD',true,'',now())`, first, lowKey); err != nil {
		t.Fatalf("insert low balance: %v", err)
	}
	if _, err := db.Exec(ctx, `insert into channel_balances(channel_id,key_id,balance,used,total,currency,supported,error,fetched_at) values($1,$2,99,1,100,'EUR',true,'',now())`, first, highKey); err != nil {
		t.Fatalf("insert high balance: %v", err)
	}
	if _, err := db.Exec(ctx, `insert into channel_balances(channel_id,key_id,balance,used,total,currency,supported,error,fetched_at) values($1,$2,1,1,2,'JPY',true,'',now())`, first, disabledKey); err != nil {
		t.Fatalf("insert disabled balance: %v", err)
	}

	for _, route := range []struct {
		public   string
		upstream string
		priority int
	}{{"route-b", "upstream-b", 5}, {"route-a", "upstream-a", 20}, {"route-c", "upstream-c", 5}} {
		if _, err := db.Exec(ctx, `insert into model_routes(id,channel_id,public_model,upstream_model,priority,weight,enabled,hidden) values(gen_random_uuid(),$1,$2,$3,$4,7,true,false)`, first, route.public, route.upstream, route.priority); err != nil {
			t.Fatalf("insert model route: %v", err)
		}
	}

	insertLog(first, 100, 40, 11)
	insertLog(first, 300, nil, 22)
	insertLog(second, 500, 60, 33)

	tracer := &queryPerformanceTracer{}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	config.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("traced pool: %v", err)
	}
	defer traced.Close()
	s := integrationService(t, traced)

	fetch := func(url string) pagedList {
		t.Helper()
		tracer.queries = 0
		recorder := httptest.NewRecorder()
		s.listChannels(recorder, httptest.NewRequest(http.MethodGet, url, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var body pagedList
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return body
	}

	body := fetch("/admin/channels?page=1&page_size=50")
	if body.Total != 3 || body.Page != 1 || body.PageSize != 50 {
		t.Fatalf("unexpected page envelope: total=%d page=%d page_size=%d", body.Total, body.Page, body.PageSize)
	}
	rows, err := json.Marshal(body.Data)
	if err != nil {
		t.Fatalf("re-encode data: %v", err)
	}
	var channels []map[string]any
	if err := json.Unmarshal(rows, &channels); err != nil {
		t.Fatalf("decode channels: %v", err)
	}
	if len(channels) != 3 {
		t.Fatalf("channels=%d, want 3", len(channels))
	}
	if got := channels[0]["name"]; got != "list-first" {
		t.Fatalf("first channel = %v, want list-first (priority ordering)", got)
	}

	byID := map[float64]map[string]any{}
	for _, channel := range channels {
		id, _ := channel["id"].(string)
		byID[float64Of(t, id)] = channel
	}
	got := byID[float64(first)]
	if got == nil {
		t.Fatalf("channel %d missing from response: %s", first, rows)
	}
	if got["provider"] != "openai" || got["key_type"] != "multi" || got["key_count"] != float64(2) {
		t.Fatalf("provider/key metadata wrong: %v", got)
	}
	if got["response_time_ms"] != float64(200) {
		t.Fatalf("response_time_ms = %v, want 200", got["response_time_ms"])
	}
	if got["avg_first_token_ms"] != float64(40) {
		t.Fatalf("avg_first_token_ms = %v, want 40 (nulls ignored)", got["avg_first_token_ms"])
	}
	if got["used_requests"] != float64(2) || got["used_tokens"] != float64(33) {
		t.Fatalf("usage totals wrong: requests=%v tokens=%v", got["used_requests"], got["used_tokens"])
	}
	if got["upstream_balance"] != float64(99) || got["upstream_currency"] != "EUR" {
		t.Fatalf("upstream balance should come from the highest-priority enabled key: %v/%v", got["upstream_balance"], got["upstream_currency"])
	}
	if got["user_id"] != float64Of(t, userID) {
		t.Fatalf("user_id = %v, want %s", got["user_id"], userID)
	}
	if got["user_email"] != "channel-list@example.com" || got["user_name"] != userName {
		t.Fatalf("owner fields wrong: %v/%v", got["user_email"], got["user_name"])
	}
	groups, _ := got["groups"].([]any)
	wantGroups := []string{groupA, groupB}
	sort.Strings(wantGroups)
	if len(groups) != 2 || groups[0] != wantGroups[0] || groups[1] != wantGroups[1] {
		t.Fatalf("groups = %v, want sorted %v", got["groups"], wantGroups)
	}
	routes, _ := got["model_routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("model_routes = %v, want 3", got["model_routes"])
	}
	wantRouteOrder := []string{"route-a", "route-b", "route-c"}
	for i, want := range wantRouteOrder {
		route, _ := routes[i].(map[string]any)
		if route["public_model"] != want {
			t.Fatalf("route %d = %v, want %s", i, route["public_model"], want)
		}
	}
	if models, _ := got["models"].([]any); len(models) != 2 || models[0] != "model-a" {
		t.Fatalf("models = %v", got["models"])
	}

	emptyRow := byID[float64(empty)]
	if emptyRow == nil {
		t.Fatalf("channel %d missing from response", empty)
	}
	if emptyRow["used_requests"] != float64(0) || emptyRow["response_time_ms"] != float64(0) || emptyRow["avg_first_token_ms"] != nil {
		t.Fatalf("channel without logs should report zeroed usage: %v", emptyRow)
	}
	if emptyRow["model_routes"] != nil {
		t.Fatalf("channel without routes should report null model_routes: %v", emptyRow["model_routes"])
	}
	if emptyRow["user_email"] != "" || emptyRow["user_id"] != nil {
		t.Fatalf("unowned channel should report blank owner: %v/%v", emptyRow["user_email"], emptyRow["user_id"])
	}

	if tracer.queries != 4 {
		t.Fatalf("listChannels issued %d queries, want 4 (count, page, usage, routes)", tracer.queries)
	}

	paged := fetch("/admin/channels?page=2&page_size=2")
	if paged.Total != 3 || paged.Page != 2 {
		t.Fatalf("second page envelope: total=%d page=%d", paged.Total, paged.Page)
	}
	pageRows, err := json.Marshal(paged.Data)
	if err != nil {
		t.Fatal(err)
	}
	var secondPage []map[string]any
	if err := json.Unmarshal(pageRows, &secondPage); err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 1 || secondPage[0]["name"] != "list-empty" {
		t.Fatalf("second page = %s, want the remaining single channel", pageRows)
	}

	beyond := fetch("/admin/channels?page=9&page_size=2")
	if beyond.Data == nil {
		t.Fatal("empty page must encode as [] rather than null")
	}
	if raw, _ := json.Marshal(beyond.Data); string(raw) != "[]" {
		t.Fatalf("empty page = %s, want []", raw)
	}
	if tracer.queries != 2 {
		t.Fatalf("empty page issued %d queries, want 2 (no usage or route lookup for an empty page)", tracer.queries)
	}
}

func float64Of(t *testing.T, value string) float64 {
	t.Helper()
	var out float64
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		t.Fatalf("parse id %q: %v", value, err)
	}
	return out
}
