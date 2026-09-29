package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkspaceHeaderValidationBeforeDatabase(t *testing.T) {
	for _, headers := range [][]string{{"not-a-uuid"}, {"../../admin"}, {"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"}} {
		r := httptest.NewRequest(http.MethodGet, "/account/keys", nil)
		r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{userID: "1"}))
		for _, value := range headers {
			r.Header.Add("X-Workspace-ID", value)
		}
		w := httptest.NewRecorder()
		(&Service{}).withWorkspace(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid workspace reached handler") })(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("headers=%v: status=%d body=%s", headers, w.Code, w.Body.String())
		}
	}
}

func TestWorkspaceRoutesRequireAccountSession(t *testing.T) {
	s := &Service{}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/account/workspaces"},
		{http.MethodPost, "/account/workspaces"},
		{http.MethodPut, "/account/workspaces/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/account/workspaces/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/account/workspaces/00000000-0000-0000-0000-000000000001/members"},
		{http.MethodGet, "/account/keys"},
		{http.MethodGet, "/account/usage"},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(route.method, route.path, nil)
		r.Header.Set("X-Xinghai-Request", "1")
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status=%d body=%s", route.method, route.path, w.Code, w.Body.String())
		}
	}
}
