package app

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

const (
	// maxCacheEntries bounds a ttlCache so an attacker cannot grow it without limit by
	// requesting unknown models. The whole map is dropped once the bound is reached.
	maxCacheEntries = 4096

	pricingCacheTTL     = 10 * time.Second
	groupCacheTTL       = 30 * time.Second
	reliabilityCacheTTL = 10 * time.Second
	// keyTouchInterval is how often api_keys.last_used_at is refreshed for a busy key.
	keyTouchInterval = time.Minute
)

type cacheEntry[V any] struct {
	value   V
	expires time.Time
}

// ttlCache memoises short-lived reads of slow-changing configuration rows. Values may
// be up to ttl stale; every writer that changes the underlying row calls invalidate.
type ttlCache[K comparable, V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[K]cacheEntry[V]
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{ttl: ttl, entries: map[K]cacheEntry[V]{}}
}

func (c *ttlCache[K, V]) lookup(key K) (V, bool) {
	var zero V
	if c == nil {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		return zero, false
	}
	return entry.value, true
}

func (c *ttlCache[K, V]) store(key K, value V) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= maxCacheEntries {
		c.entries = map[K]cacheEntry[V]{}
	}
	c.entries[key] = cacheEntry[V]{value: value, expires: time.Now().Add(c.ttl)}
}

// storeOnce records key only if it is absent or expired, reporting whether it was
// stored. It is used to rate-limit periodic side effects such as last-used stamps.
func (c *ttlCache[K, V]) storeOnce(key K, value V) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expires) {
		return false
	}
	if len(c.entries) >= maxCacheEntries {
		c.entries = map[K]cacheEntry[V]{}
	}
	c.entries[key] = cacheEntry[V]{value: value, expires: time.Now().Add(c.ttl)}
	return true
}

// get returns the cached value for key, loading and caching it on a miss. Failed loads
// are not cached, so a transient database error does not stick for the whole ttl.
func (c *ttlCache[K, V]) get(ctx context.Context, key K, load func(context.Context) (V, error)) (V, error) {
	if value, ok := c.lookup(key); ok {
		return value, nil
	}
	value, err := load(ctx)
	if err != nil {
		return value, err
	}
	c.store(key, value)
	return value, nil
}

func (c *ttlCache[K, V]) invalidate(key K) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = map[K]cacheEntry[V]{}
	c.mu.Unlock()
}

// pricingRule is the cached form of a row in pricing_rules. found is false when no
// enabled rule exists, which is cached too so unpriced models cost one query per ttl.
type pricingRule struct {
	input, cachedInput, output, multiplier float64
	found                                  bool
}

func (s *Service) pricingFor(ctx context.Context, model string) pricingRule {
	rule, err := s.pricingCache.get(ctx, model, func(ctx context.Context) (pricingRule, error) {
		var rule pricingRule
		err := s.db.QueryRow(ctx, `select input_per_million,cached_input_per_million,output_per_million,multiplier from pricing_rules where model=$1 and enabled`, model).Scan(&rule.input, &rule.cachedInput, &rule.output, &rule.multiplier)
		if err != nil {
			// A missing row is a valid, cacheable answer; anything else is not cached.
			if isNoRows(err) {
				return pricingRule{}, nil
			}
			return pricingRule{}, err
		}
		rule.found = true
		return rule, nil
	})
	if err != nil {
		return pricingRule{}
	}
	return rule
}

// groupMultiplierFor returns the billing multiplier of a group, defaulting to 1 when the
// key has no group or the group cannot be read.
func (s *Service) groupMultiplierFor(ctx context.Context, groupID string) float64 {
	if groupID == "" {
		return 1
	}
	multiplier, err := s.groupCache.get(ctx, groupID, func(ctx context.Context) (float64, error) {
		var multiplier float64
		if err := s.db.QueryRow(ctx, `select multiplier from groups where id=$1`, groupID).Scan(&multiplier); err != nil {
			if isNoRows(err) {
				return 1, nil
			}
			return 0, err
		}
		if multiplier <= 0 {
			return 1, nil
		}
		return multiplier, nil
	})
	if err != nil {
		return 1
	}
	return multiplier
}

func (s *Service) groupConcurrencyLimitFor(ctx context.Context, groupID string) int {
	limit, err := s.groupConcurrencyCache.get(ctx, groupID, func(ctx context.Context) (int, error) {
		var limit sql.NullInt64
		if err := s.db.QueryRow(ctx, `select max_concurrency from groups where id=$1`, groupID).Scan(&limit); err != nil {
			if isNoRows(err) {
				return 0, nil
			}
			return 0, err
		}
		if !limit.Valid || limit.Int64 <= 0 {
			return 0, nil
		}
		return int(limit.Int64), nil
	})
	if err != nil {
		return 0
	}
	return limit
}
