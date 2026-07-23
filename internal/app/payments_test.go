package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEpaySign(t *testing.T) {
	values := url.Values{
		"pid":          {"1001"},
		"type":         {"alipay"},
		"out_trade_no": {"xh123"},
		"money":        {"10.00"},
		"name":         {"Top up"},
		"empty":        {""},
		"sign":         {"ignored"},
		"sign_type":    {"MD5"},
	}
	const want = "de78896d0cda3e185bd5e793935516f3"
	if got := epaySign(values, "secret"); got != want {
		t.Fatalf("epaySign() = %q, want %q", got, want)
	}
	values.Set("sign", "something-else")
	if got := epaySign(values, "secret"); got != want {
		t.Fatalf("signature fields must be ignored, got %q", got)
	}
}

func TestParsePaymentAmount(t *testing.T) {
	tests := []struct {
		value  string
		cents  int64
		amount string
		ok     bool
	}{
		{"1", 100, "1.00", true},
		{" 10.5 ", 1050, "10.50", true},
		{"100000.00", 10_000_000, "100000.00", true},
		{"0.01", 1, "0.01", true},
		{"1.001", 0, "", false},
		{"-1", 0, "", false},
		{"1e2", 0, "", false},
		{"", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			cents, amount, ok := parsePaymentAmount(tt.value)
			if cents != tt.cents || amount != tt.amount || ok != tt.ok {
				t.Fatalf("parsePaymentAmount(%q) = (%d, %q, %v), want (%d, %q, %v)", tt.value, cents, amount, ok, tt.cents, tt.amount, tt.ok)
			}
		})
	}
}

func TestValidPaymentMethod(t *testing.T) {
	for _, code := range []string{"alipay", "wxpay", "custom-pay_1"} {
		if !validPaymentMethod(code, "Payment") {
			t.Fatalf("expected %q to be valid", code)
		}
	}
	for _, code := range []string{"", "bad code", "bad/code"} {
		if validPaymentMethod(code, "Payment") {
			t.Fatalf("expected %q to be invalid", code)
		}
	}
}

func TestUpdatePaymentSettingsRejectsOversizedFieldsBeforeDatabase(t *testing.T) {
	cases := []string{
		`{"enabled":false,"base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","public_base_url":"https://app.example.com","merchant_id":"1"}`,
		`{"enabled":false,"base_url":"https://pay.example.com","public_base_url":"https://app.example.com","merchant_id":"` + strings.Repeat("m", 129) + `"}`,
		`{"enabled":false,"base_url":"https://pay.example.com","public_base_url":"https://app.example.com","merchant_id":"1","merchant_key":"` + strings.Repeat("k", 4097) + `"}`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/payment-settings", strings.NewReader(body))
		(&Service{}).updatePaymentSettings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d for oversized payment field", rec.Code, http.StatusBadRequest)
		}
	}
}
