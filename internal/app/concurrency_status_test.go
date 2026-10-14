package app

import (
	"context"
	"testing"
	"time"
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
		counts, err := s.concurrencyCounts(ctx, tc.scope, []string{tc.id}, false)
		if err != nil || len(counts) != 1 || counts[0] != tc.want {
			t.Fatalf("%s %s = %v (%v), want %d", tc.scope, tc.id, counts, err, tc.want)
		}
	}

	if _, err := s.concurrencyCounts(ctx, "unknown", []string{"1"}, false); err == nil {
		t.Fatal("unknown scope must not report zero usage")
	}
}

func TestConcurrencyCountsSharedRequiresLeaseBackend(t *testing.T) {
	s := &Service{cfg: Config{DeploymentMode: "cluster"}}
	if _, err := s.concurrencyCounts(context.Background(), "channel", []string{"1"}, true); err == nil {
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

func TestUnlimitedChannelConcurrencyIsCounted(t *testing.T) {
	s := &Service{channelLimiter: NewGroupLimiter()}
	ctx := context.Background()
	_, release, busy, err := s.acquireChannelConcurrency(ctx, channel{id: 5})
	if err != nil || busy {
		t.Fatalf("unlimited channel: busy=%v err=%v", busy, err)
	}
	if active := s.channelLimiter.active(); active["5"] != 1 {
		t.Fatalf("unlimited channel not counted: %v", active)
	}
	release()
	release()
	if active := s.channelLimiter.active(); len(active) != 0 {
		t.Fatalf("released channel still counted: %v", active)
	}
}

func TestUnlimitedRequestConcurrencyIsCounted(t *testing.T) {
	key := keyContext{userID: "11", groupID: "22222222-2222-2222-2222-222222222222"}
	s := &Service{
		userLimiter:           NewGroupLimiter(),
		groupLimiter:          NewGroupLimiter(),
		userConcurrencyCache:  newTTLCache[string, int](time.Minute),
		groupConcurrencyCache: newTTLCache[string, int](time.Minute),
	}
	// Cached zeros keep the limit lookup off the database: both scopes are
	// unlimited, and the request must still be counted.
	s.userConcurrencyCache.store(key.userID, 0)
	s.groupConcurrencyCache.store(key.groupID, 0)

	_, release, blocked, err := s.acquireRequestConcurrency(context.Background(), key)
	if err != nil || blocked != "" {
		t.Fatalf("unlimited request: blocked=%q err=%v", blocked, err)
	}
	if active := s.userLimiter.active(); active[key.userID] != 1 {
		t.Fatalf("unlimited user not counted: %v", active)
	}
	if active := s.groupLimiter.active(); active[key.groupID] != 1 {
		t.Fatalf("unlimited group not counted: %v", active)
	}
	release()
	release()
	if active := s.userLimiter.active(); len(active) != 0 {
		t.Fatalf("released user still counted: %v", active)
	}
	if active := s.groupLimiter.active(); len(active) != 0 {
		t.Fatalf("released group still counted: %v", active)
	}
}
