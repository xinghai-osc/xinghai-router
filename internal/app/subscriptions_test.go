package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseCreditAmount(t *testing.T) {
	tests := []struct {
		value string
		want  float64
		ok    bool
	}{
		{"", 0, true},
		{"10", 10, true},
		{"10.5", 10.5, true},
		{"0.01", 0.01, true},
		{"1000000", 1000000, true},
		{"999999.99", 999999.99, true},
		{"1000000.01", 1000000.01, true},
		{"-1", 0, false},
		{"1.001", 0, false},
		{"abc", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseCreditAmount(tt.value)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Fatalf("parseCreditAmount(%q) = (%v, %v), want (%v, %v)", tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{100, "1.00"},
		{1050, "10.50"},
		{1, "0.01"},
		{10000000, "100000.00"},
	}
	for _, tt := range tests {
		if got := formatAmount(tt.cents); got != tt.want {
			t.Fatalf("formatAmount(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

func TestFormatCredit(t *testing.T) {
	if got := formatCredit(10.5); got != "10.5" {
		t.Fatalf("formatCredit(10.5) = %q, want %q", got, "10.5")
	}
	if got := formatCredit(0); got != "0" {
		t.Fatalf("formatCredit(0) = %q, want %q", got, "0")
	}
}

func TestSubscriptionDisplayName(t *testing.T) {
	if got := subscriptionDisplayName("Pro", ""); got != "Pro" {
		t.Fatalf("single tier display = %q, want Pro", got)
	}
	if got := subscriptionDisplayName("Pro", "Flagship"); got != "Pro · Flagship" {
		t.Fatalf("multi tier display = %q, want Pro · Flagship", got)
	}
	if got := subscriptionDisplayName("Pro", "Pro"); got != "Pro" {
		t.Fatalf("duplicate display = %q, want Pro", got)
	}
}

func TestReadSubscriptionProductInputValid(t *testing.T) {
	body := `{"name":"Pro","description":"desc","sort_order":1,"tiers":[{"level":1,"credit_amount":"5","model_whitelist":["gpt-4"],"max_requests_per_period":100,"max_credit_per_period":50,"prices":[{"billing_period":"month","price":"10.00","currency":"cny","credit_amount":"2"},{"billing_period":"year","price":"100.00","max_requests_per_period":1200}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
	product, err := readSubscriptionProductInput(req, &Service{})
	if err != nil {
		t.Fatal(err)
	}
	if product.Name != "Pro" || !product.Enabled || len(product.Tiers) != 1 {
		t.Fatalf("unexpected product: %+v", product)
	}
	tier := product.Tiers[0]
	if tier.Level != 1 || tier.CreditAmount != "5" || tier.MaxRequests == nil || *tier.MaxRequests != 100 {
		t.Fatalf("unexpected tier: %+v", tier)
	}
	if len(tier.Prices) != 2 {
		t.Fatalf("prices = %+v, want 2 entries", tier.Prices)
	}
	if tier.Prices[0].BillingPeriod != "month" || tier.Prices[0].Price != "10.00" || tier.Prices[0].Currency != "CNY" || tier.Prices[0].CreditAmount != "2" {
		t.Fatalf("unexpected monthly price: %+v", tier.Prices[0])
	}
	if tier.Prices[1].MaxRequests == nil || *tier.Prices[1].MaxRequests != 1200 || tier.Prices[1].CreditAmount != "" {
		t.Fatalf("unexpected yearly override: %+v", tier.Prices[1])
	}
}

func TestReadSubscriptionProductInputDefaults(t *testing.T) {
	body := `{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"10"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
	product, err := readSubscriptionProductInput(req, &Service{})
	if err != nil {
		t.Fatal(err)
	}
	if len(product.Tiers) != 1 || product.Tiers[0].CreditAmount != "" {
		t.Fatalf("unexpected product: %+v", product)
	}
	if product.Tiers[0].OveragePolicy != "allow_wallet" {
		t.Fatalf("overage policy = %q, want default allow_wallet", product.Tiers[0].OveragePolicy)
	}
}

func TestReadSubscriptionProductInputRejectsInvalid(t *testing.T) {
	cases := []string{
		`{}`,
		`{"name":""}`,
		`{"name":"` + strings.Repeat("n", 101) + `"}`,
		`{"name":"Pro","sort_order":10001}`,
		`{"name":"Pro","tiers":[{"level":0}]}`,
		`{"name":"Pro","tiers":[{"level":1},{"level":1}]}`,
		`{"name":"Pro","tiers":[{"level":1,"name":"a"},{"level":2,"name":"a"}]}`,
		`{"name":"Pro","tiers":[{"level":1,"name":"a","prices":[{"billing_period":"month","price":"1"}]},{"level":2}]}`,
		`{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"fortnight","price":"1"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"-1"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"100000.01"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"1","currency":"TOOLONGCODE"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"1"},{"billing_period":"month","price":"2"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"credit_amount":"1000000.01"}]}`,
		`{"name":"Pro","tiers":[{"level":1,"overage_policy":"refund"}]}`,
		`{"name":"Pro","tiers":[{"level":1,"model_whitelist":[""]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"max_requests_per_period":-1}]}`,
		`{"name":"Pro","tiers":[{"level":1,"model_quotas":[{"model":"gpt-4o"}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"model_quotas":[{"model":""}]}]}`,
		`{"name":"Pro","tiers":[{"level":1,"model_quotas":[{"model":"gpt-4o","max_requests_per_period":1},{"model":"gpt-4o","max_requests_per_period":2}]}]}`,
		`{"name":"Pro","description":"` + strings.Repeat("d", 2001) + `"}`,
		`not-json`,
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
		if _, err := readSubscriptionProductInput(req, &Service{}); err == nil {
			t.Fatalf("expected rejection for body %s", body)
		}
	}
}

func TestCreateSubscriptionProductRejectsInvalidBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"x","tiers":[{"level":1,"prices":[{"billing_period":"fortnight","price":"1"}]}]}`,
		`{"name":"x","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"bad"}]}]}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
		(&Service{}).createSubscriptionProduct(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestNullableGroupRef(t *testing.T) {
	if nullableGroupRef("") != nil {
		t.Fatal("empty group should be nil")
	}
	if nullableGroupRef("abc") != "abc" {
		t.Fatal("non-empty group should pass through")
	}
}

func TestNullableCredit(t *testing.T) {
	if nullableCredit("") != nil {
		t.Fatal("empty credit should be nil")
	}
	if nullableCredit("5") != "5" {
		t.Fatal("non-empty credit should pass through")
	}
}

func TestReadSubscriptionProductInputUnlimitedCredit(t *testing.T) {
	body := `{"name":"Pro","tiers":[{"level":1,"prices":[{"billing_period":"month","price":"10.00"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
	product, err := readSubscriptionProductInput(req, &Service{})
	if err != nil {
		t.Fatal(err)
	}
	if product.Tiers[0].CreditAmount != "" {
		t.Fatalf("credit amount = %q, want empty for unlimited", product.Tiers[0].CreditAmount)
	}
}

func TestReadSubscriptionProductInputModelQuotas(t *testing.T) {
	body := `{"name":"Pro","tiers":[{"level":1,"model_quotas":[{"model":"gpt-4o","max_requests_per_period":100,"max_credit_per_period":100000}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-products", strings.NewReader(body))
	product, err := readSubscriptionProductInput(req, &Service{})
	if err != nil {
		t.Fatal(err)
	}
	quotas := product.Tiers[0].ModelQuotas
	if len(quotas) != 1 {
		t.Fatalf("model quotas = %+v, want 1 entry", quotas)
	}
	if quotas[0].Model != "gpt-4o" || quotas[0].MaxRequestsPerRule == nil || *quotas[0].MaxRequestsPerRule != 100 {
		t.Fatalf("unexpected model quota: %+v", quotas[0])
	}
}

func TestSubscriptionPeriodEnd(t *testing.T) {
	start := time.Date(2026, 1, 31, 12, 30, 0, 0, time.UTC)
	tests := []struct {
		billing string
		want    time.Time
	}{
		{"hour", time.Date(2026, 1, 31, 13, 30, 0, 0, time.UTC)},
		{"day", time.Date(2026, 2, 1, 12, 30, 0, 0, time.UTC)},
		{"week", time.Date(2026, 2, 7, 12, 30, 0, 0, time.UTC)},
		{"month", time.Date(2026, 3, 3, 12, 30, 0, 0, time.UTC)},
		{"year", time.Date(2027, 1, 31, 12, 30, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		if got := subscriptionPeriodEnd(start, tt.billing); !got.Equal(tt.want) {
			t.Fatalf("subscriptionPeriodEnd(%s) = %v, want %v", tt.billing, got, tt.want)
		}
	}
}

func TestSubscriptionStatusClause(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   string
		ok     bool
	}{
		{"", "status='active'", true},
		{"active", "status='active'", true},
		{"inactive", "status in ('pending','expired','cancelled')", true},
		{"all", "status in ('pending','active','expired','cancelled')", true},
		{"paused", "", false},
	} {
		got, ok := subscriptionStatusClause(tc.status)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("subscriptionStatusClause(%q) = (%q, %v), want (%q, %v)", tc.status, got, ok, tc.want, tc.ok)
		}
	}
}

func TestBatchExtendSubscriptionsRejectsInvalidDaysBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`not-json`,
		`{"tier_id":"t1","days":0}`,
		`{"tier_id":"t1","days":-3651}`,
		`{"tier_id":"t1","days":3651}`,
		`{"tier_id":"t1","days":30,"status":"paused"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/subscriptions/extend", strings.NewReader(body))
		(&Service{}).batchExtendSubscriptions(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestAdminResetSubscriptionQuotaRouteRequiresAuth(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/subscriptions/sub-1/reset-quotas", nil)
	request.Header.Set("X-Xinghai-Request", "1")
	(&Service{}).routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestProviderForModel(t *testing.T) {
	providers := []modelProvider{
		{Name: "OpenAI", Slug: "openai", Prefixes: []string{"gpt-", "o1"}, Priority: 10},
		{Name: "Anthropic", Slug: "anthropic", Prefixes: []string{"claude"}, Priority: 20},
	}
	if got := providerForModel("gpt-4o", providers); got.Slug != "openai" {
		t.Fatalf("got %+v", got)
	}
	if got := providerForModel("CLAUDE-3", providers); got.Slug != "anthropic" {
		t.Fatalf("got %+v", got)
	}
	if got := providerForModel("unknown-model", providers); got.Slug != "other" {
		t.Fatalf("fallback got %+v", got)
	}
}
