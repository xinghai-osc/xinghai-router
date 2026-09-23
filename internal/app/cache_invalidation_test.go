package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTTLCacheGetInvalidationDuringLoad(t *testing.T) {
	for _, operation := range []string{"clear", "invalidate"} {
		for _, fresh := range []bool{false, true} {
			name := operation + "/empty"
			if fresh {
				name = operation + "/fresh"
			}
			t.Run(name, func(t *testing.T) {
				cache := newTTLCache[string, string](time.Hour)
				started := make(chan struct{})
				release := make(chan struct{})
				done := make(chan string, 1)
				go func() {
					value, _ := cache.get(context.Background(), "key", func(context.Context) (string, error) {
						close(started)
						<-release
						return "old", nil
					})
					done <- value
				}()
				<-started
				if operation == "clear" {
					cache.clear()
				} else {
					cache.invalidate("key")
				}
				if fresh {
					cache.store("key", "new")
				}
				close(release)
				if value := <-done; value != "old" {
					t.Fatalf("in-flight response = %q", value)
				}
				value, ok := cache.lookup("key")
				if ok != fresh || (fresh && value != "new") {
					t.Fatalf("stale load repopulated cache: value=%q present=%v", value, ok)
				}
			})
		}
	}
}

func TestTTLCacheGenerationStores(t *testing.T) {
	cache := newTTLCache[string, int](time.Hour)
	_, _, generation := cache.lookupGeneration("key")
	cache.storeIfGeneration("key", 1, generation)
	if value, ok := cache.lookup("key"); !ok || value != 1 {
		t.Fatalf("same-generation store = %d, %v", value, ok)
	}
	cache.invalidate("key")
	cache.storeIfGeneration("key", 2, generation)
	if _, ok := cache.lookup("key"); ok {
		t.Fatal("invalidated generation was stored")
	}
	_, _, generation = cache.lookupGeneration("key")
	cache.storeIfGeneration("key", 3, generation)
	cache.clear()
	cache.storeIfGeneration("key", 4, generation)
	if _, ok := cache.lookup("key"); ok {
		t.Fatal("cleared generation was stored")
	}
}

func TestTTLCacheGetHitsFailuresAndNil(t *testing.T) {
	cache := newTTLCache[string, int](time.Hour)
	calls := 0
	load := func(context.Context) (int, error) {
		calls++
		return calls, nil
	}
	for range 2 {
		value, err := cache.get(context.Background(), "key", load)
		if err != nil || value != 1 {
			t.Fatalf("get = %d, %v", value, err)
		}
	}
	if calls != 1 {
		t.Fatalf("loads = %d", calls)
	}
	loadErr := errors.New("load failed")
	_, err := cache.get(context.Background(), "failure", func(context.Context) (int, error) {
		return 0, loadErr
	})
	if !errors.Is(err, loadErr) {
		t.Fatalf("load error = %v", err)
	}
	if _, ok := cache.lookup("failure"); ok {
		t.Fatal("failed load was cached")
	}
	var missing *ttlCache[string, int]
	missing.clear()
	missing.invalidate("key")
	missing.storeIfGeneration("key", 0, 0)
	value, err := missing.get(context.Background(), "key", load)
	if err != nil || value != 2 {
		t.Fatalf("nil cache get = %d, %v", value, err)
	}
}

func configInvalidationTestService() *Service {
	return &Service{
		pricingCache:          newTTLCache[string, pricingRule](time.Hour),
		groupCache:            newTTLCache[string, float64](time.Hour),
		groupConcurrencyCache: newTTLCache[string, int](time.Hour),
		userConcurrencyCache:  newTTLCache[string, int](time.Hour),
		reliabilityData:       newTTLCache[struct{}, reliabilitySettings](time.Hour),
		contentPolicyData:     newTTLCache[struct{}, contentPolicySnapshot](time.Hour),
		conversationCacheData: newTTLCache[struct{}, conversationCacheSettings](time.Hour),
		channelKeyCache:       newTTLCache[int64, []channelKeyCredential](time.Hour),
		channelCache:          newTTLCache[channelRouteKey, []channel](time.Hour),
		subscriptionCache:     newTTLCache[subscriptionRouteKey, subscriptionAccess](time.Hour),
		channelQuotaCache:     newTTLCache[int64, bool](time.Hour),
		quotaAbsentCache:      newTTLCache[quotaRouteKey, struct{}](time.Hour),
		rankingsCache:         newTTLCache[string, rankingsPayload](time.Hour),
		performanceCache:      newTTLCache[string, modelPerformancePayload](time.Hour),
		keyTouchCache:         newTTLCache[string, struct{}](time.Hour),
	}
}

func seedConfigInvalidationTestCache[K comparable, V any](cache *ttlCache[K, V]) func() bool {
	var key K
	var value V
	cache.store(key, value)
	return func() bool {
		_, ok := cache.lookup(key)
		return ok
	}
}

func TestInvalidateConfigCaches(t *testing.T) {
	var missing *Service
	missing.invalidateConfigCaches()
	missing.startConfigInvalidation(context.Background())
	(&Service{}).invalidateConfigCaches()
	(&Service{}).startConfigInvalidation(context.Background())
	s := configInvalidationTestService()
	checks := map[string]func() bool{
		"pricing":           seedConfigInvalidationTestCache(s.pricingCache),
		"groups":            seedConfigInvalidationTestCache(s.groupCache),
		"groupConcurrency":  seedConfigInvalidationTestCache(s.groupConcurrencyCache),
		"userConcurrency":   seedConfigInvalidationTestCache(s.userConcurrencyCache),
		"reliability":       seedConfigInvalidationTestCache(s.reliabilityData),
		"contentPolicy":     seedConfigInvalidationTestCache(s.contentPolicyData),
		"conversationCache": seedConfigInvalidationTestCache(s.conversationCacheData),
		"channelKeys":       seedConfigInvalidationTestCache(s.channelKeyCache),
		"channels":          seedConfigInvalidationTestCache(s.channelCache),
		"subscriptions":     seedConfigInvalidationTestCache(s.subscriptionCache),
		"channelQuotas":     seedConfigInvalidationTestCache(s.channelQuotaCache),
		"quotaAbsence":      seedConfigInvalidationTestCache(s.quotaAbsentCache),
		"rankings":          seedConfigInvalidationTestCache(s.rankingsCache),
		"performance":       seedConfigInvalidationTestCache(s.performanceCache),
	}
	keyTouchPresent := seedConfigInvalidationTestCache(s.keyTouchCache)
	s.invalidateConfigCaches()
	for name, present := range checks {
		if present() {
			t.Errorf("%s cache was not cleared", name)
		}
	}
	if !keyTouchPresent() {
		t.Fatal("runtime key-touch state was cleared")
	}
}
