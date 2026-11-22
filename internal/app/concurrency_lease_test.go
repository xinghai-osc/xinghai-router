package app

import (
	"context"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
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

func testLeaseManager(t testing.TB, url string, ttl time.Duration) *concurrencyLeaseManager {
	t.Helper()
	manager, err := newConcurrencyLeaseManager(url, ttl)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.client.close)
	return manager
}

func BenchmarkClusterLeaseAdmission(b *testing.B) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		b.Skip("TEST_REDIS_URL is not set")
	}
	manager := testLeaseManager(b, url, 30*time.Second)
	id, err := randomID()
	if err != nil {
		b.Fatal(err)
	}
	limits := []concurrencyLimit{{key: id, scope: "user", max: 1024}}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, release, scope, err := manager.acquire(b.Context(), limits)
			if err != nil || scope != "" {
				b.Errorf("acquire: %q %v", scope, err)
				return
			}
			release()
		}
	})
}

func TestClusterLeaseConnectionReuse(t *testing.T) {
	var selects atomic.Int32
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		if args[0] == "SELECT" {
			selects.Add(1)
			return "+OK\r\n"
		}
		switch args[1] {
		case concurrencyAcquireScript:
			return ":0\r\n"
		case concurrencyReleaseScript:
			return ":1\r\n"
		case concurrencyCountScript:
			return "*1\r\n:0\r\n"
		}
		return "-ERR unexpected command\r\n"
	})
	manager := testLeaseManager(t, server.url()+"/1", time.Minute)
	limits := []concurrencyLimit{{key: "test", scope: "user", max: 1}}
	for range 3 {
		_, release, scope, err := manager.acquire(t.Context(), limits)
		if err != nil || scope != "" {
			t.Fatalf("acquire: %s %v", scope, err)
		}
		release()
		if _, err := manager.counts(t.Context(), "user", []string{"test"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := selects.Load(); n != 1 {
		t.Fatalf("opened %d connections for sequential commands, want 1", n)
	}
	manager.client.close()
	if _, _, _, err := manager.acquire(t.Context(), limits); err != errConcurrencyUnavailable {
		t.Fatalf("acquire after shutdown: %v", err)
	}
}

func TestConcurrencyLeasesShareLimitsAndRelease(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	a := testLeaseManager(t, url, 3*time.Second)
	b := testLeaseManager(t, url, 3*time.Second)
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
	managers := []*concurrencyLeaseManager{testLeaseManager(t, url, 5*time.Second), testLeaseManager(t, url, 5*time.Second)}
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
	manager := testLeaseManager(t, url, 600*time.Millisecond)
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
	manager := testLeaseManager(t, server.url(), 300*time.Millisecond)
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
	manager := testLeaseManager(t, "redis://127.0.0.1:1", 5*time.Second)
	_, release, scope, err := manager.acquire(context.Background(), []concurrencyLimit{{key: "test", max: 1, scope: "user"}})
	if err == nil || release != nil || scope != "" {
		t.Fatalf("outage admitted request: scope=%s err=%v", scope, err)
	}
}

func TestConcurrencyLeaseCountsTrackAcquireReleaseAndExpiry(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	manager := testLeaseManager(t, url, 300*time.Millisecond)
	_, release, scope, err := manager.acquire(context.Background(), []concurrencyLimit{{key: id, max: 3, scope: "channel"}})
	if err != nil || scope != "" {
		t.Fatalf("acquire: %s %v", scope, err)
	}
	counts, err := manager.counts(context.Background(), "channel", []string{id, "absent"})
	if err != nil || len(counts) != 2 || counts[0] != 1 || counts[1] != 0 {
		t.Fatalf("counts = %v (%v), want [1 0]", counts, err)
	}
	other := testLeaseManager(t, url, 300*time.Millisecond)
	counts, err = other.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 1 {
		t.Fatalf("replica read %v (%v), want [1]", counts, err)
	}
	release()
	counts, err = manager.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 0 {
		t.Fatalf("released lease counted as %v (%v)", counts, err)
	}
	owner, cancel := context.WithCancel(context.Background())
	_, release, scope, err = manager.acquire(owner, []concurrencyLimit{{key: id, max: 3, scope: "channel"}})
	cancel()
	if err != nil || scope != "" {
		t.Fatalf("re-acquire: %s %v", scope, err)
	}
	defer release()
	<-time.After(450 * time.Millisecond)
	counts, err = manager.counts(context.Background(), "channel", []string{id})
	if err != nil || counts[0] != 0 {
		t.Fatalf("expired lease counted as %v (%v)", counts, err)
	}
}

func TestConcurrencyLeaseCountsUnavailableFailsClosed(t *testing.T) {
	manager := testLeaseManager(t, "redis://127.0.0.1:1", time.Second)
	if counts, err := manager.counts(context.Background(), "channel", []string{"x"}); err == nil {
		t.Fatalf("outage reported usage %v instead of failing", counts)
	}
	if counts, err := manager.counts(context.Background(), "channel", nil); err != nil || len(counts) != 0 {
		t.Fatalf("empty key set = %v (%v)", counts, err)
	}
}

func TestConcurrencyLeaseCancelledOwnerExpires(t *testing.T) {
	url, _, id := leaseTestRedis(t)
	manager := testLeaseManager(t, url, 300*time.Millisecond)
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
