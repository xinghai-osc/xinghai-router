package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type usageRatesFixture struct {
	s                                           *Service
	user, otherUser, workspace, otherWorkspace  string
	key, otherKey, channel, otherChannel, group string
	otherGroup, team                            string
	start, end                                  time.Time
}

type usageRatesRecord struct {
	id, user, workspace, key, channel, group, model string
	status                                          int
	errorCode                                       string
	settled                                         bool
	created                                         time.Time
}

func newUsageRatesFixture(t *testing.T) usageRatesFixture {
	t.Helper()
	f := usageRatesFixture{s: performanceIntegrationService(t), start: time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)}
	f.end = f.start.Add(10 * time.Minute)
	ctx := context.Background()
	for i, user := range []*string{&f.user, &f.otherUser} {
		if err := f.s.db.QueryRow(ctx, `insert into users(email,name) values($1,$1) returning id`, fmt.Sprintf("usage-rates-%d@example.test", i)).Scan(user); err != nil {
			t.Fatal(err)
		}
	}
	for i, item := range []struct {
		user           string
		workspace, key *string
	}{{f.user, &f.workspace, &f.key}, {f.otherUser, &f.otherWorkspace, &f.otherKey}} {
		if err := f.s.db.QueryRow(ctx, `select id from workspaces where owner_id=$1 and is_personal`, item.user).Scan(item.workspace); err != nil {
			t.Fatal(err)
		}
		if err := f.s.db.QueryRow(ctx, `insert into api_keys(id,user_id,name,key_prefix,secret_hash) values(gen_random_uuid(),$1,$2,'fake',$2) returning id`, item.user, fmt.Sprintf("Rates key %d", i)).Scan(item.key); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.db.QueryRow(ctx, `with w as (insert into workspaces(name,slug,owner_id) values('Rates team','rates-team',$1) returning id) insert into workspace_members(workspace_id,user_id,role) select id,$1,'owner' from w returning workspace_id`, f.user).Scan(&f.team); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.db.Exec(ctx, `insert into workspace_members(workspace_id,user_id,role) values($1,$2,'member')`, f.team, f.otherUser); err != nil {
		t.Fatal(err)
	}
	for i, channel := range []*string{&f.channel, &f.otherChannel} {
		if err := f.s.db.QueryRow(ctx, `insert into channels(name,base_url,api_key) values($1,'https://example.test','fake') returning id`, fmt.Sprintf("Rates channel %d", i)).Scan(channel); err != nil {
			t.Fatal(err)
		}
	}
	for i, group := range []*string{&f.group, &f.otherGroup} {
		if err := f.s.db.QueryRow(ctx, `insert into groups(id,name,display_name) values(gen_random_uuid(),$1,$1) returning id`, fmt.Sprintf("Rates group %d", i)).Scan(group); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f usageRatesFixture) record(id string) usageRatesRecord {
	return usageRatesRecord{id: id, user: f.user, workspace: f.workspace, key: f.key, channel: f.channel, group: f.group, model: "rates_%", status: 200, settled: true, created: f.start}
}

func (f usageRatesFixture) insert(t *testing.T, row usageRatesRecord) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.s.db.Exec(ctx, `insert into request_logs(id,request_id,user_id,workspace_id,api_key_id,channel_id,group_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code,created_at) values(gen_random_uuid(),$1,$2,$3,nullif($4,'')::uuid,$5,$6,$7,$8,20,10,300,10,nullif($9,''),$10)`, row.id, row.user, row.workspace, row.key, row.channel, row.group, row.model, row.status, row.errorCode, row.created); err != nil {
		t.Fatal(err)
	}
	if row.settled {
		if _, err := f.s.db.Exec(ctx, `insert into usage_records(id,request_id,user_id,workspace_id,api_key_id,model,prompt_tokens,completion_tokens,cost,created_at) values(gen_random_uuid(),$1,$2,$3,nullif($4,'')::uuid,$5,20,10,0.25,$6)`, row.id, row.user, row.workspace, row.key, row.model, row.created); err != nil {
			t.Fatal(err)
		}
	}
}

func fetchUsageRates(t *testing.T, handler http.HandlerFunc, query url.Values, account accountContext) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil).WithContext(context.WithValue(ctx, accountContextKey{}, account))
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertUsageRatesNumbers(t *testing.T, got map[string]any, want map[string]float64) {
	t.Helper()
	for name, expected := range want {
		actual, ok := got[name].(float64)
		if !ok || math.IsNaN(actual) || math.IsInf(actual, 0) || math.Abs(actual-expected) > 1e-9 {
			t.Errorf("%s=%v, want %v", name, got[name], expected)
		}
	}
}

func TestUsageRatesIntegrationAdminFiltersAndPagination(t *testing.T) {
	f := newUsageRatesFixture(t)
	for i := 0; i < 120; i++ {
		row := f.record(fmt.Sprintf("match-%03d", i))
		row.created = f.start.Add(time.Duration(i) * time.Second)
		if i == 119 {
			row.created = f.end
		}
		f.insert(t, row)
	}
	for i, alter := range []func(*usageRatesRecord){
		func(r *usageRatesRecord) { r.user, r.workspace, r.key = f.otherUser, f.otherWorkspace, f.otherKey },
		func(r *usageRatesRecord) { r.model = "rates-ab" },
		func(r *usageRatesRecord) { r.channel = f.otherChannel },
		func(r *usageRatesRecord) { r.group = f.otherGroup },
		func(r *usageRatesRecord) { r.id = "different-prefix" },
		func(r *usageRatesRecord) { r.status = 500 },
		func(r *usageRatesRecord) { r.status, r.errorCode = 429, "channel_concurrency_limit" },
		func(r *usageRatesRecord) { r.created = f.start.Add(-time.Second) },
		func(r *usageRatesRecord) { r.created = f.end.Add(time.Second) },
		func(r *usageRatesRecord) { r.created = time.Now().Add(time.Hour) },
	} {
		row := f.record(fmt.Sprintf("match-excluded-%d", i))
		alter(&row)
		f.insert(t, row)
	}
	query := url.Values{"user_id": {f.user}, "model": {"rates_%"}, "channel_id": {f.channel}, "group_id": {f.group}, "status": {"success"}, "request_id": {"match-"}, "start": {f.start.Format(time.RFC3339)}, "end": {f.end.Format(time.RFC3339)}, "page_size": {"50"}, "breakdown": {"1"}}
	for i, page := range []string{"1", "3"} {
		query.Set("page", page)
		logs := fetchUsageRates(t, f.s.listUsageLogs, query, accountContext{})
		assertUsageRatesNumbers(t, logs, map[string]float64{"total": 120})
		if got := len(logs["data"].([]any)); got != []int{50, 20}[i] {
			t.Fatalf("page %s rows=%d", page, got)
		}
		stats := fetchUsageRates(t, f.s.usageStats, query, accountContext{})
		assertUsageRatesNumbers(t, stats, map[string]float64{"total_requests": 120, "total_tokens": 36000, "total_cost": 30, "rpm": 12, "tpm": 3600})
		breakdown := stats["breakdown"].([]any)
		if len(breakdown) != 1 {
			t.Fatalf("breakdown=%v", breakdown)
		}
		assertUsageRatesNumbers(t, breakdown[0].(map[string]any), map[string]float64{"requests": 120, "total_tokens": 36000})
	}
	query.Set("page", "1")
	for _, tc := range []struct {
		status, requestID string
		requests          float64
	}{{"error", "match-", 1}, {"success", "match-000", 1}, {"in_progress", "match-", 0}, {"success", "missing", 0}} {
		query.Set("status", tc.status)
		query.Set("request_id", tc.requestID)
		stats := fetchUsageRates(t, f.s.usageStats, query, accountContext{})
		assertUsageRatesNumbers(t, stats, map[string]float64{"total_requests": tc.requests, "rpm": tc.requests / 10, "tpm": tc.requests * 30})
		logs := fetchUsageRates(t, f.s.listUsageLogs, query, accountContext{})
		assertUsageRatesNumbers(t, logs, map[string]float64{"total": tc.requests})
	}
	query.Set("status", "success")
	query.Set("request_id", "match-000")
	query.Del("start")
	assertUsageRatesNumbers(t, fetchUsageRates(t, f.s.usageStats, query, accountContext{}), map[string]float64{"rpm": 0.1, "tpm": 30})
	query.Set("start", time.Now().Add(time.Hour).Format(time.RFC3339))
	query.Set("end", time.Now().Add(2*time.Hour).Format(time.RFC3339))
	assertUsageRatesNumbers(t, fetchUsageRates(t, f.s.usageStats, query, accountContext{}), map[string]float64{"total_requests": 0, "rpm": 0, "tpm": 0})
	query.Set("start", f.end.Format(time.RFC3339))
	query.Set("end", f.start.Format(time.RFC3339))
	assertUsageRatesNumbers(t, fetchUsageRates(t, f.s.usageStats, query, accountContext{}), map[string]float64{"total_requests": 0, "rpm": 0, "tpm": 0})
}

func TestUsageRatesIntegrationAccountFiltersAndIsolation(t *testing.T) {
	f := newUsageRatesFixture(t)
	for i := 0; i < 120; i++ {
		row := f.record(fmt.Sprintf("match-%03d", i))
		if i == 119 {
			row.created = f.end
		}
		f.insert(t, row)
	}
	for i, alter := range []func(*usageRatesRecord){
		func(r *usageRatesRecord) { r.user, r.workspace, r.key = f.otherUser, f.otherWorkspace, f.otherKey },
		func(r *usageRatesRecord) { r.workspace, r.key = f.team, "" },
		func(r *usageRatesRecord) { r.model = "rates-ab" },
		func(r *usageRatesRecord) { r.group = f.otherGroup },
		func(r *usageRatesRecord) { r.key = "" },
		func(r *usageRatesRecord) { r.id = "different-prefix" },
		func(r *usageRatesRecord) { r.settled = false },
		func(r *usageRatesRecord) { r.settled, r.status = false, 500 },
		func(r *usageRatesRecord) { r.settled, r.errorCode = false, "upstream_error" },
		func(r *usageRatesRecord) { r.created = time.Now().Add(time.Hour) },
	} {
		row := f.record(fmt.Sprintf("match-excluded-%d", i))
		alter(&row)
		f.insert(t, row)
	}
	account := accountContext{userID: f.user, workspaceID: f.workspace}
	query := url.Values{"model": {"rates_%"}, "group_name": {"Rates group 0"}, "key_name": {"Rates key 0"}, "request_id": {"MATCH-"}, "status": {"settled"}, "start": {f.start.Format(time.RFC3339)}, "end": {f.end.Format(time.RFC3339)}}
	body := fetchUsageRates(t, f.s.accountUsage, query, account)
	if len(body["data"].([]any)) != 120 {
		t.Fatalf("filtered rows=%d", len(body["data"].([]any)))
	}
	assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": 120, "tokens": 3600, "cost": 30, "rpm": 12, "tpm": 360})
	for _, tc := range []struct {
		status string
		count  float64
	}{{"success", 1}, {"failed", 2}, {"in_progress", 0}} {
		query.Set("status", tc.status)
		body := fetchUsageRates(t, f.s.accountUsage, query, account)
		if float64(len(body["data"].([]any))) != tc.count {
			t.Fatalf("status=%s rows=%d", tc.status, len(body["data"].([]any)))
		}
		assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": tc.count, "tokens": tc.count * 30, "cost": 0, "rpm": tc.count / 10, "tpm": tc.count * 3})
	}
	body = fetchUsageRates(t, f.s.accountUsage, nil, account)
	if len(body["data"].([]any)) != 100 {
		t.Fatalf("default rows=%d", len(body["data"].([]any)))
	}
	assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": 127, "tokens": 3810, "cost": 31})
	for _, end := range []string{"", time.Now().Add(2 * time.Hour).Format(time.RFC3339)} {
		query := url.Values{"start": {f.start.Format(time.RFC3339)}, "end": {end}}
		body := fetchUsageRates(t, f.s.accountUsage, query, account)
		if len(body["data"].([]any)) != 127 {
			t.Fatalf("clamped rows=%d", len(body["data"].([]any)))
		}
		assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": 127, "tokens": 3810, "cost": 31})
	}
	for _, bounds := range [][2]time.Time{{f.end, f.start}, {time.Now().Add(time.Minute), time.Now().Add(time.Hour)}} {
		body := fetchUsageRates(t, f.s.accountUsage, url.Values{"start": {bounds[0].Format(time.RFC3339)}, "end": {bounds[1].Format(time.RFC3339)}}, account)
		if len(body["data"].([]any)) != 0 {
			t.Fatalf("empty range rows=%v", body["data"])
		}
		assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": 0, "tokens": 0, "cost": 0, "rpm": 0, "tpm": 0})
	}
	row := f.record("team-member")
	row.user, row.workspace, row.key = f.otherUser, f.team, ""
	f.insert(t, row)
	team := accountContext{userID: f.user, workspaceID: f.team}
	body = fetchUsageRates(t, f.s.accountUsage, url.Values{"start": {f.start.Format(time.RFC3339)}, "end": {f.end.Format(time.RFC3339)}}, team)
	if len(body["data"].([]any)) != 2 {
		t.Fatalf("team rows=%d", len(body["data"].([]any)))
	}
	assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": 2, "tokens": 60, "cost": 0.5, "rpm": 0.2, "tpm": 6})
}

func TestUsageRatesIntegrationAccountLiteralFilters(t *testing.T) {
	f := newUsageRatesFixture(t)
	ctx := context.Background()
	for _, item := range []struct{ id, name string }{{f.group, "Rates_group%"}, {f.otherGroup, "RatesXgroupQ"}} {
		if _, err := f.s.db.Exec(ctx, `update groups set display_name=$2 where id=$1`, item.id, item.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.s.db.Exec(ctx, `update api_keys set name='Rates_key%' where id=$1`, f.key); err != nil {
		t.Fatal(err)
	}
	var decoyKey string
	if err := f.s.db.QueryRow(ctx, `insert into api_keys(id,user_id,name,key_prefix,secret_hash) values(gen_random_uuid(),$1,'RatesXkeyQ','fake','literal-decoy') returning id`, f.user).Scan(&decoyKey); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"literal_%one", "literalXYtwo", "literal_%group", "literal_%key"} {
		row := f.record(id)
		if id == "literal_%group" {
			row.group = f.otherGroup
		}
		if id == "literal_%key" {
			row.key = decoyKey
		}
		f.insert(t, row)
	}
	account := accountContext{userID: f.user, workspaceID: f.workspace}
	for _, filter := range []url.Values{
		{"group_name": {"Rates_group%"}},
		{"key_name": {"Rates_key%"}},
		{"request_id": {"literal_%"}},
		{"group_name": {"Rates_group%"}, "key_name": {"Rates_key%"}, "request_id": {"literal_%"}},
	} {
		want := 3
		if len(filter) == 3 {
			want = 1
		}
		filter.Set("start", f.start.Format(time.RFC3339))
		filter.Set("end", f.end.Format(time.RFC3339))
		body := fetchUsageRates(t, f.s.accountUsage, filter, account)
		if len(body["data"].([]any)) != want {
			t.Fatalf("filter=%v rows=%d, want %d", filter, len(body["data"].([]any)), want)
		}
		assertUsageRatesNumbers(t, body["stats"].(map[string]any), map[string]float64{"requests": float64(want), "tokens": float64(want * 30), "rpm": float64(want) / 10, "tpm": float64(want * 3)})
	}
}
