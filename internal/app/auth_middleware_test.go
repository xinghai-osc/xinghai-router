package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordChangeAllowedPath(t *testing.T) {
	cases := []struct {
		path    string
		allowed bool
	}{
		{"/account/me", true},
		{"/account/password", true},
		{"/auth/logout", true},
		{"/account/keys", false},
		{"/admin/users", false},
		{"/account/usage", false},
		{"/account/preferences", false},
		{"/account/me/", false},
		{"/account/password/change", false},
		{"/auth/login", false},
		{"", false},
		{"/", false},
		{"/v1/models", false},
	}
	for _, tc := range cases {
		if got := passwordChangeAllowedPath(tc.path); got != tc.allowed {
			t.Fatalf("passwordChangeAllowedPath(%q) = %v, want %v", tc.path, got, tc.allowed)
		}
	}
}

func TestAccountMiddlewareRequiresSessionToken(t *testing.T) {
	called := false
	handler := (&Service{}).account(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/account/me", nil))
	if called {
		t.Fatal("handler must not run without session")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "unauthorized" {
		t.Fatalf("code = %#v", errObj["code"])
	}
}

func TestAPIMiddlewareRequiresAPIKey(t *testing.T) {
	called := false
	handler := (&Service{limiter: newMemoryLimiter(10)}).api(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if called {
		t.Fatal("handler must not run without API key")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), "invalid_api_key") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPermissionGateAdminBypass(t *testing.T) {
	cases := []struct {
		name       string
		account    accountContext
		permission string
		want       bool
	}{
		{"admin any permission", accountContext{role: "admin"}, "system.manage", true},
		{"admin empty permission", accountContext{role: "admin"}, "", true},
		{"admin nil permissions map", accountContext{role: "admin", permissions: nil}, "users.read", true},
		{"user empty grants", accountContext{role: "user", permissions: map[string]bool{}}, "system.manage", false},
		{"user nil permissions", accountContext{role: "user"}, "users.read", false},
		{"operator granted", accountContext{role: "operator", permissions: map[string]bool{"logs.read": true}}, "logs.read", true},
		{"operator missing", accountContext{role: "operator", permissions: map[string]bool{"logs.read": true}}, "audit.read", false},
		{"operator false grant", accountContext{role: "operator", permissions: map[string]bool{"logs.read": false}}, "logs.read", false},
		{"user granted keys", accountContext{role: "user", permissions: map[string]bool{"keys.manage": true}}, "keys.manage", true},
		{"user wrong grant", accountContext{role: "user", permissions: map[string]bool{"keys.manage": true}}, "channels.manage", false},
	}
	for _, tc := range cases {
		if got := accountHasPermission(tc.account, tc.permission); got != tc.want {
			t.Fatalf("%s: accountHasPermission = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPermissionMiddlewareForbiddenWithoutGrant(t *testing.T) {
	// permission wraps account(), which needs DB for real tokens. Exercise the
	// gate via the real accountHasPermission symbol.
	check := func(account accountContext, permission string) int {
		if !accountHasPermission(account, permission) {
			return http.StatusForbidden
		}
		return http.StatusNoContent
	}
	if code := check(accountContext{role: "user", permissions: map[string]bool{}}, "users.read"); code != http.StatusForbidden {
		t.Fatalf("code = %d", code)
	}
	if code := check(accountContext{role: "admin"}, "users.read"); code != http.StatusNoContent {
		t.Fatalf("code = %d", code)
	}
	if code := check(accountContext{role: "operator", permissions: map[string]bool{"users.read": true}}, "users.read"); code != http.StatusNoContent {
		t.Fatalf("code = %d", code)
	}
	if code := check(accountContext{role: "operator", permissions: map[string]bool{"logs.read": true}}, "users.read"); code != http.StatusForbidden {
		t.Fatalf("code = %d", code)
	}
}

func TestAvailablePermissionsCatalog(t *testing.T) {
	required := []string{
		"users.read", "keys.manage", "channels.read", "channels.manage",
		"logs.read", "audit.read", "pricing.read", "pricing.manage",
		"wallets.manage", "routes.manage", "quotas.manage", "system.manage",
	}
	for _, permission := range required {
		if !availablePermissions[permission] {
			t.Fatalf("missing permission catalog entry %q", permission)
		}
	}
	if availablePermissions["not.a.real.permission"] {
		t.Fatal("unknown permission must be absent")
	}
}

func TestSetUserPermissionsRejectsInvalidBeforeDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/users/1/permissions", strings.NewReader(`{"permissions":["unknown.perm"]}`))
	req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{role: "admin", userID: "admin"}))
	(&Service{}).setUserPermissions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWriteErrorShape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusForbidden, "forbidden", "missing permission: logs.read")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "forbidden" || errObj["type"] != "forbidden" {
		t.Fatalf("error object = %#v", errObj)
	}
	if errObj["message"] != "missing permission: logs.read" {
		t.Fatalf("message = %#v", errObj["message"])
	}
}

func TestParseExpiry(t *testing.T) {
	if v, err := parseExpiry(""); err != nil || v != nil {
		t.Fatalf("empty expiry = %v %v", v, err)
	}
	if _, err := parseExpiry("not-a-date"); err == nil {
		t.Fatal("expected parse error")
	}
	if v, err := parseExpiry("2026-01-02T03:04:05Z"); err != nil || v == nil {
		t.Fatalf("valid expiry failed: %v %v", v, err)
	}
}
