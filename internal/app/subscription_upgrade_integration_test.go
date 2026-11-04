//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestIntegrationSubscriptionUpgrade(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()

	adminID, _ := integrationUser(t, db, "upg-admin@example.com", 0)
	if _, err := db.Exec(ctx, `update users set role='admin' where id=$1`, adminID); err != nil {
		t.Fatal(err)
	}
	buyerID, _ := integrationUser(t, db, "upg-buyer@example.com", 1000)

	s := integrationService(t, db)
	admin := subscriptionAPIHarness{t: t, handler: s.Handler()}.login("upg-admin@example.com")
	buyer := subscriptionAPIHarness{t: t, handler: s.Handler()}.login("upg-buyer@example.com")

	create := admin.request(http.MethodPost, "/admin/subscription-products", `{"name":"Upgrade Membership","description":"upgrade flow","sort_order":9,"enabled":true,"tiers":[
		{"level":1,"name":"Core","credit_amount":"100","max_requests_per_period":100,"max_credit_per_period":100,"prices":[{"billing_period":"month","price":"100.00","currency":"CNY"}]},
		{"level":2,"name":"Pro","credit_amount":"300","max_requests_per_period":300,"max_credit_per_period":300,"prices":[{"billing_period":"month","price":"200.00","currency":"CNY"}]},
		{"level":3,"name":"Max","credit_amount":"500","max_requests_per_period":500,"max_credit_per_period":500,"prices":[{"billing_period":"month","price":"200.00","currency":"CNY"}]}]}`)
	if create.Code != http.StatusNoContent {
		t.Fatalf("create product status=%d body=%s", create.Code, create.Body.String())
	}
	var productID, coreTier, proTier, maxTier string
	if err := db.QueryRow(ctx, `select s.id::text,
		(select id::text from subscription_tiers where level=1 and subscription_id=s.id),
		(select id::text from subscription_tiers where level=2 and subscription_id=s.id),
		(select id::text from subscription_tiers where level=3 and subscription_id=s.id)
		from subscriptions s where s.name='Upgrade Membership'`).Scan(&productID, &coreTier, &proTier, &maxTier); err != nil {
		t.Fatal(err)
	}

	// A Core instance halfway through its monthly period, bought at 100.00.
	start := time.Now().AddDate(0, 0, -15)
	end := time.Now().AddDate(0, 0, 15).Truncate(time.Microsecond)
	var instanceID string
	if err := db.QueryRow(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,current_period_start,current_period_end,billing_period,price,currency) values(gen_random_uuid(),$1,$2,'active',$3,$4,'month',100.00,'CNY') returning id::text`, buyerID, coreTier, start, end).Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `update user_subscriptions set remaining_requests=100,remaining_credit=100 where id=$1`, instanceID); err != nil {
		t.Fatal(err)
	}

	quote := buyer.request(http.MethodGet, "/account/subscription-upgrade?tier_id="+proTier+"&billing_period=month", "")
	if quote.Code != http.StatusOK {
		t.Fatalf("upgrade quote status=%d body=%s", quote.Code, quote.Body.String())
	}
	var quoted struct {
		Data *subscriptionUpgrade `json:"data"`
	}
	if err := json.Unmarshal(quote.Body.Bytes(), &quoted); err != nil {
		t.Fatal(err)
	}
	if quoted.Data == nil || quoted.Data.InstanceID != instanceID || quoted.Data.FromTierID != coreTier || quoted.Data.ToTierID != proTier {
		t.Fatalf("upgrade quote = %+v", quoted.Data)
	}
	if cents, _, ok := parsePaymentAmount(quoted.Data.Amount); !ok || cents < 4900 || cents > 5100 {
		t.Fatalf("prorated amount = %q, want about half of the 100.00 difference", quoted.Data.Amount)
	}
	if credit, ok := parseCreditAmount(quoted.Data.CreditAmount); !ok || credit < 99 || credit > 101 {
		t.Fatalf("prorated credit = %q, want about half of the 200 credit difference", quoted.Data.CreditAmount)
	}
	// The tier the instance already owns is a renewal, not an upgrade.
	same := buyer.request(http.MethodGet, "/account/subscription-upgrade?tier_id="+coreTier+"&billing_period=month", "")
	if same.Code != http.StatusOK || string(same.Body.Bytes()) != "{\"data\":null}\n" {
		t.Fatalf("same-tier quote status=%d body=%s", same.Code, same.Body.String())
	}

	buy := buyer.request(http.MethodPost, "/account/subscriptions/balance", `{"tier_id":"`+proTier+`","billing_period":"month"}`)
	if buy.Code != http.StatusCreated {
		t.Fatalf("balance upgrade status=%d body=%s", buy.Code, buy.Body.String())
	}
	if upgraded, err := s.loadSubscriptionUpgrade(ctx, db, buyerID, tierPurchase{TierID: proTier, TierLevel: 2, ProductID: productID, BillingPeriod: "month", Currency: "CNY", Price: "200.00"}); err != nil || upgraded != nil {
		t.Fatalf("the upgraded instance must not upgrade to its own tier again: %+v %v", upgraded, err)
	}

	var tierID, price, currency, kind, orderAmount string
	var periodEnd time.Time
	if err := db.QueryRow(ctx, `select tier_id::text,current_period_end,price::text,currency from user_subscriptions where id=$1`, instanceID).Scan(&tierID, &periodEnd, &price, &currency); err != nil {
		t.Fatal(err)
	}
	if tierID != proTier || price != "200.00" || currency != "CNY" {
		t.Fatalf("upgraded instance tier=%s price=%s currency=%s", tierID, price, currency)
	}
	if !periodEnd.Equal(end) {
		t.Fatalf("period end moved during an upgrade: %s -> %s", end, periodEnd)
	}
	var remainingCredit *float64
	if err := db.QueryRow(ctx, `select remaining_credit from user_subscriptions where id=$1`, instanceID).Scan(&remainingCredit); err != nil {
		t.Fatal(err)
	}
	if remainingCredit == nil || *remainingCredit != 300 {
		t.Fatalf("quota after upgrade = %v, want the new tier's 300", remainingCredit)
	}
	var walletBalance float64
	if err := db.QueryRow(ctx, `select balance from user_wallets where user_id=$1`, buyerID).Scan(&walletBalance); err != nil {
		t.Fatal(err)
	}
	// 1000 - 50 points paid + 100 prorated credit granted.
	if walletBalance != 1050 {
		t.Fatalf("wallet after upgrade = %v, want 1050", walletBalance)
	}
	if err := db.QueryRow(ctx, `select period_kind,amount::text from subscription_orders where subscription_id=$1 order by created_at desc limit 1`, instanceID).Scan(&kind, &orderAmount); err != nil {
		t.Fatal(err)
	}
	if kind != "upgrade" {
		t.Fatalf("order kind = %s, want upgrade", kind)
	}
	if cents, _, ok := parsePaymentAmount(orderAmount); !ok || cents < 4900 || cents > 5100 {
		t.Fatalf("order amount = %s", orderAmount)
	}

	// The payment-notification path activates an upgrade order the same way.
	var epayInstance string
	if err := db.QueryRow(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,current_period_start,current_period_end,billing_period,price,currency) values(gen_random_uuid(),$1,$2,'active',now(),now()+interval '20 days','month',100.00,'CNY') returning id::text`, buyerID, coreTier).Scan(&epayInstance); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `update user_subscriptions set remaining_requests=100,remaining_credit=100 where id=$1`, epayInstance); err != nil {
		t.Fatal(err)
	}
	purchase, err := s.loadTierPurchase(ctx, db, proTier, "month", true)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.loadSubscriptionUpgrade(ctx, db, buyerID, purchase)
	if err != nil || pending == nil || pending.InstanceID != epayInstance {
		t.Fatalf("pending upgrade = %+v err=%v", pending, err)
	}
	orderID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	orderNo := "xhuptest" + orderID[:8]
	if _, err := db.Exec(ctx, `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period,period_kind) values($1,$2,$3,$4,$5,$6,'epay','alipay',$7,'month','upgrade')`,
		orderID, orderNo, epayInstance, buyerID, proTier, purchase.DisplayName, pending.Amount); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	activated, err := s.activateSubscriptionOrderTx(ctx, tx, orderNo, "trade-upgrade-1", pending.Amount)
	if err != nil || !activated {
		t.Fatalf("activate upgrade order = %v err=%v", activated, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var epayTier, epayKind string
	if err := db.QueryRow(ctx, `select us.tier_id::text,o.period_kind from user_subscriptions us join subscription_orders o on o.subscription_id=us.id where o.order_no=$1`, orderNo).Scan(&epayTier, &epayKind); err != nil {
		t.Fatal(err)
	}
	if epayTier != proTier || epayKind != "upgrade" {
		t.Fatalf("notify activation tier=%s kind=%s", epayTier, epayKind)
	}

	// A prorated difference that rounds to zero activates without a payment.
	proPurchase, err := s.loadTierPurchase(ctx, db, maxTier, "month", true)
	if err != nil {
		t.Fatal(err)
	}
	free, err := s.loadSubscriptionUpgrade(ctx, db, buyerID, proPurchase)
	if err != nil || free == nil || free.Amount != "0.00" {
		t.Fatalf("free upgrade quote = %+v err=%v", free, err)
	}
	freeBuy := buyer.request(http.MethodPost, "/account/subscriptions/balance", `{"tier_id":"`+maxTier+`","billing_period":"month"}`)
	if freeBuy.Code != http.StatusCreated || !containsJSONString(freeBuy.Body.Bytes(), "paid") {
		t.Fatalf("free upgrade status=%d body=%s", freeBuy.Code, freeBuy.Body.String())
	}
	var maxTierID string
	if err := db.QueryRow(ctx, `select tier_id::text from user_subscriptions where id=$1`, free.InstanceID).Scan(&maxTierID); err != nil {
		t.Fatal(err)
	}
	if maxTierID != maxTier {
		t.Fatalf("free upgrade left tier=%s, want %s", maxTierID, maxTier)
	}
}

func containsJSONString(data []byte, want string) bool {
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return false
	}
	value, ok := payload["status"].(string)
	return ok && value == want
}
