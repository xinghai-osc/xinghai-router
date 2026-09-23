//go:build integration

package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func billingIntegrationFixture(t *testing.T, upstreamURL string) (*Service, *pgxpool.Pool, string) {
	t.Helper()
	db, _ := integrationPool(t)
	t.Cleanup(db.Close)
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "billing-integration@example.com", 100)
	secret := "sk-xh-billing-integration"
	integrationKey(t, db, userID, secret)
	if _, err := db.Exec(context.Background(), `insert into channels(name,base_url,api_key,models,provider,upstream_format,user_id,auto_disable) values('billing',$1,'fake-upstream-key','["image-model","claude-model"]','openai','openai_chat',$2,false)`, upstreamURL, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,dimension_prices) values
		(gen_random_uuid(),'image-model',1000,0,1000,'{"images":[{"size":"1024x1024","quality":"hd","price":0.25}]}'),
		(gen_random_uuid(),'claude-model',1,0.1,2,'{"cache_write_5m_per_million":1.25,"cache_write_1h_per_million":2}')`); err != nil {
		t.Fatal(err)
	}
	return integrationService(t, db), db, secret
}

func billingIntegrationCall(s *Service, secret, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestIntegrationImageBillingWithoutTokenUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"b64_json":"fake-image"}]}`)
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	r := billingIntegrationCall(s, secret, "/v1/images/generations", `{"model":"image-model","prompt":"test","n":2,"size":"1024x1024","quality":"hd"}`)
	if r.Code != 200 {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	var cost string
	var raw []byte
	if err := db.QueryRow(context.Background(), `select cost::text,billing_snapshot from usage_records where request_id=$1`, r.Header().Get("X-Request-ID")).Scan(&cost, &raw); err != nil {
		t.Fatal(err)
	}
	var snapshot BillingSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if cost != "0.25000000" || snapshot.Usage.ImageCount != 1 || snapshot.Usage.UsageSource != "response_count" || snapshot.Pricing.BillingMode != "image" {
		t.Fatalf("cost=%s snapshot=%s", cost, raw)
	}
	if got, err := snapshot.Recompute(); err != nil || got != cost {
		t.Fatalf("recompute=%s err=%v", got, err)
	}
	var reserved string
	if err := db.QueryRow(context.Background(), `select reserved::text from user_wallets where user_id=(select user_id from usage_records where request_id=$1)`, r.Header().Get("X-Request-ID")).Scan(&reserved); err != nil || reserved != cost {
		t.Fatalf("reserved=%s err=%v", reserved, err)
	}
	if _, err := db.Exec(context.Background(), `update pricing_rules set dimension_prices='{}' where model='image-model'`); err != nil {
		t.Fatal(err)
	}
	s.pricingCache.clear()
	r = billingIntegrationCall(s, secret, "/v1/images/generations", `{"model":"image-model","prompt":"test","size":"1024x1024","quality":"hd"}`)
	if r.Code != 402 || !strings.Contains(r.Body.String(), "pricing_unavailable") {
		t.Fatalf("unpriced image status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestIntegrationBillingSubscriptionFacts(t *testing.T) {
	s, db, _ := billingIntegrationFixture(t, "http://127.0.0.1:1")
	var key keyContext
	if err := db.QueryRow(context.Background(), `select user_id::text,id::text from api_keys limit 1`).Scan(&key.userID, &key.keyID); err != nil {
		t.Fatal(err)
	}
	var planID, subID string
	if err := db.QueryRow(context.Background(), `insert into subscription_plans(id,name,billing_period) values(gen_random_uuid(),'billing-test','month') returning id::text`).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(context.Background(), `insert into user_subscriptions(id,user_id,plan_id,status,current_period_start,current_period_end,remaining_requests,remaining_credit) values(gen_random_uuid(),$1,$2,'active',now(),now()+interval '1 month',10,10) returning id::text`, key.userID, planID).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	pricing := pricingRule{found: true, input: 1, multiplier: 1, exchangeRate: 1}
	for _, test := range []struct {
		name  string
		facts UsageFacts
	}{
		{"priced", UsageFacts{InputTokens: 1000000, UsageSource: "upstream"}},
		{"unpriced", UsageFacts{ToolCalls: 1, UsageSource: "upstream"}},
		{"missing", UsageFacts{UsageSource: "missing"}},
	} {
		ctx := context.WithValue(context.Background(), requestIDKey{}, "sub-"+test.name)
		ctx = context.WithValue(ctx, subscriptionCoveredKey{}, subscriptionAccess{Covered: true, SubscriptionID: subID})
		s.logRequest(ctx, key, 0, "", "claude-model", 200, 0, 0, 0, 0, "", "")
		for i := 0; i < 2; i++ {
			s.settleSubscriptionFacts(ctx, key, "claude-model", test.facts, pricing, 1)
		}
	}
	var remaining int
	var credit string
	if err := db.QueryRow(context.Background(), `select remaining_requests,remaining_credit::text from user_subscriptions where id=$1`, subID).Scan(&remaining, &credit); err != nil || remaining != 7 || credit != "9.00000000" {
		t.Fatalf("remaining=%d credit=%s err=%v", remaining, credit, err)
	}
}

func TestIntegrationBillingReconcileConcurrentAndCaps(t *testing.T) {
	for _, test := range []struct {
		name          string
		balance, held float64
		want, status  string
	}{
		{"full", 10, 0.25, "1.00000000", "settled"},
		{"cap", 0.5, 0.25, "0.25000000", "hold_capped"},
		{"zero-hold", 0, 0, "0.00000000", "settlement_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, db, _ := billingIntegrationFixture(t, "http://127.0.0.1:1")
			var key keyContext
			if err := db.QueryRow(context.Background(), `select user_id::text,id::text from api_keys limit 1`).Scan(&key.userID, &key.keyID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(context.Background(), `update user_wallets set balance=$1 where user_id=$2`, test.balance, key.userID); err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), requestIDKey{}, "reconcile-"+test.name)
			s.logRequest(ctx, key, 0, "", "claude-model", 200, 1000000, 0, 1000000, 0, "", "")
			held, err := s.reserveAmount(ctx, key, "claude-model", test.held)
			if err != nil {
				t.Fatal(err)
			}
			pricing := pricingRule{found: true, input: 1, multiplier: 1, exchangeRate: 1}
			facts := UsageFacts{InputTokens: 1000000, UsageSource: "upstream"}
			if test.name == "full" {
				bill, _ := calculateBill(facts, pricing, 1)
				bill.Status = "settlement_failed"
				s.recordUnsettledUsage(ctx, key, "claude-model", bill)
			}
			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); s.settleUsageFacts(ctx, key, held, "claude-model", facts, pricing, 1) }()
			}
			wg.Wait()
			s.releaseReservation(ctx, key, held)
			var raw []byte
			var cost, reserved string
			if err := db.QueryRow(context.Background(), `select cost::text,billing_snapshot from usage_records where request_id=$1`, requestID(ctx)).Scan(&cost, &raw); err != nil {
				t.Fatal(err)
			}
			var snapshot BillingSnapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(context.Background(), `select reserved::text from user_wallets where user_id=$1`, key.userID).Scan(&reserved); err != nil {
				t.Fatal(err)
			}
			if cost != test.want || reserved != test.want || snapshot.Status != test.status || snapshot.ComputedAmount != "1.00000000" {
				t.Fatalf("cost=%s reserved=%s snapshot=%s", cost, reserved, raw)
			}
		})
	}
}

func TestIntegrationBillingPriceConfiguration(t *testing.T) {
	s, db, _ := billingIntegrationFixture(t, "http://127.0.0.1:1")
	var userID string
	if err := db.QueryRow(context.Background(), `select user_id::text from channels where name='billing'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		body          string
		status, count int
	}{
		{`{"model":"image-model","input_per_million":1,"output_per_million":2}`, 200, 1},
		{`{"model":"image-model","dimension_prices":null}`, 400, 1},
		{`{"model":"image-model","dimension_prices":{}}`, 200, 0},
		{`{"model":"image-model","dimension_prices":{"images":[{"size":"*","quality":"*","price":0.1}]}}`, 200, 1},
	} {
		r := httptest.NewRequest(http.MethodPost, "/admin/pricing", strings.NewReader(test.body))
		r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{userID: userID}))
		w := httptest.NewRecorder()
		s.upsertPricing(w, r)
		if w.Code != test.status {
			t.Fatalf("status=%d want=%d body=%s", w.Code, test.status, w.Body.String())
		}
		if price := s.pricingFor(context.Background(), "image-model"); !price.found || len(price.dimensions.Images) != test.count {
			t.Fatalf("loaded pricing=%+v want images=%d", price, test.count)
		}
	}
}

func TestIntegrationCacheTTLBillingAndReplay(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_test","type":"message","role":"assistant","model":"claude-model","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":20,"ephemeral_1h_input_tokens":10},"output_tokens":5}}`)
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	if _, err := db.Exec(context.Background(), `update channels set provider='anthropic',upstream_format='anthropic'`); err != nil {
		t.Fatal(err)
	}
	r := billingIntegrationCall(s, secret, "/v1/chat/completions", `{"model":"claude-model","messages":[{"role":"user","content":"test"}],"max_tokens":100}`)
	if r.Code != 200 {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	var cost, userID, keyID string
	var raw []byte
	if err := db.QueryRow(context.Background(), `select cost::text,billing_snapshot,user_id::text,api_key_id::text from usage_records where request_id=$1`, r.Header().Get("X-Request-ID")).Scan(&cost, &raw, &userID, &keyID); err != nil {
		t.Fatal(err)
	}
	var snapshot BillingSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if cost != "0.00006700" || snapshot.Usage.InputTokens != 10 || snapshot.Usage.CacheReadTokens != 20 || snapshot.Usage.CacheWrite5mTokens != 20 || snapshot.Usage.CacheWrite1hTokens != 10 {
		t.Fatalf("cost=%s snapshot=%s", cost, raw)
	}
	if _, err := db.Exec(context.Background(), `update pricing_rules set input_per_million=900,dimension_prices='{}'`); err != nil {
		t.Fatal(err)
	}
	if got, err := snapshot.Recompute(); err != nil || got != cost {
		t.Fatalf("recompute=%s err=%v", got, err)
	}
	ctx := context.WithValue(context.Background(), requestIDKey{}, r.Header().Get("X-Request-ID"))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			held := s.settleUsageFacts(ctx, keyContext{userID: userID, keyID: keyID}, reservation{amount: 0.000067}, "claude-model", snapshot.Usage, pricingRule{found: true, input: 1, cachedInput: 0.1, output: 2, multiplier: 1, exchangeRate: 1, dimensions: snapshot.Pricing.DimensionPrices}, 1)
			if held.amount != 0 {
				t.Errorf("replayed settlement retained hold %v", held.amount)
			}
		}()
	}
	wg.Wait()
	var reserved string
	if err := db.QueryRow(context.Background(), `select reserved::text from user_wallets where user_id=$1`, userID).Scan(&reserved); err != nil || reserved != cost {
		t.Fatalf("replay reserved=%s err=%v", reserved, err)
	}
	var count int
	if err := db.QueryRow(context.Background(), `select count(*) from wallet_settlements where request_id=$1`, r.Header().Get("X-Request-ID")).Scan(&count); err != nil || count != 1 {
		t.Fatalf("settlements=%d err=%v", count, err)
	}
}
