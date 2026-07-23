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
func TestLoginRejectsPasswordLengthBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{"email":"user@example.com","password":"short"}`,
		`{"email":"user@example.com","password":"` + strings.Repeat("a", 73) + `"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		(&Service{}).login(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rec.Body.String(), "password must be between 8 and 72 characters") {
			t.Fatalf("body %s response = %q", body, rec.Body.String())
		}
	}
}
