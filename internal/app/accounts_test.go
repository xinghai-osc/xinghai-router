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

func TestValidateAvatarDataURL(t *testing.T) {
	if msg := validateAvatarDataURL("https://evil.example/x.png"); msg == "" {
		t.Fatal("remote http(s) avatar must be rejected")
	}
	if msg := validateAvatarDataURL("data:image/svg+xml;base64,PHN2Zy8+"); msg == "" {
		t.Fatal("svg must be rejected")
	}
	if msg := validateAvatarDataURL("data:image/png;base64,not!!!valid"); msg == "" {
		t.Fatal("invalid base64 must be rejected")
	}
	const png = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	if msg := validateAvatarDataURL(png); msg != "" {
		t.Fatalf("valid png rejected: %s", msg)
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
