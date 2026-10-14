//go:build integration

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// subscriptionAPIHarness drives the real HTTP routes with session cookies so the
// JSON contract the console consumes is exercised end to end.
type subscriptionAPIHarness struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
}

func (h subscriptionAPIHarness) request(method, path, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, "http://router.example"+path, strings.NewReader(body))
	req.Header.Set("X-Xinghai-Request", "1")
	req.Header.Set("Origin", "http://router.example")
	req.Header.Set("Content-Type", "application/json")
	if h.cookie != nil {
		req.AddCookie(h.cookie)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h subscriptionAPIHarness) login(email string) subscriptionAPIHarness {
	h.t.Helper()
	rec := h.request(http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"password123"}`)
	if rec.Code != http.StatusOK {
		h.t.Fatalf("login %s: status=%d body=%s", email, rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			h.cookie = cookie
			return h
		}
	}
	h.t.Fatalf("login %s set no session cookie", email)
	return h
}

func TestIntegrationSubscriptionAPIContract(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	adminID, _ := integrationUser(t, db, "sub-admin@example.com", 0)
	if _, err := db.Exec(ctx, `update users set role='admin' where id=$1`, adminID); err != nil {
		t.Fatal(err)
	}
	buyerID, _ := integrationUser(t, db, "sub-buyer@example.com", 500)
	s := integrationService(t, db)
	admin := subscriptionAPIHarness{t: t, handler: s.Handler()}.login("sub-admin@example.com")
	buyer := subscriptionAPIHarness{t: t, handler: s.Handler()}.login("sub-buyer@example.com")

	create := admin.request(http.MethodPost, "/admin/subscription-products", `{"name":"E2E Membership","description":"end to end","sort_order":3,"enabled":true,"tiers":[{"level":1,"name":"Base","max_requests_per_period":100,"max_credit_per_period":50,"model_whitelist":["gpt-4o"],"prices":[{"billing_period":"month","price":"10.00","currency":"CNY"},{"billing_period":"year","price":"100.00","currency":"CNY","max_requests_per_period":1200}]}]}`)
	if create.Code != http.StatusNoContent {
		t.Fatalf("create product status=%d body=%s", create.Code, create.Body.String())
	}

	list := admin.request(http.MethodGet, "/admin/subscription-products", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list products status=%d body=%s", list.Code, list.Body.String())
	}
	var catalog struct {
		Data []subscriptionProduct `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	var product *subscriptionProduct
	for i := range catalog.Data {
		if catalog.Data[i].Name == "E2E Membership" {
			product = &catalog.Data[i]
		}
	}
	if product == nil || len(product.Tiers) != 1 || len(product.Tiers[0].Prices) != 2 {
		t.Fatalf("catalogue = %+v", catalog.Data)
	}
	tierID := product.Tiers[0].ID
	if product.Tiers[0].OveragePolicy != "allow_wallet" || product.Tiers[0].MaxRequestsPerRule == nil || *product.Tiers[0].MaxRequestsPerRule != 100 {
		t.Fatalf("tier = %+v", product.Tiers[0])
	}

	public := buyer.request(http.MethodGet, "/subscription-plans", "")
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), "E2E Membership") {
		t.Fatalf("public catalogue status=%d body=%s", public.Code, public.Body.String())
	}

	buy := buyer.request(http.MethodPost, "/account/subscriptions/balance", `{"tier_id":"`+tierID+`","billing_period":"month"}`)
	if buy.Code != http.StatusCreated {
		t.Fatalf("balance purchase status=%d body=%s", buy.Code, buy.Body.String())
	}
	var purchase struct {
		SubscriptionID string `json:"subscription_id"`
		Amount         string `json:"amount"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(buy.Body.Bytes(), &purchase); err != nil {
		t.Fatal(err)
	}
	if purchase.Status != "paid" || purchase.Amount != "10.00" || purchase.SubscriptionID == "" {
		t.Fatalf("purchase = %+v", purchase)
	}

	owned := buyer.request(http.MethodGet, "/account/subscriptions", "")
	if owned.Code != http.StatusOK {
		t.Fatalf("account subscriptions status=%d body=%s", owned.Code, owned.Body.String())
	}
	var mine struct {
		Data []userSubscription `json:"data"`
	}
	if err := json.Unmarshal(owned.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	if len(mine.Data) != 1 {
		t.Fatalf("subscriptions = %+v, want 1", mine.Data)
	}
	first := mine.Data[0]
	if first.ID != purchase.SubscriptionID || first.ProductName != "E2E Membership" || first.DisplayName != "E2E Membership · Base" || first.BillingPeriod != "month" {
		t.Fatalf("subscription = %+v", first)
	}
	if first.MaxRequestsPerRule == nil || *first.MaxRequestsPerRule != 100 || first.Price != "10.00" {
		t.Fatalf("period quota = %+v", first)
	}
	if first.UsageRequests != 0 || len(first.ModelQuotas) != 0 {
		t.Fatalf("usage = %+v", first)
	}

	// Buying the same tier for a year merges into the instance: still one row, the
	// period moves to the year price, and the year override becomes the quota.
	buyYear := buyer.request(http.MethodPost, "/account/subscriptions/balance", `{"tier_id":"`+tierID+`","billing_period":"year"}`)
	if buyYear.Code != http.StatusCreated {
		t.Fatalf("yearly purchase status=%d body=%s", buyYear.Code, buyYear.Body.String())
	}
	owned = buyer.request(http.MethodGet, "/account/subscriptions", "")
	if err := json.Unmarshal(owned.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	if len(mine.Data) != 1 || mine.Data[0].ID != first.ID {
		t.Fatalf("after the yearly purchase subscriptions = %+v, want the same single instance", mine.Data)
	}
	if mine.Data[0].BillingPeriod != "year" || mine.Data[0].Price != "100.00" {
		t.Fatalf("after the yearly purchase = %+v", mine.Data[0])
	}
	if mine.Data[0].MaxRequestsPerRule == nil || *mine.Data[0].MaxRequestsPerRule != 1200 {
		t.Fatalf("yearly period quota = %+v", mine.Data[0])
	}

	orders := buyer.request(http.MethodGet, "/account/subscription-orders", "")
	if orders.Code != http.StatusOK {
		t.Fatalf("orders status=%d body=%s", orders.Code, orders.Body.String())
	}
	var orderPage struct {
		Data []subscriptionOrder `json:"data"`
	}
	if err := json.Unmarshal(orders.Body.Bytes(), &orderPage); err != nil {
		t.Fatal(err)
	}
	if len(orderPage.Data) != 2 {
		t.Fatalf("orders = %+v, want 2", orderPage.Data)
	}
	for _, order := range orderPage.Data {
		if order.PlanName != "E2E Membership · Base" || order.TierID != tierID {
			t.Fatalf("order snapshot = %+v", order)
		}
	}
	if orderPage.Data[0].PeriodKind != "renewal" {
		t.Fatalf("newest order = %+v, want a renewal", orderPage.Data[0])
	}

	// The admin sees the same instance with its tier identity.
	all := admin.request(http.MethodGet, "/admin/subscriptions", "")
	if all.Code != http.StatusOK || !strings.Contains(all.Body.String(), tierID) {
		t.Fatalf("admin subscriptions status=%d body=%s", all.Code, all.Body.String())
	}

	// Explicit admin grants create separate instances for the same tier.
	grant := admin.request(http.MethodPost, "/admin/users/"+buyerID+"/subscriptions", `{"tier_id":"`+tierID+`","billing_period":"month","auto_renew":false}`)
	if grant.Code != http.StatusOK {
		t.Fatalf("second instance grant status=%d body=%s", grant.Code, grant.Body.String())
	}
	var instances []string
	if err := db.QueryRow(ctx, `select array_agg(id::text order by created_at,id) from user_subscriptions where user_id=$1 and tier_id=$2`, buyerID, tierID).Scan(&instances); err != nil {
		t.Fatal(err)
	}
	if len(instances) != 2 {
		t.Fatalf("same tier has %d instances, want 2", len(instances))
	}

	keyResponse := buyer.request(http.MethodPost, "/account/keys", `{"name":"Bound key","expires_at":"","group_id":"","subscription_id":"`+first.ID+`"}`)
	if keyResponse.Code != http.StatusCreated {
		t.Fatalf("create bound key status=%d body=%s", keyResponse.Code, keyResponse.Body.String())
	}
	var keyResult struct {
		Key            string `json:"key"`
		SubscriptionID string `json:"subscription_id"`
	}
	if err := json.Unmarshal(keyResponse.Body.Bytes(), &keyResult); err != nil {
		t.Fatal(err)
	}
	if keyResult.SubscriptionID != first.ID {
		t.Fatalf("created key subscription_id=%q, want %q", keyResult.SubscriptionID, first.ID)
	}
	loadedKey, err := s.loadAPIKeyContext(ctx, keyResult.Key)
	if err != nil {
		t.Fatal(err)
	}
	if loadedKey.subscriptionID != first.ID {
		t.Fatalf("authenticated key subscription_id=%q, want %q", loadedKey.subscriptionID, first.ID)
	}
	if _, err := db.Exec(ctx, `update user_subscriptions set remaining_requests=0 where id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	boundAccess := s.subscriptionCoversModel(ctx, loadedKey, "gpt-4o")
	if boundAccess.Covered {
		t.Fatal("bound key borrowed the other same-tier instance after its own quota ran out")
	}
	loadedKey.subscriptionID = ""
	autoAccess := s.subscriptionCoversModel(ctx, loadedKey, "gpt-4o")
	if !autoAccess.Covered || autoAccess.SubscriptionID == first.ID {
		t.Fatalf("unbound key did not select the other instance: %+v", autoAccess)
	}

	// A product that still has subscribers refuses to be removed without force.
	remove := admin.request(http.MethodDelete, "/admin/subscription-products/"+product.ID, "")
	if remove.Code != http.StatusConflict || !strings.Contains(remove.Body.String(), "subscription_in_use") {
		t.Fatalf("delete product status=%d body=%s", remove.Code, remove.Body.String())
	}
}
