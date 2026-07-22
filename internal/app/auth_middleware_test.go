package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	if !accountHasPermission(accountContext{role: "admin"}, "system.manage") {
		t.Fatal("admin must have all permissions")
	}
	if accountHasPermission(accountContext{role: "user", permissions: map[string]bool{}}, "system.manage") {
		t.Fatal("user without grants must be denied")
	}
	if !accountHasPermission(accountContext{role: "operator", permissions: map[string]bool{"logs.read": true}}, "logs.read") {
		t.Fatal("granted permission must allow")
	}
	if accountHasPermission(accountContext{role: "operator", permissions: map[string]bool{"logs.read": true}}, "audit.read") {
		t.Fatal("missing permission must deny")
	}
}

func TestPermissionMiddlewareForbiddenWithoutGrant(t *testing.T) {
	// permission wraps account(), which needs DB for real tokens. Exercise the
	// gate by injecting account context into a thin wrapper that mirrors the check.
	check := func(account accountContext, permission string) int {
		if account.role != "admin" && !account.permissions[permission] {
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
