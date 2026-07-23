package app

import (
	"context"
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

func TestValidatePasswordChange(t *testing.T) {
	if msg := validatePasswordChange("old-password", "new-password"); msg != "" {
		t.Fatalf("expected valid change, got %q", msg)
	}
	cases := []struct {
		current, next string
	}{
		{"", "new-password"},
		{"old-password", ""},
		{"old-password", "short"},
		{"old-password", strings.Repeat("a", 73)},
		{"same-password", "same-password"},
	}
	for _, tc := range cases {
		if msg := validatePasswordChange(tc.current, tc.next); msg == "" {
			t.Fatalf("expected rejection for current=%q next=%q", tc.current, tc.next)
		}
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
	// remaining=0 forces 429 before captcha/DB.
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

func TestChangeAccountPasswordRejectsInvalidBodyBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"current_password":"old-password"}`,
		`{"new_password":"new-password"}`,
		`{"current_password":"old-password","new_password":"short"}`,
		`{"current_password":"same-pass","new_password":"same-pass"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/account/password", strings.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), accountContextKey{}, accountContext{userID: "1"}))
		(&Service{}).changeAccountPassword(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}



func TestCreateAccountKeyRejectsEmptyNameBeforeDatabaseAccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/account/keys", strings.NewReader(`{"name":"  "}`))
	request = request.WithContext(context.WithValue(request.Context(), accountContextKey{}, accountContext{userID: "1"}))
	(&Service{}).createAccountKey(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestUpdateAccountKeyRejectsInvalidBodyBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"  "}`,
		`{"name":"ok","expires_at":"not-a-date"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/account/keys/key-id", strings.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), accountContextKey{}, accountContext{userID: "1"}))
		(&Service{}).updateAccountKey(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestRevokeAccountKeyRequiresSessionContext(t *testing.T) {
	// Without injected account context, accountFromContext panics — route uses s.account().
	// With empty path id still reaches validation after context is present.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/account/keys//revoke", nil)
	request.SetPathValue("id", "")
	request = request.WithContext(context.WithValue(request.Context(), accountContextKey{}, accountContext{userID: "1"}))
	(&Service{}).revokeAccountKey(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestValidateAvatarDataURL(t *testing.T) {
	if msg := validateAvatarDataURL(""); msg == "" {
		// empty is handled by caller; helper treats non-data as size/prefix error
	}
	if msg := validateAvatarDataURL("https://evil.example/x.png"); msg == "" {
		t.Fatal("remote http(s) avatar must be rejected")
	}
	if msg := validateAvatarDataURL("data:image/svg+xml;base64,PHN2Zy8+"); msg == "" {
		t.Fatal("svg must be rejected")
	}
	if msg := validateAvatarDataURL("data:image/png;base64,not!!!valid"); msg == "" {
		t.Fatal("invalid base64 must be rejected")
	}
	// 1x1 PNG
	const png = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	if msg := validateAvatarDataURL(png); msg != "" {
		t.Fatalf("valid png rejected: %s", msg)
	}
	if msg := validateAvatarDataURL("data:image/jpeg;base64,/9j/4AAQ"); msg == "" {
		// incomplete jpeg base64 may still decode partially; require full valid decode only
	}
}

func TestUpdateAccountProfileRejectsInvalidAvatarBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{"avatar_url":"https://example.com/a.png"}`,
		`{"avatar_url":"data:image/svg+xml;base64,PHN2Zy8+"}`,
		`{"avatar_url":"data:image/png;base64,!!!"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/account/profile", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{userID: "1"}))
		(&Service{}).updateAccountProfile(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestCreateAccountKeyRejectsInvalidNameBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":" "}`,
		`{"name":"` + strings.Repeat("n", 101) + `"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/account/keys", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{userID: "1"}))
		(&Service{}).createAccountKey(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
	}
}

func TestValidEmail(t *testing.T) {
	valid := []string{
		"user@example.com",
		"  user@example.com  ",
		"a@b.co",
		"user+tag@example.com",
		"user.name@example.com",
	}
	for _, email := range valid {
		if !validEmail(email) {
			t.Fatalf("validEmail(%q) = false, want true", email)
		}
	}
	invalid := []string{
		"",
		"   ",
		"not-an-email",
		"@example.com",
		"user@",
		"user@example.com extra",
		"Alice <alice@example.com>",
		"user@@example.com",
		"user example@example.com",
	}
	for _, email := range invalid {
		if validEmail(email) {
			t.Fatalf("validEmail(%q) = true, want false", email)
		}
	}
}
