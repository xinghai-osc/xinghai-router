package app

import (
	"net/http"
	"net/http/httptest"
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
