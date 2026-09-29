//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIntegrationQueryPerformanceLoadChannels(t *testing.T) {
	db, dsn := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "query-channels@example.com", 0)
	keyID := integrationKey(t, db, userID, "sk-query-channels-integration")
	ctx := context.Background()
	insertChannel := func(name, legacy string, priority int) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(ctx, `insert into channels(name,user_id,base_url,api_key,models,priority) values($1,$2,'https://example.com',$3,'[]',$4) returning id`, name, userID, legacy, priority).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `insert into model_routes(id,channel_id,public_model,upstream_model,priority,weight) values(gen_random_uuid(),$1,'query-model',$2,$3,7)`, id, name+"-upstream", priority); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insertKey := func(channelID int64, secret string, priority int, enabled bool) string {
		t.Helper()
		var id string
		if err := db.QueryRow(ctx, `insert into channel_api_keys(channel_id,key_encrypted,priority,enabled) values($1,$2,$3,$4) returning id`, channelID, secret, priority, enabled).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := insertChannel("query-first", "unused-legacy", 30)
	second := insertChannel("query-second", "", 20)
	legacy := insertChannel("query-legacy", mustCrypt(t, "legacy-secret"), 10)
	lowID := insertKey(first, "first-low", 1, true)
	highID := insertKey(first, mustCrypt(t, "first-high"), 20, true)
	insertKey(first, "disabled-secret", 100, false)
	insertKey(first, channelCredentialPrefix+"invalid-ciphertext", 90, true)
	secondID := insertKey(second, "second-secret", 5, true)
	insertKey(legacy, "disabled-legacy", 100, false)
	insertKey(legacy, channelCredentialPrefix+"invalid-ciphertext", 90, true)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	tracer := &queryPerformanceTracer{}
	config.MaxConns = 1
	config.ConnConfig.Tracer = tracer
	single, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer single.Close()
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cached), func(t *testing.T) {
			s := integrationService(t, single)
			if !cached {
				s.channelKeyCache = nil
			}
			for pass := 0; pass < 3; pass++ {
				tracer.queries = 0
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				channels, err := s.loadChannelsForModel(ctx, keyContext{userID: userID, keyID: keyID}, "query-model")
				cancel()
				if err != nil {
					t.Fatalf("pass %d with one connection: %v", pass, err)
				}
				wantQueries := 2
				if cached && pass == 1 {
					wantQueries = 1
				}
				if tracer.queries != wantQueries {
					t.Fatalf("pass %d queries=%d, want %d", pass, tracer.queries, wantQueries)
				}
				if len(channels) != 3 {
					t.Fatalf("channels=%d, want 3", len(channels))
				}
				for i, want := range []struct {
					id       int64
					priority int
					upstream string
					secret   string
					keyID    string
					keys     []channelKeyCredential
				}{
					{first, 30, "query-first-upstream", "first-high", highID, []channelKeyCredential{{id: highID, key: "first-high", priority: 20}, {id: lowID, key: "first-low", priority: 1}}},
					{second, 20, "query-second-upstream", "second-secret", secondID, []channelKeyCredential{{id: secondID, key: "second-secret", priority: 5}}},
					{legacy, 10, "query-legacy-upstream", "legacy-secret", "", nil},
				} {
					got := channels[i]
					if got.id != want.id || got.priority != want.priority || got.upstreamModel != want.upstream || got.weight != 7 || !got.inKeyGroup || got.apiKey != want.secret || got.keyID != want.keyID || !reflect.DeepEqual(got.keys, want.keys) {
						t.Fatalf("channel %d has incorrect routing, priority, or credentials", i)
					}
				}
				if cached {
					for _, id := range []int64{first, second, legacy} {
						if _, ok := s.channelKeyCache.lookup(id); !ok {
							t.Fatalf("channel %d keys were not cached", id)
						}
					}
					if pass == 1 {
						s.channelKeyCache.invalidate(second)
					}
				}
			}
		})
	}
}

func TestIntegrationQueryPerformanceCheckQuota(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "query-quota@example.com", 0)
	otherUserID, _ := integrationUser(t, db, "query-quota-other@example.com", 0)
	keyID := integrationKey(t, db, userID, "sk-query-quota-integration")
	otherKeyID := integrationKey(t, db, otherUserID, "sk-query-quota-other-integration")
	s := integrationService(t, db)
	key := keyContext{userID: userID, keyID: keyID}
	ctx := context.Background()
	var now time.Time
	if err := db.QueryRow(ctx, `select now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	for i, age := range []time.Duration{0, 2 * time.Minute, 2 * 24 * time.Hour, 40 * 24 * time.Hour} {
		queryPerformanceInsertUsage(t, db, queryPerformanceUsage{
			requestID: fmt.Sprintf("quota-%d", i), userID: userID, keyID: keyID, model: "historical-model", status: 200,
			prompt: 6, completion: 4, total: 10, cost: 0.25, withUsage: true, created: now.Add(-age),
		})
	}
	queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: "quota-unsettled", userID: userID, keyID: keyID, model: "query-model", status: 500, total: 3, created: now})
	queryPerformanceInsertUsage(t, db, queryPerformanceUsage{requestID: "quota-other", userID: otherUserID, keyID: otherKeyID, model: "query-model", status: 200, total: 10000, cost: 1000, withUsage: true, created: now})
	clearLimits := func() {
		t.Helper()
		if _, err := db.Exec(ctx, `delete from quota_limits`); err != nil {
			t.Fatal(err)
		}
		s.quotaAbsentCache = nil
	}
	insertLimit := func(user, apiKey, model any, window string, requests, tokens, cost any) {
		t.Helper()
		if _, err := db.Exec(ctx, `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens,max_cost) values(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7)`, user, apiKey, model, window, requests, tokens, cost); err != nil {
			t.Fatal(err)
		}
	}
	assertQuota := func(reject bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := s.checkQuota(ctx, key, "query-model")
		if reject && !errors.Is(err, errInvalid) {
			t.Fatalf("quota error=%v, want rejection", err)
		}
		if !reject && err != nil {
			t.Fatalf("quota error=%v, want allowed", err)
		}
	}
	for _, window := range []struct {
		name     string
		requests int64
		tokens   int64
		cost     float64
	}{{"minute", 2, 13, 0.25}, {"day", 3, 23, 0.50}, {"month", 4, 33, 0.75}, {"total", 5, 43, 1}} {
		for _, metric := range []string{"requests", "tokens", "cost"} {
			for _, reject := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/reject=%t", window.name, metric, reject), func(t *testing.T) {
					clearLimits()
					var requests, tokens, cost any
					increment := int64(1)
					if reject {
						increment = 0
					}
					switch metric {
					case "requests":
						requests = window.requests + increment
					case "tokens":
						tokens = window.tokens + increment
					case "cost":
						cost = window.cost + float64(increment)*0.01
					}
					insertLimit(userID, keyID, "query-model", window.name, requests, tokens, cost)
					assertQuota(reject)
				})
			}
		}
	}
	for scope := 0; scope < 8; scope++ {
		t.Run(fmt.Sprintf("nullable-scope-%d", scope), func(t *testing.T) {
			clearLimits()
			var user, apiKey, model any
			if scope&1 != 0 {
				user = userID
			}
			if scope&2 != 0 {
				apiKey = keyID
			}
			if scope&4 != 0 {
				model = "query-model"
			}
			insertLimit(user, apiKey, model, "minute", 2, nil, nil)
			assertQuota(true)
		})
	}
	t.Run("unmatched-and-unlimited", func(t *testing.T) {
		clearLimits()
		insertLimit(otherUserID, nil, nil, "total", 0, 0, 0)
		insertLimit(nil, otherKeyID, nil, "total", 0, 0, 0)
		insertLimit(nil, nil, "another-model", "total", 0, 0, 0)
		insertLimit(userID, keyID, "query-model", "total", nil, nil, nil)
		assertQuota(false)
	})
	t.Run("multiple-windows", func(t *testing.T) {
		clearLimits()
		insertLimit(userID, keyID, "query-model", "minute", 3, 14, 0.26)
		insertLimit(userID, keyID, "query-model", "day", 4, 24, 0.51)
		insertLimit(userID, keyID, "query-model", "month", 5, 34, 0.76)
		insertLimit(userID, keyID, "query-model", "total", 6, 44, 1.01)
		assertQuota(false)
		if _, err := db.Exec(ctx, `update quota_limits set max_cost=0.50 where "window"='day'`); err != nil {
			t.Fatal(err)
		}
		assertQuota(true)
	})
	t.Run("no-limits", func(t *testing.T) {
		clearLimits()
		assertQuota(false)
		s.quotaAbsentCache = newTTLCache[quotaRouteKey, struct{}](time.Minute)
		assertQuota(false)
		if _, ok := s.quotaAbsentCache.lookup(quotaRouteKey{userID: userID, keyID: keyID, model: "query-model"}); !ok {
			t.Fatal("missing no-limits cache entry")
		}
		assertQuota(false)
	})
}

func TestIntegrationQueryPerformanceUsageStats(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	userID, _ := integrationUser(t, db, "query-stats@example.com", 0)
	otherUserID, _ := integrationUser(t, db, "query-stats-other@example.com", 0)
	keyID := integrationKey(t, db, userID, "sk-query-stats-integration")
	var channelID, otherChannelID int64
	var groupID, otherGroupID string
	ctx := context.Background()
	for i, id := range []*int64{&channelID, &otherChannelID} {
		if err := db.QueryRow(ctx, `insert into channels(name,user_id,base_url,api_key) values($1,$2,'https://example.com','') returning id`, fmt.Sprintf("stats-channel-%d", i), userID).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []*string{&groupID, &otherGroupID} {
		if err := db.QueryRow(ctx, `insert into groups(id,name) values(gen_random_uuid(),$1) returning id`, fmt.Sprintf("stats-group-%d", i)).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	filters := url.Values{"user_id": {userID}, "model": {"stats-model"}, "channel_id": {strconv.FormatInt(channelID, 10)}, "group_id": {groupID}, "start": {start.Format(time.RFC3339)}, "end": {end.Format(time.RFC3339)}, "breakdown": {"1"}, "period": {"day"}}
	fetch := func(values url.Values) map[string]any {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		r := httptest.NewRequest(http.MethodGet, "/admin/usage-stats?"+values.Encode(), nil).WithContext(ctx)
		w := httptest.NewRecorder()
		s.usageStats(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("usageStats status=%d body=%s", w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	assertNumbers := func(result map[string]any, want map[string]float64) {
		t.Helper()
		for key, expected := range want {
			value, ok := result[key].(float64)
			if !ok || math.Abs(value-expected) > 1e-9 {
				t.Errorf("%s=%v, want %v", key, result[key], expected)
			}
		}
	}
	empty := fetch(filters)
	assertNumbers(empty, map[string]float64{"total_requests": 0, "success_count": 0, "error_count": 0, "prompt_tokens": 0, "cached_prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0, "total_cost": 0, "avg_duration_ms": 0})
	if empty["avg_first_token_ms"] != nil || len(empty["breakdown"].([]any)) != 0 {
		t.Fatalf("unexpected empty stats: %v", empty)
	}
	base := queryPerformanceUsage{userID: userID, keyID: keyID, channelID: channelID, groupID: groupID, model: "stats-model", status: 200, prompt: 10, completion: 5, total: 15, cached: 2, cost: 0.25, withUsage: true, created: start}
	for i, status := range []int{100, 200, 302, 500} {
		row := base
		row.requestID = fmt.Sprintf("stats-%d", status)
		row.status = status
		row.duration = (i + 1) * 100
		if i > 0 {
			row.firstToken = i * 10
		}
		if i >= 2 {
			row.created = end
		}
		queryPerformanceInsertUsage(t, db, row)
	}
	for i, alter := range []func(*queryPerformanceUsage){
		func(row *queryPerformanceUsage) { row.userID = otherUserID },
		func(row *queryPerformanceUsage) { row.model = "other-model" },
		func(row *queryPerformanceUsage) { row.channelID = otherChannelID },
		func(row *queryPerformanceUsage) { row.groupID = otherGroupID },
		func(row *queryPerformanceUsage) { row.created = start.Add(-time.Second) },
		func(row *queryPerformanceUsage) { row.created = end.Add(time.Second) },
		func(row *queryPerformanceUsage) { row.errorCode = "user_concurrency_limit"; row.status = 429 },
		func(row *queryPerformanceUsage) { row.errorCode = "group_concurrency_limit"; row.status = 429 },
	} {
		row := base
		row.requestID = fmt.Sprintf("stats-excluded-%d", i)
		row.prompt, row.cached, row.completion, row.total, row.cost = 1000, 1000, 1000, 2000, 100
		alter(&row)
		queryPerformanceInsertUsage(t, db, row)
	}
	result := fetch(filters)
	assertNumbers(result, map[string]float64{"total_requests": 4, "success_count": 2, "error_count": 1, "prompt_tokens": 40, "cached_prompt_tokens": 8, "completion_tokens": 20, "total_tokens": 60, "total_cost": 1, "avg_duration_ms": 250, "avg_first_token_ms": 20})
	breakdown, ok := result["breakdown"].([]any)
	if !ok || len(breakdown) != 2 {
		t.Fatalf("breakdown=%v, want two buckets", result["breakdown"])
	}
	var previous time.Time
	for i, raw := range breakdown {
		bucket := raw.(map[string]any)
		assertNumbers(bucket, map[string]float64{"requests": 2, "prompt_tokens": 20, "cached_prompt_tokens": 4, "completion_tokens": 10, "total_tokens": 30, "cost": 0.5, "avg_first_token_ms": []float64{10, 25}[i]})
		period, err := time.Parse(time.RFC3339, bucket["period"].(string))
		if err != nil || (!previous.IsZero() && !period.After(previous)) {
			t.Fatalf("invalid or unordered bucket: %v", bucket["period"])
		}
		previous = period
	}
	filters.Del("breakdown")
	if _, exists := fetch(filters)["breakdown"]; exists {
		t.Fatal("breakdown returned without being requested")
	}
	filters.Set("model", "absent-model")
	assertNumbers(fetch(filters), map[string]float64{"total_requests": 0, "success_count": 0, "error_count": 0, "total_cost": 0})
}

func TestIntegrationQueryPerformanceIndexes(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	const name = "101_query_performance_indexes.sql"
	migration, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err := db.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("migration pass %d: %v", pass, err)
		}
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}
	for index, table := range map[string]string{
		"api_keys_user_created_idx":                  "api_keys",
		"request_logs_model_created_idx":             "request_logs",
		"request_logs_trimmed_model_created_idx":     "request_logs",
		"model_routes_channel_model_idx":             "model_routes",
		"channel_api_keys_channel_enabled_order_idx": "channel_api_keys",
		"quota_limits_api_key_window_idx":            "quota_limits",
	} {
		var count int
		if err := db.QueryRow(ctx, `select count(*) from pg_indexes where schemaname='public' and indexname=$1 and tablename=$2`, index, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("index %s on %s: count=%d, want 1", index, table, count)
		}
	}
	var applied int
	if err := db.QueryRow(ctx, `select count(*) from schema_migrations where name=$1`, name).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("migration recorded %d times, want 1", applied)
	}
}

type queryPerformanceTracer struct {
	queries int
}

func (tracer *queryPerformanceTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	tracer.queries++
	return ctx
}

func (*queryPerformanceTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type queryPerformanceUsage struct {
	requestID, userID, keyID, groupID, model, errorCode string
	channelID                                           int64
	status, prompt, completion, total, cached, duration int
	firstToken                                          any
	cost                                                float64
	withUsage                                           bool
	created                                             time.Time
}

func queryPerformanceInsertUsage(t *testing.T, db *pgxpool.Pool, row queryPerformanceUsage) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `insert into request_logs(id,request_id,user_id,api_key_id,channel_id,group_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,first_token_ms,error_code,created_at) values(gen_random_uuid(),$1,$2,$3,nullif($4::bigint,0),nullif($5,'')::uuid,$6,$7,$8,$9,$10,$11,$12,nullif($13,''),$14)`, row.requestID, row.userID, row.keyID, row.channelID, row.groupID, row.model, row.status, row.prompt, row.completion, row.total, row.duration, row.firstToken, row.errorCode, row.created); err != nil {
		t.Fatal(err)
	}
	if row.withUsage {
		if _, err := db.Exec(ctx, `insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost,created_at) values(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,now())`, row.requestID, row.userID, row.keyID, row.model, row.prompt, row.cached, row.completion, row.cost); err != nil {
			t.Fatal(err)
		}
	}
}
