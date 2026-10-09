package app

import (
	"context"
	"testing"
)

func TestConcurrencyCountsUseScopeLimiter(t *testing.T) {
	s := &Service{channelLimiter: NewGroupLimiter(), userLimiter: NewGroupLimiter(), groupLimiter: NewGroupLimiter()}
	ctx := context.Background()

	_, releaseChannel, busy, err := s.acquireChannelConcurrency(ctx, channel{id: 42, maxConcurrency: 5})
	if err != nil || busy {
		t.Fatalf("acquire channel: busy=%v err=%v", busy, err)
	}
	defer releaseChannel()
	if !s.userLimiter.acquire("7", 5) {
		t.Fatal("acquire user slot")
	}
	defer s.userLimiter.release("7")
	if !s.groupLimiter.acquire("group-1", 5) {
		t.Fatal("acquire group slot")
	}
	defer s.groupLimiter.release("group-1")

	for _, tc := range []struct {
		scope string
		id    string
		want  int
	}{
		{"channel", "42", 1},
		{"user", "7", 1},
		{"group", "group-1", 1},
		{"channel", "999", 0},
		{"user", "999", 0},
		{"group", "999", 0},
	} {
		counts, err := s.concurrencyCounts(ctx, tc.scope, []concurrencyStatusEntry{{ID: tc.id, Limit: 5}}, false)
		if err != nil || len(counts) != 1 || counts[0] != tc.want {
			t.Fatalf("%s %s = %v (%v), want %d", tc.scope, tc.id, counts, err, tc.want)
		}
	}

	if _, err := s.concurrencyCounts(ctx, "unknown", []concurrencyStatusEntry{{ID: "1"}}, false); err == nil {
		t.Fatal("unknown scope must not report zero usage")
	}
}

func TestConcurrencyCountsSharedRequiresLeaseBackend(t *testing.T) {
	s := &Service{cfg: Config{DeploymentMode: "cluster"}}
	if _, err := s.concurrencyCounts(context.Background(), "channel", []concurrencyStatusEntry{{ID: "1"}}, true); err == nil {
		t.Fatal("cluster mode without a lease manager must not report zero usage")
	}
}

func TestConcurrencyCountsEmptyScope(t *testing.T) {
	s := &Service{}
	counts, err := s.concurrencyCounts(context.Background(), "channel", nil, false)
	if err != nil || len(counts) != 0 {
		t.Fatalf("empty scope = %v (%v)", counts, err)
	}
}
