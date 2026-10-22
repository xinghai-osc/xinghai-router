package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestPasswordResetDisabledWithoutSMTP(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", strings.NewReader(`{"email":"user@example.com"}`))
	(&Service{}).requestPasswordReset(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequestPasswordResetRejectsInvalidEmailBeforeDatabase(t *testing.T) {
	s := &Service{}
	s.cfg.SMTPHost = "smtp.example.com"
	s.cfg.SMTPFrom = "noreply@example.com"
	for _, body := range []string{
		`{}`,
		`{"email":"not-an-email"}`,
		`{"email":"  "}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.10:12345"
		s.requestPasswordReset(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestRequestPasswordResetRateLimitBeforeDatabase(t *testing.T) {
	s := &Service{limiter: &sequenceLimiter{remaining: 0}}
	s.cfg.SMTPHost = "smtp.example.com"
	s.cfg.SMTPFrom = "noreply@example.com"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", strings.NewReader(`{"email":"user@example.com"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	s.requestPasswordReset(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestConfirmPasswordResetRejectsInvalidBodyBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"password":"new-password"}`,
		`{"token":"  "}`,
		`{"token":"token","password":"short"}`,
		`{"token":"token","password":"` + strings.Repeat("a", 73) + `"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/confirm", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.10:12345"
		(&Service{}).confirmPasswordReset(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestResetLinkRequiresTrustedPublicOrigin(t *testing.T) {
	if _, err := (&Service{}).trustedPublicBaseURL(t.Context()); err == nil {
		t.Fatal("missing configured public origin was accepted")
	}
}

func TestNormalizeTrustedPublicOrigin(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"https://router.example.test", "https://router.example.test"},
		{" https://router.example.test/ ", "https://router.example.test"},
		{"https://router.example.test:8443/", "https://router.example.test:8443"},
		{"http://localhost:5173/", "http://localhost:5173"},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"http://[::1]:8080", "http://[::1]:8080"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := normalizeTrustedPublicOrigin(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("origin=%q want=%q err=%v", got, tc.want, err)
			}
		})
	}
	for _, input := range []string{
		"", "//router.example.test", "http://router.example.test", "javascript:alert(1)",
		"https://user:password@router.example.test", "https://router.example.test/path",
		"https://router.example.test//", "https://router.example.test?next=evil", "https://router.example.test?",
		"https://router.example.test#fragment", "https://router.example.test#", "https://router.example.test/%2f",
		"https://router.example.test:0", "https://router.example.test:65536", "https://router.example.test:invalid",
		"https://:443", "https:///missing-host", "https://router.example.test\\@evil.test",
	} {
		t.Run(input, func(t *testing.T) {
			if got, err := normalizeTrustedPublicOrigin(input); err == nil {
				t.Fatalf("accepted unsafe origin %q as %q", input, got)
			}
		})
	}
}

func TestUpdateSiteSettingsRejectsUnsafePublicOrigin(t *testing.T) {
	for _, origin := range []string{"http://evil.example", "https://router.example/path", "https://router.example?", "https://user:pass@router.example"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/admin/site-settings", strings.NewReader(`{"public_base_url":"`+origin+`"}`))
		(&Service{}).updateSiteSettings(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("origin=%q status=%d body=%s", origin, w.Code, w.Body.String())
		}
	}
}
