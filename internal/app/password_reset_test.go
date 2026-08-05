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

func TestResetLink(t *testing.T) {
	cases := []struct {
		name string
		req  *http.Request
		want string
	}{
		{name: "plain http", req: httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil), want: "http://example.com/auth/reset?token=abc"},
		{name: "https via forward header", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil)
			req.Header.Set("X-Forwarded-Proto", "https")
			return req
		}(), want: "https://example.com/auth/reset?token=abc"},
		{name: "host and token escaped", req: httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil), want: "http://example.com/auth/reset?token=a+b"},
	}
	cases[0].req.Host = "example.com"
	cases[1].req.Host = "example.com"
	cases[2].req.Host = "example.com"
	if got := resetLink(cases[0].req, "abc"); got != cases[0].want {
		t.Fatalf("http resetLink = %q, want %q", got, cases[0].want)
	}
	if got := resetLink(cases[1].req, "abc"); got != cases[1].want {
		t.Fatalf("https resetLink = %q, want %q", got, cases[1].want)
	}
	if got := resetLink(cases[2].req, "a b"); got != cases[2].want {
		t.Fatalf("escaped resetLink = %q, want %q", got, cases[2].want)
	}
}
