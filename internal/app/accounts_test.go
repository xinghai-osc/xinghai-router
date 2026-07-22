package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOptionalAccountAllowsAnonymousRequest(t *testing.T) {
	s := &Service{}
	called := false
	handler := s.optionalAccount(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if account := accountFromContext(r); account.userID != "" {
			t.Fatalf("anonymous request has user ID %q", account.userID)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/model-catalog", nil))

	if !called {
		t.Fatal("model catalog handler was not called")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

type sequenceLimiter struct {
	remaining int
}

func (l *sequenceLimiter) allow(key string) bool {
	if l.remaining <= 0 {
		return false
	}
	l.remaining--
	return true
}

func (l *sequenceLimiter) close() {}

func TestLoginRateLimitBeforeDatabase(t *testing.T) {
	s := &Service{limiter: &sequenceLimiter{remaining: 0}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.com","password":"password1"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	s.login(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
	(&Service{}).login(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty login status = %d", rec.Code)
	}
}

func TestRegisterRateLimitBeforeDatabase(t *testing.T) {
	s := &Service{limiter: &sequenceLimiter{remaining: 0}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"user@example.com","name":"User","password":"password1"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	s.register(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestSendEmailCodeRateLimitBeforeDatabase(t *testing.T) {
	s := &Service{limiter: &sequenceLimiter{remaining: 0}}
	// Enable email verification via env-backed config fields so the handler does not 404.
	s.cfg.SMTPHost = "smtp.example.com"
	s.cfg.SMTPFrom = "noreply@example.com"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/email-code", strings.NewReader(`{"email":"user@example.com"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	s.sendEmailCode(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestDummyPasswordHashSpendsTime(t *testing.T) {
	if dummyPasswordHash == "" {
		t.Fatal("dummy password hash must be initialized")
	}
	if passwordMatches(dummyPasswordHash, "password1") {
		t.Fatal("dummy hash must not match arbitrary passwords")
	}
}
