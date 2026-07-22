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

func TestRevokeAccountKeyRequiresKeyID(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/account/keys//revoke", nil)
	request.SetPathValue("id", "")
	request = request.WithContext(context.WithValue(request.Context(), accountContextKey{}, accountContext{userID: "1"}))
	(&Service{}).revokeAccountKey(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
