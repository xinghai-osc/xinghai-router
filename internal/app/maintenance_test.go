package app

import (
	"context"
	"testing"
	"time"
)

func TestMaintenancePathAllowed(t *testing.T) {
	cases := []struct {
		path    string
		allowed bool
	}{
		{"/healthz", true},
		{"/site-settings", true},
		{"/account/me", true},
		{"/account/legal-consent", true},
		{"/account/data-transfer-consent", true},
		{"/account/workspaces", true},
		{"/account/workspaces/ws-1/members", true},
		{"/admin/site-settings", true},
		{"/admin/users", true},
		{"/account/keys", false},
		{"/v1/models", false},
		{"/notifications", false},
	}
	for _, tc := range cases {
		if got := maintenancePathAllowed(tc.path); got != tc.allowed {
			t.Fatalf("maintenancePathAllowed(%q) = %v, want %v", tc.path, got, tc.allowed)
		}
	}
}

func TestAuthCleanupIntervalIsPositive(t *testing.T) {
	if authCleanupInterval < time.Minute {
		t.Fatalf("authCleanupInterval = %v, want at least 1m", authCleanupInterval)
	}
	if pendingOrderMaxAge < time.Hour {
		t.Fatalf("pendingOrderMaxAge = %v, want at least 1h", pendingOrderMaxAge)
	}
	if pendingOrderAgeSQL == "" {
		t.Fatal("pendingOrderAgeSQL must not be empty")
	}
}

func TestCleanupExpiredAuthStateNoopWithoutDB(t *testing.T) {
	// Must not panic when service has no database (unit-test construction).
	(&Service{}).cleanupExpiredAuthState(context.Background())
	(&Service{}).expireStalePendingOrders(context.Background())
}
