package app

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func leaseTestRedis(t *testing.T) (string, *redisLimiter, string) {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL is not set; use an isolated Redis for lease integration tests")
	}
	client, err := newRedisLimiter(url, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.close)
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	return url, client, id
}

func leaseCommand(t *testing.T, client *redisLimiter, args ...string) any {
	t.Helper()
	reply, err := client.command(context.Background(), args...)
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func TestConcurrencyLeasesShareLimitsAndRelease(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	a := &concurrencyLeaseManager{redisURL: url, ttl: 3 * time.Second}
	b := &concurrencyLeaseManager{redisURL: url, ttl: 3 * time.Second}
	limits := []concurrencyLimit{{key: id, max: 2, scope: "user"}, {key: id, max: 1, scope: "group"}}
	_, release, scope, err := a.acquire(context.Background(), limits)
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	_, _, scope, err = b.acquire(context.Background(), limits)
	if err != nil || scope != "group" {
		t.Fatalf("second replica group check: %s %v", scope, err)
	}
	_, releaseUser, scope, err := b.acquire(context.Background(), limits[:1])
	if err != nil || scope != "" {
		t.Fatalf("failed acquisition leaked a user slot: %s %v", scope, err)
	}
	defer releaseUser()
	_, _, scope, err = a.acquire(context.Background(), limits[:1])
	if err != nil || scope != "user" {
		t.Fatalf("user check: %s %v", scope, err)
	}
	release()
	release()
	_, releaseNext, scope, err := b.acquire(context.Background(), limits)
	if err != nil || scope != "" {
		t.Fatalf("idempotent release: %s %v", scope, err)
	}
	releaseNext()
}

func TestConcurrencyLeasesAtomicAcrossReplicas(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	managers := []*concurrencyLeaseManager{{redisURL: url, ttl: 5 * time.Second}, {redisURL: url, ttl: 5 * time.Second}}
	limits := []concurrencyLimit{{key: id, max: 3, scope: "user"}}
	var wg sync.WaitGroup
	results := make(chan func(), 24)
	for i := range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, release, scope, err := managers[i%2].acquire(context.Background(), limits)
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			if scope == "" {
				results <- release
			}
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for release := range results {
		count++
		release()
	}
	if count != 3 {
		t.Fatalf("admitted %d, want 3", count)
	}
}

func TestConcurrencyLeaseRenewalAndLossCancellation(t *testing.T) {
	url, client, id := leaseTestRedis(t)
	manager := &concurrencyLeaseManager{redisURL: url, ttl: 600 * time.Millisecond}
	limits := []concurrencyLimit{{key: id, max: 1, scope: "user"}}
	ctx, release, scope, err := manager.acquire(context.Background(), limits)
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	select {
	case <-ctx.Done():
		t.Fatal("lease cancelled while Redis healthy")
	case <-time.After(2 * manager.ttl):
	}
	_, _, scope, err = manager.acquire(context.Background(), limits)
	if err != nil || scope != "user" {
		t.Fatalf("long request lost lease: %s %v", scope, err)
	}
	leaseCommand(t, client, "DEL", "xh:concurrency:{leases}:user:"+id)
	select {
	case <-ctx.Done():
	case <-time.After(manager.ttl):
		t.Fatal("lost lease did not cancel request")
	}
}

func TestConcurrencyLeaseExpiryAndStaleRelease(t *testing.T) {
	_, client, id := leaseTestRedis(t)
	key := "xh:concurrency:{leases}:user:" + id
	acquire := func(token string, ttl int) any {
		return leaseCommand(t, client, "EVAL", concurrencyAcquireScript, "1", key, token, strconv.Itoa(ttl), "1")
	}
	if reply := acquire("abandoned", 80); reply != int64(0) {
		t.Fatalf("acquire=%v", reply)
	}
	<-time.After(130 * time.Millisecond)
	if reply := acquire("replacement", 2000); reply != int64(0) {
		t.Fatalf("expired lease not reclaimed: %v", reply)
	}
	leaseCommand(t, client, "EVAL", concurrencyReleaseScript, "1", key, "abandoned")
	if reply := acquire("third", 2000); reply != int64(1) {
		t.Fatalf("stale release removed replacement: %v", reply)
	}
	if reply := leaseCommand(t, client, "EVAL", concurrencyRenewScript, "1", key, "abandoned", "2000"); reply != int64(0) {
		t.Fatalf("expired owner resurrected lease: %v", reply)
	}
	leaseCommand(t, client, "DEL", key)
}

func TestConcurrencyLeaseRenewalOutageCancels(t *testing.T) {
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		if len(args) > 1 && args[1] == concurrencyAcquireScript {
			return ":0\r\n"
		}
		return "-ERR simulated outage\r\n"
	})
	manager := &concurrencyLeaseManager{redisURL: server.url(), ttl: 300 * time.Millisecond}
	ctx, release, scope, err := manager.acquire(context.Background(), []concurrencyLimit{{key: "test", max: 1, scope: "user"}})
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	select {
	case <-ctx.Done():
	case <-time.After(manager.ttl):
		t.Fatal("renewal failure did not cancel upstream before lease expiry")
	}
}

func TestConcurrencyLeaseDifferentTTLsDoNotExpireOtherOwners(t *testing.T) {
	_, client, id := leaseTestRedis(t)
	key := "xh:concurrency:{leases}:user:" + id
	for _, entry := range []struct{ token, ttl string }{{"long", "2000"}, {"short", "50"}} {
		if reply := leaseCommand(t, client, "EVAL", concurrencyAcquireScript, "1", key, entry.token, entry.ttl, "2"); reply != int64(0) {
			t.Fatalf("acquire: %v", reply)
		}
	}
	<-time.After(100 * time.Millisecond)
	if reply := leaseCommand(t, client, "EVAL", concurrencyAcquireScript, "1", key, "third", "1000", "1"); reply != int64(1) {
		t.Fatalf("short lease expired long owner: %v", reply)
	}
	leaseCommand(t, client, "DEL", key)
}

func TestConcurrencyLeaseRedisUnavailableFailsClosed(t *testing.T) {
	manager := &concurrencyLeaseManager{redisURL: "redis://127.0.0.1:1", ttl: 5 * time.Second}
	_, release, scope, err := manager.acquire(context.Background(), []concurrencyLimit{{key: "test", max: 1, scope: "user"}})
	if err == nil || release != nil || scope != "" {
		t.Fatalf("outage admitted request: scope=%s err=%v", scope, err)
	}
}

func TestConcurrencyLeaseCountsTrackAcquireReleaseAndExpiry(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	manager := &concurrencyLeaseManager{redisURL: url, ttl: 300 * time.Millisecond}
	_, release, scope, err := manager.acquire(context.Background(), []concurrencyLimit{{key: id, max: 3, scope: "channel"}})
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	counts, err := manager.counts(context.Background(), "channel", []string{id, "absent"})
	if err != nil || len(counts) != 2 || counts[0] != 1 || counts[1] != 0 {
		t.Fatalf("counts = %v (%v), want [1 0]", counts, err)
	}
	other := &concurrencyLeaseManager{redisURL: url, ttl: 300 * time.Millisecond}
	counts, err = other.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 1 {
		t.Fatalf("replica read %v (%v), want [1]", counts, err)
	}
	release()
	counts, err = manager.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 0 {
		t.Fatalf("released lease counted as %v (%v)", counts, err)
	}
	if _, _, scope, err = manager.acquire(context.Background(), []concurrencyLimit{{key: id, max: 3, scope: "channel"}}); err != nil || scope != "" {
		t.Fatalf("re-acquire: %s %v", scope, err)
	}
	<-time.After(450 * time.Millisecond)
	counts, err = manager.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 0 {
		t.Fatalf("expired lease counted as %v (%v)", counts, err)
	}
}

func TestConcurrencyLeaseCountsUnavailableFailsClosed(t *testing.T) {
	manager := &concurrencyLeaseManager{redisURL: "redis://127.0.0.1:1", ttl: time.Second}
	if counts, err := manager.counts(context.Background(), "channel", []string{"x"}); err == nil {
		t.Fatalf("outage reported usage %v instead of failing", counts)
	}
	if counts, err := manager.counts(context.Background(), "channel", nil); err != nil || len(counts) != 0 {
		t.Fatalf("empty key set = %v (%v)", counts, err)
	}
}

func TestConcurrencyLeaseCancelledOwnerExpires(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	manager := &concurrencyLeaseManager{redisURL: url, ttl: 300 * time.Millisecond}
	owner, cancel := context.WithCancel(context.Background())
	limits := []concurrencyLimit{{key: id, max: 1, scope: "user"}}
	_, release, scope, err := manager.acquire(owner, limits)
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	defer release()
	cancel()
	<-time.After(450 * time.Millisecond)
	_, nextRelease, scope, err := manager.acquire(context.Background(), limits)
	if err != nil || scope != "" {
		t.Fatalf("abandoned owner not reclaimed: %s %v", scope, err)
	}
	defer nextRelease()
	release()
	_, _, scope, err = manager.acquire(context.Background(), limits)
	if err != nil || scope != "user" {
		t.Fatalf("old owner released replacement: %s %v", scope, err)
	}
}
