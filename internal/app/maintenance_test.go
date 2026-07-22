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
}

func TestCleanupExpiredAuthStateNoopWithoutDB(t *testing.T) {
	// Must not panic when service has no database (unit-test construction).
	(&Service{}).cleanupExpiredAuthState(context.Background())
}
