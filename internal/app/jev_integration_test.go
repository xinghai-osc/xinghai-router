//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const jevIntegrationBody = `{"model":"jev-latest","state":{"ticket":9007199254740993},"questions":{"ping":{"type":"noul","instructions":"Is this a health check?"}}}`
const jevIntegrationResponse = `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":7,"output_tokens":1}}`

func jevIntegrationFixture(t *testing.T, upstreamURL string, retries int) (*Service, *pgxpool.Pool, string, int64) {
	t.Helper()
	db, _ := integrationPool(t)
	t.Cleanup(db.Close)
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "jev-integration@example.com", 100)
	secret := "sk-xh-jev-integration"
	integrationKey(t, db, userID, secret)
	var channelID int64
	if err := db.QueryRow(context.Background(), `insert into channels(name,base_url,api_key,models,provider,test_model,user_id,auto_disable) values('jev',$1,'upstream-key','["jev-latest"]','jev','jev-latest',$2,false) returning id`, upstreamURL, userID).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `insert into pricing_rules(id,model,input_per_million,output_per_million) values(gen_random_uuid(),'jev-latest',1,1)`); err != nil {
		t.Fatal(err)
	}
	s := integrationService(t, db)
	s.reliabilityData = newTTLCache[struct{}, reliabilitySettings](time.Minute)
	settings := defaultReliabilitySettings()
	settings.RetryCount = retries
	settings.AutoDisableStatusCodes = ""
	settings.AutoDisableKeywords = ""
	settings.compile()
	s.reliabilityData.store(struct{}{}, settings)
	return s, db, secret, channelID
}

func jevIntegrationCall(s *Service, secret, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestIntegrationJEVProviderConfiguration(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer upstream-key" {
			t.Errorf("unexpected upstream request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, jevIntegrationResponse)
	}))
	defer upstream.Close()
	s, db, secret, channelID := jevIntegrationFixture(t, upstream.URL, 0)
	if _, err := db.Exec(context.Background(), `delete from channels where id=$1`, channelID); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"name": "direct-jev", "base_url": upstream.URL, "provider": "jev",
		"api_keys": "upstream-key", "models": []string{"jev-latest"}, "user_email": "jev-integration@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.createChannel(rec, httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(string(payload))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("create response=%s err=%v", rec.Body.String(), err)
	}
	var provider, format string
	if err := db.QueryRow(context.Background(), `select provider,upstream_format from channels where id=$1`, created.ID).Scan(&provider, &format); err != nil || provider != "jev" || format != "" {
		t.Fatalf("stored provider=%s format=%s err=%v", provider, format, err)
	}
	for _, config := range []struct{ provider, format string }{{"jev", ""}, {"custom", "jev"}, {"jev", ""}} {
		updateBody, err := json.Marshal(map[string]any{
			"name": "direct-jev", "base_url": upstream.URL, "provider": config.provider,
			"upstream_format": config.format, "models": []string{"jev-latest"},
		})
		if err != nil {
			t.Fatal(err)
		}
		update := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/channels/"+created.ID, strings.NewReader(string(updateBody)))
		req.SetPathValue("id", created.ID)
		s.updateChannel(update, req)
		if update.Code != http.StatusOK {
			t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
		}
		rec = jevIntegrationCall(s, secret, "/v1/systemone", jevIntegrationBody)
		if rec.Code != http.StatusOK || rec.Body.String() != jevIntegrationResponse {
			t.Fatalf("provider=%s format=%s status=%d body=%s", config.provider, config.format, rec.Code, rec.Body.String())
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("calls=%d, want 3", calls.Load())
	}
}

func TestIntegrationJEVPassthroughAndIsolation(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer upstream-key" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected upstream request: %s %s %v", r.Method, r.URL.Path, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != jevIntegrationBody {
			t.Errorf("request changed: %s", body)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Typesafe-Request-Id", "jev-upstream-id")
		_, _ = io.WriteString(w, jevIntegrationResponse)
	}))
	defer upstream.Close()
	s, db, secret, channelID := jevIntegrationFixture(t, upstream.URL, 0)
	if _, err := db.Exec(context.Background(), `insert into channels(name,base_url,api_key,models,provider,user_id,priority) select 'not-jev',$1,'wrong-key',models,'openai',user_id,999 from channels where id=$2`, upstream.URL+"/wrong", channelID); err != nil {
		t.Fatal(err)
	}
	rec := jevIntegrationCall(s, secret, "/v1/systemone", jevIntegrationBody)
	if rec.Code != http.StatusOK || rec.Body.String() != jevIntegrationResponse || calls.Load() != 1 {
		t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), calls.Load())
	}
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Header().Get("X-Typesafe-Request-Id") != "jev-upstream-id" {
		t.Fatalf("missing upstream headers: %v", rec.Header())
	}
	var prompt, completion, total int
	var loggedChannel int64
	if err := db.QueryRow(context.Background(), `select prompt_tokens,completion_tokens,total_tokens,channel_id from request_logs where request_id=$1`, rec.Header().Get("X-Request-ID")).Scan(&prompt, &completion, &total, &loggedChannel); err != nil || prompt != 7 || completion != 1 || total != 8 || loggedChannel != channelID {
		t.Fatalf("usage log=%d/%d/%d channel=%d err=%v", prompt, completion, total, loggedChannel, err)
	}
	var billedPrompt, billedOutput int
	if err := db.QueryRow(context.Background(), `select prompt_tokens,completion_tokens from usage_records where request_id=$1`, rec.Header().Get("X-Request-ID")).Scan(&billedPrompt, &billedOutput); err != nil || billedPrompt != 7 || billedOutput != 1 {
		t.Fatalf("billing=%d/%d err=%v", billedPrompt, billedOutput, err)
	}
	if _, err := db.Exec(context.Background(), `delete from channels where name='not-jev'`); err != nil {
		t.Fatal(err)
	}
	s.invalidateChannels()
	for _, path := range []string{"/v1/chat/completions", "/v1/messages", "/v1/responses", "/v1/images/generations"} {
		rec = jevIntegrationCall(s, secret, path, `{"model":"jev-latest","messages":[{"role":"user","content":"ping"}],"input":"ping","prompt":"ping","max_tokens":8}`)
		if rec.Code < 400 || calls.Load() != 1 {
			t.Fatalf("non-JEV endpoint %s reached JEV: status=%d calls=%d", path, rec.Code, calls.Load())
		}
	}
	if _, err := db.Exec(context.Background(), `update channels set upstream_format='openai_chat' where id=$1`, channelID); err != nil {
		t.Fatal(err)
	}
	s.invalidateChannels()
	rec = jevIntegrationCall(s, secret, "/v1/systemone", jevIntegrationBody)
	if rec.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
		t.Fatalf("JEV reached non-JEV: status=%d calls=%d body=%s", rec.Code, calls.Load(), rec.Body.String())
	}
}

func TestIntegrationJEVAuthValidationPolicyAndPricing(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, jevIntegrationResponse)
	}))
	defer upstream.Close()
	s, db, secret, _ := jevIntegrationFixture(t, upstream.URL, 0)
	for _, test := range []struct {
		name, key, body string
		status          int
	}{
		{"missing key", "", jevIntegrationBody, http.StatusUnauthorized},
		{"wrong key", "not-a-valid-key", jevIntegrationBody, http.StatusUnauthorized},
		{"invalid body", secret, `{}`, http.StatusUnprocessableEntity},
		{"large invalid body", secret, strings.Repeat("x", (2<<20)+1), http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := jevIntegrationCall(s, test.key, "/v1/systemone", test.body)
			if rec.Code != test.status || calls.Load() != 0 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls.Load(), rec.Body.String())
			}
		})
	}
	s.contentPolicyData = newTTLCache[struct{}, contentPolicySnapshot](time.Minute)
	s.contentPolicyData.store(struct{}{}, contentPolicySnapshot{Settings: contentPolicySettings{Mode: "block"}, Rules: []contentPolicyRule{{ID: "test-rule", Term: "blocked-state", Enabled: true, Action: "block"}}})
	rec := jevIntegrationCall(s, secret, "/v1/systemone", strings.Replace(jevIntegrationBody, `{"ticket":9007199254740993}`, `{"arbitrary":{"nested":"blocked-state"}}`, 1))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "content_policy_violation") || calls.Load() != 0 {
		t.Fatalf("policy bypassed: status=%d body=%s calls=%d", rec.Code, rec.Body.String(), calls.Load())
	}
	if _, err := db.Exec(context.Background(), `delete from pricing_rules where model='jev-latest'`); err != nil {
		t.Fatal(err)
	}
	rec = jevIntegrationCall(s, secret, "/v1/systemone", jevIntegrationBody)
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), "pricing_unavailable") || calls.Load() != 0 {
		t.Fatalf("pricing bypassed: status=%d body=%s calls=%d", rec.Code, rec.Body.String(), calls.Load())
	}
}

func TestIntegrationJEVMappingOverridesAndKeyRotation(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := calls.Add(1)
		if r.URL.Path != "/custom/evaluate" {
			t.Errorf("path=%s", r.URL.Path)
		}
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if string(payload["model"]) != `"jev-1.13.0"` || string(payload["state"]) != `{"ticket":9007199254740993}` || string(payload["extension"]) != `"set-by-channel"` {
			t.Errorf("mapped request=%s", payload)
		}
		if _, ok := payload["remove_me"]; ok {
			t.Error("deleted field forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			if r.Header.Get("Authorization") != "Bearer first-key" {
				t.Errorf("first key=%s", r.Header.Get("Authorization"))
			}
			w.WriteHeader(529)
			_, _ = io.WriteString(w, `{"detail":"overloaded"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer second-key" {
			t.Errorf("rotated key=%s", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, jevIntegrationResponse)
	}))
	defer upstream.Close()
	s, db, secret, channelID := jevIntegrationFixture(t, upstream.URL, 1)
	for _, query := range []string{
		`update channels set upstream_path='/custom/evaluate',request_overrides='{"delete":["remove_me"],"set":{"extension":"set-by-channel"}}' where id=$1`,
		`insert into model_routes(id,channel_id,public_model,upstream_model) values(gen_random_uuid(),$1,'jev-latest','jev-1.13.0')`,
		`insert into channel_api_keys(id,channel_id,key_encrypted,priority) values(gen_random_uuid(),$1,'first-key',100),(gen_random_uuid(),$1,'second-key',50)`,
	} {
		if _, err := db.Exec(context.Background(), query, channelID); err != nil {
			t.Fatal(err)
		}
	}
	body := strings.TrimSuffix(jevIntegrationBody, "}") + `,"remove_me":true}`
	rec := jevIntegrationCall(s, secret, "/v1/systemone", body)
	if rec.Code != http.StatusOK || rec.Body.String() != jevIntegrationResponse || calls.Load() != 2 {
		t.Fatalf("status=%d body=%s calls=%d", rec.Code, rec.Body.String(), calls.Load())
	}
}

func TestIntegrationJEVErrorPassthrough(t *testing.T) {
	for _, status := range []int{422, 429, 529} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			const upstreamError = `{"detail":[{"loc":["body","questions"],"msg":"upstream detail https://typesafe.example/help"}]}`
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", "7")
				w.Header().Set("X-Typesafe-Request-Id", "jev-error-id")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, upstreamError)
			}))
			defer upstream.Close()
			s, db, secret, channelID := jevIntegrationFixture(t, upstream.URL, 1)
			if _, err := db.Exec(context.Background(), `insert into channels(name,base_url,api_key,models,provider,user_id,priority) select 'skipped-after',base_url,api_key,models,'openai',user_id,1 from channels where id=$1`, channelID); err != nil {
				t.Fatal(err)
			}
			rec := jevIntegrationCall(s, secret, "/v1/systemone", jevIntegrationBody)
			if rec.Code != status || rec.Body.String() != upstreamError || rec.Header().Get("Retry-After") != "7" || rec.Header().Get("X-Typesafe-Request-Id") != "jev-error-id" || calls.Load() != 2 {
				t.Fatalf("status=%d body=%s headers=%v calls=%d", rec.Code, rec.Body.String(), rec.Header(), calls.Load())
			}
			var loggedChannel int64
			if err := db.QueryRow(context.Background(), `select channel_id from request_logs where request_id=$1`, rec.Header().Get("X-Request-ID")).Scan(&loggedChannel); err != nil || loggedChannel != channelID {
				t.Fatalf("wrong error channel=%d want=%d err=%v", loggedChannel, channelID, err)
			}
		})
	}
}
