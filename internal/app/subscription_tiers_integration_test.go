//go:build integration

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// seedSubscriptionTier inserts one product with a single tier whose monthly
// period grants 100 requests and whose model whitelist only allows gpt-4o.
func seedSubscriptionTier(t *testing.T, s *Service, name string) (productID, tierID string) {
	t.Helper()
	ctx := context.Background()
	if err := s.db.QueryRow(ctx, `insert into subscriptions(id,name,sort_order) values(gen_random_uuid(),$1,1) returning id::text`, name).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `insert into subscription_tiers(id,subscription_id,level,name,model_whitelist,max_requests_per_period,overage_policy) values(gen_random_uuid(),$1,1,'',array['gpt-4o'],100,'allow_wallet') returning id::text`, productID).Scan(&tierID); err != nil {
		t.Fatal(err)
	}
	for _, price := range []struct {
		period string
		amount string
	}{
		{"month", "30.00"},
		{"year", "300.00"},
	} {
		if _, err := s.db.Exec(ctx, `insert into subscription_tier_prices(id,tier_id,billing_period,price,currency) values(gen_random_uuid(),$1,$2,$3,'CNY')`, tierID, price.period, price.amount); err != nil {
			t.Fatal(err)
		}
	}
	return productID, tierID
}

func subscriptionRowCount(t *testing.T, s *Service, userID, tierID string) int {
	t.Helper()
	var count int
	if err := s.db.QueryRow(context.Background(), `select count(*) from user_subscriptions where user_id=$1 and tier_id=$2`, userID, tierID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestIntegrationSubscriptionMergeKeepsOneInstance(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "merge@example.com", 0)
	_, tierID := seedSubscriptionTier(t, s, "Merge")

	monthly, err := s.loadTierPurchase(ctx, db, tierID, "month", true)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstID, renewal, err := s.activateSubscriptionTx(ctx, tx, userID, monthly, subscriptionActivation{})
	if err != nil {
		t.Fatal(err)
	}
	if renewal {
		t.Fatal("first activation must not be a renewal")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var firstEnd time.Time
	var remaining int64
	if err := s.db.QueryRow(ctx, `select current_period_end,remaining_requests from user_subscriptions where id=$1`, firstID).Scan(&firstEnd, &remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 100 {
		t.Fatalf("remaining_requests = %d, want 100", remaining)
	}

	// Buy a year on the same tier: the same instance must be extended, not a
	// second row created, and the period quota resets.
	yearly, err := s.loadTierPurchase(ctx, db, tierID, "year", true)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondID, renewal, err := s.activateSubscriptionTx(ctx, tx, userID, yearly, subscriptionActivation{})
	if err != nil {
		t.Fatal(err)
	}
	if !renewal {
		t.Fatal("second activation must be a renewal")
	}
	if secondID != firstID {
		t.Fatalf("renewal created a new instance: %s vs %s", secondID, firstID)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count := subscriptionRowCount(t, s, userID, tierID); count != 1 {
		t.Fatalf("subscription rows = %d, want 1", count)
	}
	var end time.Time
	var billing string
	var credit int64
	if err := s.db.QueryRow(ctx, `select current_period_end,billing_period,remaining_requests from user_subscriptions where id=$1`, firstID).Scan(&end, &billing, &credit); err != nil {
		t.Fatal(err)
	}
	if billing != "year" {
		t.Fatalf("billing_period = %q, want year", billing)
	}
	wantEnd := subscriptionPeriodEnd(firstEnd, "year")
	if end.Sub(wantEnd).Abs() > time.Second {
		t.Fatalf("period end = %v, want %v (extended from the previous end)", end, wantEnd)
	}
	if credit != 100 {
		t.Fatalf("remaining_requests after renewal = %d, want the reset quota 100", credit)
	}
}

func TestIntegrationSubscriptionCoverageTracksQuota(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "coverage@example.com", 0)
	_, tierID := seedSubscriptionTier(t, s, "Coverage")

	purchase, err := s.loadTierPurchase(ctx, db, tierID, "month", true)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	subID, _, err := s.activateSubscriptionTx(ctx, tx, userID, purchase, subscriptionActivation{})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	key := keyContext{userID: userID, keyID: "key-1"}
	access := s.subscriptionCoversModel(ctx, key, "gpt-4o")
	if !access.Covered || access.SubscriptionID != subID {
		t.Fatalf("covered=%v subscription=%q, want covered by %s", access.Covered, access.SubscriptionID, subID)
	}
	if blocked := s.subscriptionCoversModel(ctx, key, "claude-3"); blocked.Covered {
		t.Fatal("a model outside the whitelist must not be covered")
	}

	if _, err := s.db.Exec(ctx, `update user_subscriptions set remaining_requests=0 where id=$1`, subID); err != nil {
		t.Fatal(err)
	}
	if exhausted := s.subscriptionCoversModel(ctx, key, "gpt-4o"); exhausted.Covered {
		t.Fatal("a subscription without remaining requests must not cover")
	}
	if exhausted := s.subscriptionCoversModel(ctx, key, "gpt-4o"); exhausted.OveragePolicy != "allow_wallet" {
		t.Fatalf("overage policy = %q, want allow_wallet", exhausted.OveragePolicy)
	}
}

func TestIntegrationSubscriptionOrderActivation(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "order@example.com", 0)
	_, tierID := seedSubscriptionTier(t, s, "Order")

	purchase, err := s.loadTierPurchase(ctx, db, tierID, "month", true)
	if err != nil {
		t.Fatal(err)
	}

	// Checkout: a pending instance plus a pending order.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	subID, renewal, err := s.ensureSubscriptionRowTx(ctx, tx, userID, purchase, true)
	if err != nil {
		t.Fatal(err)
	}
	if renewal {
		t.Fatal("first checkout must create a new pending instance")
	}
	if _, err = tx.Exec(ctx, `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period,period_kind,auto_renew) values(gen_random_uuid(),$1,$2,$3,$4,$5,'epay','alipay',$6,$7,'new',false)`,
		"XHORDER1", subID, userID, tierID, purchase.DisplayName, purchase.Price, purchase.BillingPeriod); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := s.db.QueryRow(ctx, `select status from user_subscriptions where id=$1`, subID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("checkout status = %q, want pending", status)
	}

	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	activated, err := s.activateSubscriptionOrderTx(ctx, tx, "XHORDER1", "TRADE1", purchase.Price)
	if err != nil || !activated {
		t.Fatalf("activate (%v, %v), want (true, nil)", activated, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var subStatus, orderStatus string
	var remaining int64
	if err := s.db.QueryRow(ctx, `select status,remaining_requests from user_subscriptions where id=$1`, subID).Scan(&subStatus, &remaining); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `select status from subscription_orders where order_no='XHORDER1'`).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if subStatus != "active" || orderStatus != "paid" || remaining != 100 {
		t.Fatalf("sub=%q order=%q remaining=%d, want active/paid/100", subStatus, orderStatus, remaining)
	}

	// The notify callback is idempotent: replaying it must not extend twice.
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := s.activateSubscriptionOrderTx(ctx, tx, "XHORDER1", "TRADE1", purchase.Price)
	if err != nil || !replayed {
		t.Fatalf("replay (%v, %v), want (true, nil)", replayed, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count := subscriptionRowCount(t, s, userID, tierID); count != 1 {
		t.Fatalf("subscription rows = %d, want 1", count)
	}

	// A later checkout reuses the active instance. Its auto-renew choice rides on
	// the new order and must not touch the instance before the order is paid.
	var priorEnd time.Time
	if err := s.db.QueryRow(ctx, `select current_period_end from user_subscriptions where id=$1`, subID).Scan(&priorEnd); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reused, renewal, err := s.ensureSubscriptionRowTx(ctx, tx, userID, purchase, true)
	if err != nil {
		t.Fatal(err)
	}
	if reused != subID || !renewal {
		t.Fatalf("checkout reused=%s renewal=%v, want the existing instance %s", reused, renewal, subID)
	}
	if _, err = tx.Exec(ctx, `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period,period_kind,auto_renew) values(gen_random_uuid(),'XHORDER3',$1,$2,$3,$4,'epay','alipay',$5,$6,'renewal',true)`,
		subID, userID, tierID, purchase.DisplayName, purchase.Price, purchase.BillingPeriod); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var autoRenew bool
	if err := s.db.QueryRow(ctx, `select auto_renew from user_subscriptions where id=$1`, subID).Scan(&autoRenew); err != nil {
		t.Fatal(err)
	}
	if autoRenew {
		t.Fatal("an unpaid checkout must not flip the instance auto-renew")
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.activateSubscriptionOrderTx(ctx, tx, "XHORDER3", "TRADE3", purchase.Price); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var extended time.Time
	if err := s.db.QueryRow(ctx, `select auto_renew,current_period_end from user_subscriptions where id=$1`, subID).Scan(&autoRenew, &extended); err != nil {
		t.Fatal(err)
	}
	if !autoRenew {
		t.Fatal("a paid checkout must apply its auto-renew choice")
	}
	if !extended.After(priorEnd) {
		t.Fatalf("second purchase end = %v, want later than %v", extended, priorEnd)
	}
	if count := subscriptionRowCount(t, s, userID, tierID); count != 1 {
		t.Fatalf("subscription rows = %d, want 1 after a second purchase", count)
	}
}

func TestIntegrationSubscriptionOrderAmountMismatch(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "mismatch@example.com", 0)
	_, tierID := seedSubscriptionTier(t, s, "Mismatch")
	purchase, err := s.loadTierPurchase(ctx, db, tierID, "month", true)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	subID, _, err := s.ensureSubscriptionRowTx(ctx, tx, userID, purchase, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period) values(gen_random_uuid(),'XHORDER2',$1,$2,$3,'Mismatch','epay','alipay',$4,'month')`, subID, userID, tierID, purchase.Price); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := s.activateSubscriptionOrderTx(ctx, tx, "XHORDER2", "TRADE2", "1.00"); err == nil {
		t.Fatal("expected an amount mismatch error")
	}
	var status string
	if err := s.db.QueryRow(ctx, `select status from subscription_orders where order_no='XHORDER2'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("order status = %q, want pending after a rejected notify", status)
	}
}

func applyProductInput(t *testing.T, s *Service, ctx context.Context, productID string, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/admin/subscription-products/"+productID, strings.NewReader(body))
	in, err := readSubscriptionProductInput(req, s)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `update subscriptions set name=$1,description=$2,sort_order=$3,enabled=$4,updated_at=now() where id=$5`, in.Name, in.Description, in.SortOrder, in.Enabled, productID); err != nil {
		t.Fatal(err)
	}
	if err = s.syncSubscriptionTiersTx(ctx, tx, productID, in.Tiers); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func findProduct(t *testing.T, s *Service, ctx context.Context, id string) subscriptionProduct {
	t.Helper()
	products, err := s.loadSubscriptionCatalog(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range products {
		if product.ID == id {
			return product
		}
	}
	t.Fatalf("product %s not found", id)
	return subscriptionProduct{}
}

func TestIntegrationSubscriptionTierSync(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "catalogue@example.com", 0)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	productID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `insert into subscriptions(id,name) values($1,'Catalogue')`, productID); err != nil {
		t.Fatal(err)
	}
	in, err := readSubscriptionProductInput(httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(`{"name":"Catalogue","tiers":[{"level":1,"name":"Base","max_requests_per_period":100,"prices":[{"billing_period":"month","price":"10.00"},{"billing_period":"year","price":"100.00","max_requests_per_period":1200}]},{"level":2,"name":"Pro","model_quotas":[{"model":"gpt-4o","max_requests_per_period":50}],"prices":[{"billing_period":"month","price":"20.00"}]}]}`)), s)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.syncSubscriptionTiersTx(ctx, tx, productID, in.Tiers); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	product := findProduct(t, s, ctx, productID)
	if len(product.Tiers) != 2 {
		t.Fatalf("tiers = %d, want 2", len(product.Tiers))
	}
	base, pro := product.Tiers[0], product.Tiers[1]
	if base.Level != 1 || base.Name != "Base" || len(base.Prices) != 2 || len(base.ModelQuotas) != 0 {
		t.Fatalf("unexpected base tier: %+v", base)
	}
	if base.Prices[1].BillingPeriod != "year" || base.Prices[1].MaxRequestsPerRule == nil || *base.Prices[1].MaxRequestsPerRule != 1200 {
		t.Fatalf("unexpected yearly override: %+v", base.Prices[1])
	}
	if pro.Level != 2 || len(pro.ModelQuotas) != 1 || pro.ModelQuotas[0].Model != "gpt-4o" {
		t.Fatalf("unexpected pro tier: %+v", pro)
	}

	// Renaming both tiers at once must not trip the unique (subscription, name) index.
	applyProductInput(t, s, ctx, productID, `{"name":"Catalogue","tiers":[{"id":"`+base.ID+`","level":2,"name":"Pro"},{"id":"`+pro.ID+`","level":1,"name":"Base"}]}`)
	product = findProduct(t, s, ctx, productID)
	if product.Tiers[0].Name != "Base" || product.Tiers[0].Level != 1 || product.Tiers[1].Name != "Pro" || len(product.Tiers[0].Prices) != 0 {
		t.Fatalf("tier swap failed: %+v", product.Tiers)
	}

	// A tier with subscribers is disabled when the payload drops it; one without
	// subscribers is deleted outright.
	if _, err := db.Exec(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,billing_period) values(gen_random_uuid(),$1,$2,'active','month')`, userID, pro.ID); err != nil {
		t.Fatal(err)
	}
	extraTierID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into subscription_tiers(id,subscription_id,level,name) values($1,$2,3,'Extra')`, extraTierID, productID); err != nil {
		t.Fatal(err)
	}
	applyProductInput(t, s, ctx, productID, `{"name":"Catalogue","tiers":[{"id":"`+base.ID+`","level":2,"name":"Pro"}]}`)
	product = findProduct(t, s, ctx, productID)
	if len(product.Tiers) != 2 || !product.Tiers[0].Enabled || product.Tiers[0].ID != base.ID || product.Tiers[1].Enabled {
		t.Fatalf("catalogue after reconcile = %+v, want the kept tier enabled and the retired one disabled", product.Tiers)
	}
	var enabled bool
	var retiredLevel int
	if err := db.QueryRow(ctx, `select enabled,level from subscription_tiers where id=$1`, pro.ID).Scan(&enabled, &retiredLevel); err != nil {
		t.Fatalf("the tier with subscribers must be kept: %v", err)
	}
	if enabled || retiredLevel <= product.Tiers[0].Level {
		t.Fatalf("retired tier enabled=%v level=%d, want disabled above the live tiers", enabled, retiredLevel)
	}
	var extraCount int
	if err := db.QueryRow(ctx, `select count(*) from subscription_tiers where id=$1`, extraTierID).Scan(&extraCount); err != nil {
		t.Fatal(err)
	}
	if extraCount != 0 {
		t.Fatal("a dropped tier without subscribers must be deleted")
	}

	// Moving a tier between products consolidates two subscriptions into one.
	otherID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into subscriptions(id,name,sort_order) values($1,'Other',2)`, otherID); err != nil {
		t.Fatal(err)
	}
	otherTierID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `insert into subscription_tiers(id,subscription_id,level,name) values($1,$2,1,'')`, otherTierID, otherID); err != nil {
		t.Fatal(err)
	}
	applyProductInput(t, s, ctx, productID, `{"name":"Catalogue","tiers":[{"id":"`+base.ID+`","level":1,"name":"Base"},{"id":"`+otherTierID+`","level":3,"name":"Moved"}]}`)
	var movedTo string
	if err := db.QueryRow(ctx, `select subscription_id::text from subscription_tiers where id=$1`, otherTierID).Scan(&movedTo); err != nil {
		t.Fatal(err)
	}
	if movedTo != productID {
		t.Fatalf("moved tier belongs to %s, want %s", movedTo, productID)
	}
	product = findProduct(t, s, ctx, productID)
	moved := false
	for _, tier := range product.Tiers {
		if tier.ID == otherTierID && tier.Name == "Moved" {
			moved = true
		}
	}
	if !moved {
		t.Fatalf("catalogue after move = %+v, want the moved tier", product.Tiers)
	}
}

func TestIntegrationMultipleTierInstancesAndKeyBinding(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "multi-instance@example.com", 0)
	_, tierID := seedSubscriptionTier(t, s, "Multi")
	purchase, err := s.loadTierPurchase(ctx, db, tierID, "month", true)
	if err != nil {
		t.Fatal(err)
	}

	createInstance := func() string {
		t.Helper()
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		id, _, err := s.activateSubscriptionTx(ctx, tx, userID, purchase, subscriptionActivation{ForceNew: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return id
	}
	firstID := createInstance()
	secondID := createInstance()
	if firstID == secondID {
		t.Fatal("ForceNew reused an existing subscription instance")
	}
	if count := subscriptionRowCount(t, s, userID, tierID); count != 2 {
		t.Fatalf("same-tier subscription instances = %d, want 2", count)
	}
	if _, err := db.Exec(ctx, `update user_subscriptions set remaining_requests=0 where id=$1`, firstID); err != nil {
		t.Fatal(err)
	}

	key := keyContext{userID: userID, keyID: "bound-key", subscriptionID: firstID}
	access := s.subscriptionCoversModel(ctx, key, "gpt-4o")
	if access.Covered {
		t.Fatal("a key bound to the exhausted instance borrowed another instance's quota")
	}
	key.subscriptionID = secondID
	access = s.subscriptionCoversModel(ctx, key, "gpt-4o")
	if !access.Covered || access.SubscriptionID != secondID {
		t.Fatalf("second instance coverage = %+v, want coverage by %s", access, secondID)
	}
	key.subscriptionID = ""
	access = s.subscriptionCoversModel(ctx, key, "gpt-4o")
	if !access.Covered || access.SubscriptionID != secondID {
		t.Fatalf("automatic selection = %+v, want available second instance", access)
	}

	secret := "sk-xh-multi-instance-bound-key"
	keyID := integrationKey(t, db, userID, secret)
	if _, err := db.Exec(ctx, `update api_keys set subscription_id=$1 where id=$2`, firstID, keyID); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.loadAPIKeyContext(ctx, secret)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.subscriptionID != firstID {
		t.Fatalf("loaded key subscription_id = %q, want %q", loaded.subscriptionID, firstID)
	}
}
