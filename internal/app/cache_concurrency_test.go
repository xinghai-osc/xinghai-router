package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type cacheTestResult struct {
	value int
	err   error
}

func TestTTLCacheCoalescesLoads(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cache := newTTLCache[string, int](time.Hour)
				release := make(chan struct{})
				results := make(chan cacheTestResult, 32)
				var calls atomic.Int32
				var loadErr error
				if fail {
					loadErr = errors.New("load failed")
				}
				load := func(context.Context) (int, error) {
					calls.Add(1)
					<-release
					return 42, loadErr
				}
				for range cap(results) {
					go func() {
						value, err := cache.get(context.Background(), "key", load)
						results <- cacheTestResult{value, err}
					}()
				}
				synctest.Wait()
				if got := calls.Load(); got != 1 {
					t.Errorf("concurrent loads = %d, want 1", got)
				}
				close(release)
				for range cap(results) {
					result := <-results
					if result.value != 42 || !errors.Is(result.err, loadErr) {
						t.Fatalf("result = %+v, want 42, %v", result, loadErr)
					}
				}
				if _, ok := cache.lookup("key"); ok == fail {
					t.Fatalf("cache presence = %v, load failed = %v", ok, fail)
				}
				if len(cache.flights) != 0 {
					t.Fatal("finished load retained an in-flight entry")
				}
				if fail {
					value, err := cache.get(context.Background(), "key", func(context.Context) (int, error) { return 43, nil })
					if err != nil || value != 43 {
						t.Fatalf("retry = %d, %v", value, err)
					}
				}
			})
		})
	}
}

func TestTTLCacheWaiterCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := newTTLCache[string, int](time.Hour)
		release := make(chan struct{})
		leader := make(chan cacheTestResult, 1)
		go func() {
			value, err := cache.get(context.Background(), "key", func(context.Context) (int, error) {
				<-release
				return 42, nil
			})
			leader <- cacheTestResult{value, err}
		}()
		synctest.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		waiter := make(chan error, 1)
		go func() {
			_, err := cache.get(ctx, "key", func(context.Context) (int, error) { panic("duplicate load") })
			waiter <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-waiter; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter error = %v", err)
		}
		close(release)
		if result := <-leader; result.err != nil || result.value != 42 {
			t.Fatalf("leader result = %+v", result)
		}
		if value, ok := cache.lookup("key"); !ok || value != 42 {
			t.Fatal("waiter cancellation interrupted cache population")
		}
	})
}

func TestTTLCacheLeaderCancellationRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := newTTLCache[string, int](time.Hour)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		leader := make(chan error, 1)
		go func() {
			_, err := cache.get(ctx, "key", func(ctx context.Context) (int, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			})
			leader <- err
		}()
		synctest.Wait()
		waiter := make(chan cacheTestResult, 1)
		go func() {
			value, err := cache.get(context.Background(), "key", func(context.Context) (int, error) { return 42, nil })
			waiter <- cacheTestResult{value, err}
		}()
		synctest.Wait()
		cancel()
		if err := <-leader; !errors.Is(err, context.Canceled) {
			t.Fatalf("leader error = %v", err)
		}
		if result := <-waiter; result.err != nil || result.value != 42 {
			t.Fatalf("waiter did not retry canceled load: %+v", result)
		}
	})
}

func TestTTLCacheInvalidationStartsNewLoad(t *testing.T) {
	for _, clear := range []bool{false, true} {
		name := "invalidate"
		if clear {
			name = "clear"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cache := newTTLCache[string, int](time.Hour)
				releaseOld := make(chan struct{})
				old := make(chan int, 1)
				go func() {
					value, _ := cache.get(context.Background(), "key", func(context.Context) (int, error) {
						<-releaseOld
						return 1, nil
					})
					old <- value
				}()
				synctest.Wait()
				if clear {
					cache.clear()
				} else {
					cache.invalidate("key")
				}
				releaseNew := make(chan struct{})
				fresh := make(chan cacheTestResult, 1)
				var loads atomic.Int32
				go func() {
					value, err := cache.get(context.Background(), "key", func(context.Context) (int, error) {
						loads.Add(1)
						<-releaseNew
						return 2, nil
					})
					fresh <- cacheTestResult{value, err}
				}()
				synctest.Wait()
				if loads.Load() != 1 {
					t.Error("new request joined invalidated load")
				}
				close(releaseOld)
				if value := <-old; value != 1 {
					t.Fatalf("original request = %d", value)
				}
				if _, ok := cache.lookup("key"); ok {
					t.Fatal("invalidated load repopulated cache")
				}
				if len(cache.flights) != 1 {
					t.Fatal("old load removed the new in-flight entry")
				}
				close(releaseNew)
				if result := <-fresh; result.err != nil || result.value != 2 {
					t.Fatalf("new request = %+v", result)
				}
				if value, ok := cache.lookup("key"); !ok || value != 2 {
					t.Fatalf("fresh cache = %d, %v", value, ok)
				}
			})
		})
	}
}

func TestTTLCachePanicReleasesWaiters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := newTTLCache[string, int](time.Hour)
		release := make(chan struct{})
		panicked := make(chan any, 1)
		go func() {
			defer func() { panicked <- recover() }()
			_, _ = cache.get(context.Background(), "key", func(context.Context) (int, error) {
				<-release
				panic("load panic")
			})
		}()
		synctest.Wait()
		result := make(chan cacheTestResult, 1)
		go func() {
			value, err := cache.get(context.Background(), "key", func(context.Context) (int, error) { return 42, nil })
			result <- cacheTestResult{value, err}
		}()
		synctest.Wait()
		value, err := cache.get(context.Background(), "other", func(context.Context) (int, error) { return 7, nil })
		if value != 7 || err != nil {
			t.Fatalf("independent key = %d, %v", value, err)
		}
		close(release)
		if value := <-panicked; value != "load panic" {
			t.Fatalf("panic changed: %v", value)
		}
		if got := <-result; got.err != nil || got.value != 42 {
			t.Fatalf("waiter did not recover after leader panic: %+v", got)
		}
	})
}

func TestTTLCacheRefreshAtCapacity(t *testing.T) {
	for _, once := range []bool{false, true} {
		cache := newTTLCache[int, int](time.Hour)
		for key := range maxCacheEntries {
			cache.store(key, key)
		}
		key := maxCacheEntries - 1
		if once {
			cache.entries[key] = cacheEntry[int]{value: key, expires: time.Now().Add(-time.Second)}
			if !cache.storeOnce(key, 42) {
				t.Fatal("expired entry was not refreshed")
			}
		} else {
			cache.store(key, 42)
		}
		if _, ok := cache.lookup(0); !ok {
			t.Fatal("refresh evicted an unrelated entry")
		}
		if len(cache.entries) != maxCacheEntries || len(cache.order) != maxCacheEntries {
			t.Fatalf("refresh changed cache size: %d entries, %d keys", len(cache.entries), len(cache.order))
		}
		cache.store(maxCacheEntries, 100)
		if _, ok := cache.lookup(0); ok {
			t.Fatal("insertion did not evict oldest entry")
		}
	}
}
