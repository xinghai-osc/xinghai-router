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
	req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil)
	req.Host = "example.com"
	wantA := "http://example.com/auth/reset?token=abc"
	if got := (&Service{}).resetLink(req.Context(), req, "abc"); got != wantA {
		t.Fatalf("http resetLink = %q, want %q", got, wantA)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil)
	req2.Host = "example.com"
	req2.Header.Set("X-Forwarded-Proto", "https")
	wantB := "https://example.com/auth/reset?token=abc"
	if got := (&Service{}).resetLink(req2.Context(), req2, "abc"); got != wantB {
		t.Fatalf("https resetLink = %q, want %q", got, wantB)
	}
	req3 := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", nil)
	req3.Host = "example.com"
	wantC := "http://example.com/auth/reset?token=a+b"
	if got := (&Service{}).resetLink(req3.Context(), req3, "a b"); got != wantC {
		t.Fatalf("escaped resetLink = %q, want %q", got, wantC)
	}
}
