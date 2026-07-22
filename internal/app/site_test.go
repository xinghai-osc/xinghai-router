package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidIconURL(t *testing.T) {
	for _, value := range []string{"https://cdn.example.com/icon.png", "http://127.0.0.1:3000/i.png", "http://localhost/icon.png"} {
		if !validIconURL(value) {
			t.Fatalf("expected valid icon url %q", value)
		}
	}
	for _, value := range []string{"", "ftp://x", "http://evil.example.com/x", "not-a-url"} {
		if validIconURL(value) {
			t.Fatalf("expected invalid icon url %q", value)
		}
	}
}

func TestUpdateSiteSettingsRejectsInvalidBeforeDatabase(t *testing.T) {
	cases := []string{
		`{}`,
		`{"name":""}`,
		`{"name":"` + strings.Repeat("n", 101) + `"}`,
		`{"name":"Site","icon_url":"http://evil.example.com/x"}`,
		`{"name":"Site","smtp_port":"0"}`,
		`{"name":"Site","smtp_from":"not-an-email"}`,
		`not-json`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/site-settings", strings.NewReader(body))
		(&Service{}).updateSiteSettings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestUpdatePaymentSettingsRejectsInvalidEnabledConfigBeforeDatabase(t *testing.T) {
	cases := []string{
		`{"enabled":true,"base_url":"http://evil.example.com","public_base_url":"https://app.example.com","merchant_id":"1","merchant_key":"k"}`,
		`{"enabled":true,"base_url":"https://pay.example.com","public_base_url":"http://evil.example.com","merchant_id":"1","merchant_key":"k"}`,
		`{"enabled":true,"base_url":"https://pay.example.com","public_base_url":"https://app.example.com","merchant_id":"","merchant_key":"k"}`,
		`not-json`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/payment-settings", strings.NewReader(body))
		(&Service{}).updatePaymentSettings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}
