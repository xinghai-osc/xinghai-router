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
)

func TestIntegrationDirectStreamInterruptedBilling(t *testing.T) {
	for _, usage := range []bool{true, false} {
		name := "without-usage"
		if usage {
			name = "with-usage"
		}
		t.Run(name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
				if usage {
					io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":100,\"output_tokens\":50}}}\n\n")
				} else {
					io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n")
				}
			}))
			defer upstream.Close()
			s, db, secret := billingIntegrationFixture(t, upstream.URL)
			if _, err := db.Exec(context.Background(), `update channels set upstream_format='openai'`); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"claude-model","input":"test","stream":true}`))
			r.Header.Set("Authorization", "Bearer "+secret)
			r.Header.Set("Content-Type", "application/json")
			w := &quotaFailWriter{header: make(http.Header), failAt: 3}
			s.Handler().ServeHTTP(w, r)
			if w.writes != 3 {
				t.Fatalf("downstream writes=%d, want failure on third write", w.writes)
			}
			var charges int
			var reserved float64
			if err := db.QueryRow(context.Background(), `select count(*) from wallet_settlements`).Scan(&charges); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(context.Background(), `select reserved from user_wallets where user_id=(select user_id from api_keys where secret_hash=$1)`, hashSecret(secret)).Scan(&reserved); err != nil {
				t.Fatal(err)
			}
			if usage {
				var prompt, completion int
				if err := db.QueryRow(context.Background(), `select prompt_tokens,completion_tokens from usage_records where status='settled'`).Scan(&prompt, &completion); err != nil {
					t.Fatal(err)
				}
				if charges != 1 || reserved <= 0 || prompt != 100 || completion != 50 {
					var snapshot []byte
					_ = db.QueryRow(context.Background(), `select billing_snapshot from usage_records where status='settled'`).Scan(&snapshot)
					t.Fatalf("charges=%d reserved=%v prompt=%d completion=%d snapshot=%s", charges, reserved, prompt, completion, snapshot)
				}
			} else {
				if charges != 0 || reserved != 0 {
					t.Fatalf("missing usage charged: charges=%d reserved=%v", charges, reserved)
				}
				var raw []byte
				if err := db.QueryRow(context.Background(), `select billing_snapshot from usage_records where status='missing_usage' and cost=0 and completion_tokens=0`).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				var bill BillingSnapshot
				if err := json.Unmarshal(raw, &bill); err != nil {
					t.Fatal(err)
				}
				if obs := bill.StreamObservation; obs == nil || obs.DeliveredBytes == 0 || obs.ErrorCode == "" {
					t.Fatalf("missing interruption evidence: %s", raw)
				}
			}
		})
	}
}

func TestIntegrationSubscriptionModelQuotaSettlement(t *testing.T) {
	for _, dimension := range []string{"requests", "credit"} {
		t.Run(dimension, func(t *testing.T) {
			db, _ := integrationPool(t)
			defer db.Close()
			resetIntegrationDatabase(t, db)
			s := integrationService(t, db)
			ctx := context.Background()
			user, _ := integrationUser(t, db, "subscription-scope@example.test", 10)
			keyID := integrationKey(t, db, user, "sk-subscription-scope")
			_, tier := seedSubscriptionTier(t, s, "model-scoped")
			if _, err := db.Exec(ctx, `update subscription_tiers set max_requests_per_period=10,max_credit_per_period=0.5 where id=$1`, tier); err != nil {
				t.Fatal(err)
			}
			var requests, credit any
			if dimension == "requests" {
				requests = 2
			} else {
				credit = 0.25
			}
			if _, err := db.Exec(ctx, `insert into subscription_tier_model_quotas(tier_id,model,max_requests_per_period,max_credit_per_period) values($1,'gpt-4o',$2,$3)`, tier, requests, credit); err != nil {
				t.Fatal(err)
			}
			purchase, err := s.loadTierPurchase(ctx, db, tier, "month", true)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			sub, _, err := s.activateSubscriptionTx(ctx, tx, user, purchase, subscriptionActivation{})
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			key := keyContext{userID: user, keyID: keyID}
			access := requireSubscriptionAccess(t, s, ctx, key, "gpt-4o")
			if !access.Covered {
				t.Fatalf("fractional credit was incorrectly rejected: %+v", access)
			}
			ctx = context.WithValue(ctx, subscriptionCoveredKey{}, access)
			ctx = context.WithValue(ctx, requestIDKey{}, "model-settlement")
			s.logRequest(ctx, key, 0, "", "gpt-4o", 200, 100000, 0, 100000, 0, "", "")
			for i := 0; i < 2; i++ {
				s.settleSubscriptionFacts(ctx, key, "gpt-4o", UsageFacts{InputTokens: 100000, UsageSource: "upstream"}, pricingRule{found: true, input: 1, multiplier: 1, exchangeRate: 1}, 1)
			}
			var parentRequests int
			var parentCredit float64
			var childRequests *int
			var childCredit *float64
			if err := db.QueryRow(ctx, `select remaining_requests,remaining_credit from user_subscriptions where id=$1`, sub).Scan(&parentRequests, &parentCredit); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(ctx, `select remaining_requests,remaining_credit from user_subscription_model_usage where subscription_id=$1 and model='gpt-4o'`, sub).Scan(&childRequests, &childCredit); err != nil {
				t.Fatal(err)
			}
			if dimension == "requests" {
				if parentRequests != 10 || parentCredit != 0.4 || childRequests == nil || *childRequests != 1 || childCredit != nil {
					t.Fatalf("parent=%d/%v child=%v/%v", parentRequests, parentCredit, childRequests, childCredit)
				}
			} else if parentRequests != 9 || parentCredit != 0.5 || childRequests != nil || childCredit == nil || *childCredit != 0.15 {
				t.Fatalf("parent=%d/%v child=%v/%v", parentRequests, parentCredit, childRequests, childCredit)
			}
		})
	}
}

func TestIntegrationPartialStreamRetainsReportedUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"choices":[{"delta":{"content":"ok"}}],"usage":{"prompt_tokens":17}}`+"\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	r := billingIntegrationCall(s, secret, "/v1/chat/completions", `{"model":"claude-model","messages":[{"role":"user","content":"test"}],"stream":true,"max_tokens":100000}`)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "ok") {
		t.Fatalf("stream=%d %s", r.Code, r.Body.String())
	}
	var raw []byte
	var reserved float64
	if err := db.QueryRow(context.Background(), `select billing_snapshot,w.reserved from usage_records u join user_wallets w on w.user_id=u.user_id where u.request_id=$1 and u.status='missing_usage' and u.cost=0 and u.prompt_tokens=17 and u.completion_tokens=0 and not exists(select 1 from wallet_settlements where request_id=$1)`, r.Header().Get("X-Request-ID")).Scan(&raw, &reserved); err != nil {
		t.Fatal(err)
	}
	var bill BillingSnapshot
	if err := json.Unmarshal(raw, &bill); err != nil {
		t.Fatal(err)
	}
	if obs := bill.StreamObservation; reserved != 0 || obs == nil || !obs.InputReported || obs.OutputReported || obs.ErrorCode != "" || obs.DeliveredBytes != int64(r.Body.Len()) {
		t.Fatalf("reserved=%v snapshot=%s", reserved, raw)
	}
}

func TestIntegrationSubscriptionStreamWithoutUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	_, tier := seedSubscriptionTier(t, s, "stream-reconciliation")
	if _, err := db.Exec(ctx, `update subscription_tiers set model_whitelist=array['claude-model'],max_credit_per_period=10 where id=$1`, tier); err != nil {
		t.Fatal(err)
	}
	var subID string
	if err := db.QueryRow(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,current_period_start,current_period_end,remaining_requests,remaining_credit,billing_period) select gen_random_uuid(),user_id,$1,'active',now(),now()+interval '1 month',10,10,'month' from api_keys where secret_hash=$2 returning id::text`, tier, hashSecret(secret)).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	r := billingIntegrationCall(s, secret, "/v1/chat/completions", `{"model":"claude-model","messages":[{"role":"user","content":"test"}],"stream":true,"max_tokens":100000}`)
	if r.Code != 200 {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	var requests int
	var credit float64
	if err := db.QueryRow(ctx, `select remaining_requests,remaining_credit from user_subscriptions where id=$1`, subID).Scan(&requests, &credit); err != nil || requests != 9 || credit != 10 {
		t.Fatalf("requests=%d credit=%v err=%v", requests, credit, err)
	}
	var target string
	if err := db.QueryRow(ctx, `select billing_snapshot->'stream_observation'->>'subscription_id' from usage_records where request_id=$1 and status='missing_usage' and cost=0 and completion_tokens=0`, r.Header().Get("X-Request-ID")).Scan(&target); err != nil || target != subID {
		t.Fatalf("reconciliation subscription=%s err=%v", target, err)
	}
}

func TestIntegrationSubscriptionLookupFailureStopsGateway(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer upstream.Close()
	s, db, secret := billingIntegrationFixture(t, upstream.URL)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `alter table subscription_tier_model_quotas rename to unavailable_subscription_quotas`); err != nil {
		t.Fatal(err)
	}
	r := billingIntegrationCall(s, secret, "/v1/chat/completions", `{"model":"claude-model","messages":[{"role":"user","content":"test"}]}`)
	if r.Code != http.StatusServiceUnavailable || !strings.Contains(r.Body.String(), "subscription_unavailable") || calls.Load() != 0 {
		t.Fatalf("status=%d body=%s upstream=%d", r.Code, r.Body.String(), calls.Load())
	}
	var entries int
	if err := db.QueryRow(ctx, `select count(*) from wallet_ledger where request_id=$1`, r.Header().Get("X-Request-ID")).Scan(&entries); err != nil || entries != 0 {
		t.Fatalf("wallet entries=%d err=%v", entries, err)
	}
}
