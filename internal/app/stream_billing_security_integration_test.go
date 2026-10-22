//go:build integration

package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
			} else if charges != 0 || reserved != 0 {
				t.Fatalf("missing usage changed billing policy: charges=%d reserved=%v", charges, reserved)
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
			access := s.subscriptionCoversModel(ctx, key, "gpt-4o")
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
