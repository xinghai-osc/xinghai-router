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

func TestBatchExtendSubscriptionsRejectsInvalidDaysBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`not-json`,
		`{"plan_id":"p1","days":0}`,
		`{"plan_id":"p1","days":-1}`,
		`{"plan_id":"p1","days":3651}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/subscriptions/extend", strings.NewReader(body))
		(&Service{}).batchExtendSubscriptions(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}
