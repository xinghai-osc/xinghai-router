package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSystemConfigCaptchaProvider(t *testing.T) {
	geetest := systemConfig{GeetestCaptchaID: "gid", GeetestCaptchaKey: "gkey"}
	corptcha := systemConfig{CorptchaSiteID: "cpt_abc", CorptchaSecret: "secret"}

	if got := (systemConfig{}).captchaProvider(); got != "" {
		t.Fatalf("empty config provider = %q, want empty", got)
	}
	if got := geetest.captchaProvider(); got != "geetest" {
		t.Fatalf("geetest-only auto provider = %q, want geetest", got)
	}
	if got := corptcha.captchaProvider(); got != "corptcha" {
		t.Fatalf("corptcha-only auto provider = %q, want corptcha", got)
	}
	if got := (systemConfig{GeetestCaptchaID: "k", GeetestCaptchaKey: "gkey", CorptchaSiteID: "cpt_abc", CorptchaSecret: "secret"}).captchaProvider(); got != "geetest" {
		t.Fatalf("both-configured auto provider = %q, want geetest", got)
	}

	both := systemConfig{CaptchaProvider: "corptcha", GeetestCaptchaID: "k", GeetestCaptchaKey: "gkey", CorptchaSiteID: "cpt_abc", CorptchaSecret: "secret"}
	if got := both.captchaProvider(); got != "corptcha" {
		t.Fatalf("explicit corptcha provider = %q, want corptcha", got)
	}
	// Missing credentials degrade to the other configured provider.
	explicit := systemConfig{CaptchaProvider: "corptcha"}
	if got := explicit.captchaProvider(); got != "" {
		t.Fatalf("explicit provider without credentials = %q, want empty", got)
	}
	degraded := systemConfig{CaptchaProvider: "geetest", CorptchaSiteID: "cpt_abc", CorptchaSecret: "secret"}
	if got := degraded.captchaProvider(); got != "corptcha" {
		t.Fatalf("degraded provider = %q, want corptcha", got)
	}
}

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
		`{"name":"Site","icon_url":"https://cdn.example.com/` + strings.Repeat("a", 2040) + `"}`,
		`{"name":"Site","smtp_port":"0"}`,
		`{"name":"Site","smtp_from":"not-an-email"}`,
		`{"name":"Site","smtp_host":"` + strings.Repeat("h", 256) + `"}`,
		`{"name":"Site","geetest_captcha_id":"` + strings.Repeat("g", 257) + `"}`,
		`{"name":"Site","geetest_captcha_key":"` + strings.Repeat("k", 257) + `"}`,
		`{"name":"Site","captcha_provider":"weird"}`,
		`{"name":"Site","corptcha_site_id":"` + strings.Repeat("c", 257) + `"}`,
		`{"name":"Site","corptcha_secret":"` + strings.Repeat("s", 257) + `"}`,
		`{"name":"Site","smtp_password":"` + strings.Repeat("p", 4097) + `"}`,
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
		`{"enabled":false,"base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","public_base_url":"https://app.example.com","merchant_id":"1"}`,
		`{"enabled":false,"base_url":"https://pay.example.com","public_base_url":"https://app.example.com","merchant_id":"` + strings.Repeat("m", 129) + `"}`,
		`{"enabled":false,"base_url":"https://pay.example.com","public_base_url":"https://app.example.com","merchant_id":"1","merchant_key":"` + strings.Repeat("k", 4097) + `"}`,
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
