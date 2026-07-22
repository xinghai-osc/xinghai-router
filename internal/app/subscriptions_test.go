package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func TestReadSubscriptionPlanInputValid(t *testing.T) {
	body := `{"name":"Pro","description":"desc","price":"10.00","billing_period":"month","credit_amount":"5","model_whitelist":["gpt-4"],"sort_order":1}`
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-plans", strings.NewReader(body))
	plan, err := readSubscriptionPlanInput(req, &Service{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Name != "Pro" || plan.BillingPeriod != "month" || plan.Price != "10.00" || plan.CreditAmount != "5" || plan.Currency != "CNY" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if len(plan.ModelWhitelist) != 1 || plan.ModelWhitelist[0] != "gpt-4" || !plan.Enabled {
		t.Fatalf("unexpected whitelist/enabled: %+v", plan)
	}
}

func TestReadSubscriptionPlanInputRejectsInvalid(t *testing.T) {
	cases := []string{
		`{}`,
		`{"name":"","price":"1","billing_period":"month","credit_amount":"0"}`,
		`{"name":"Plan","price":"1","billing_period":"week","credit_amount":"0"}`,
		`{"name":"Plan","price":"-1","billing_period":"month","credit_amount":"0"}`,
		`{"name":"Plan","price":"1","billing_period":"month","credit_amount":"abc"}`,
		`{"name":"Plan","price":"1","billing_period":"month","credit_amount":"0","currency":"TOOLONGCODE"}`,
		`{"name":"Plan","price":"1","billing_period":"month","credit_amount":"0","model_whitelist":[""]}`,
		`not-json`,
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/admin/subscription-plans", strings.NewReader(body))
		if _, err := readSubscriptionPlanInput(req, &Service{}, ""); err == nil {
			t.Fatalf("expected rejection for body %s", body)
		}
	}
}

func TestCreateSubscriptionPlanRejectsInvalidBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"x","price":"1","billing_period":"day","credit_amount":"0"}`,
		`{"name":"x","price":"bad","billing_period":"month","credit_amount":"0"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/subscription-plans", strings.NewReader(body))
		(&Service{}).createSubscriptionPlan(rec, req)
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
