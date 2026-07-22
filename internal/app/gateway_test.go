package app

import "testing"

func TestUsageCostAndClamp(t *testing.T) {
	if got := usageCost(1_000_000, 0, 1, 2, 1, 1); got != 1 {
		t.Fatalf("usageCost = %v, want 1", got)
	}
	if got := usageCost(1_000_000, 1_000_000, 1, 2, 1, 1); got != 3 {
		t.Fatalf("usageCost = %v, want 3", got)
	}
	if got := usageCost(1_000_000, 0, 1, 2, 2, 1.5); got != 3 {
		t.Fatalf("usageCost with multipliers = %v, want 3", got)
	}
	if got := usageCost(100, 0, 1, 1, 0, 0); got != usageCost(100, 0, 1, 1, 1, 1) {
		t.Fatal("zero multipliers must fall back to 1")
	}
	if got := clampCostToHold(5, 3); got != 3 {
		t.Fatalf("clamp = %v, want 3", got)
	}
	if got := clampCostToHold(-1, 3); got != 0 {
		t.Fatalf("negative clamp = %v, want 0", got)
	}
	if got := clampCostToHold(2, 0); got != 2 {
		t.Fatalf("zero hold must not clamp positive cost, got %v", got)
	}
}
