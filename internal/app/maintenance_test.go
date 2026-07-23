package app

import (
	"context"
	"testing"
	"time"
)

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
