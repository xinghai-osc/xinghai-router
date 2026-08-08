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
	// rankingsCacheTTL short-circuits repeated multi-table rankings aggregations.
	// Leaders change slowly, so the drift from a 30s window is acceptable for a
	// public page and it protects the database against request floods.
	rankingsCacheTTL = 30 * time.Second
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

// pricingTier is a single volume band in a tiered pricing rule.
type pricingTier struct {
	fromTokens                              int64
	input, cachedInput, output              float64
}

// pricingTimeRule is a time-windowed price override.
type pricingTimeRule struct {
	startMinute, endMinute                  int
	weekdays                                string
	input, cachedInput, output              float64
}

// pricingRule is the cached form of a row in pricing_rules. found is false when no
// enabled rule exists, which is cached too so unpriced models cost one query per ttl.
// tiers, when non-empty, override the flat input/cachedInput/output for requests
// whose total token count falls in a tier band. timeRules, when non-empty, are
// evaluated at request time; the last matching rule overrides the base prices.
type pricingRule struct {
	input, cachedInput, output, multiplier  float64
	tiers                                   []pricingTier
	timeRules                               []pricingTimeRule
	found                                   bool
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
		// Load tiered pricing bands (ordered by from_tokens ascending).
		tr, err := s.db.Query(ctx, `select from_tokens,input_per_million,cached_input_per_million,output_per_million from pricing_tiers where model=$1 order by from_tokens`, model)
		if err == nil {
			for tr.Next() {
				var t pricingTier
				if tr.Scan(&t.fromTokens, &t.input, &t.cachedInput, &t.output) == nil {
					rule.tiers = append(rule.tiers, t)
				}
			}
			tr.Close()
		}
		// Load time-based pricing overrides (ordered by created_at ascending so the
		// last match wins when multiple rules overlap).
		rr, err := s.db.Query(ctx, `select start_minute,end_minute,weekdays,input_per_million,cached_input_per_million,output_per_million from pricing_time_rules where model=$1 and enabled order by created_at`, model)
		if err == nil {
			for rr.Next() {
				var tr pricingTimeRule
				if rr.Scan(&tr.startMinute, &tr.endMinute, &tr.weekdays, &tr.input, &tr.cachedInput, &tr.output) == nil {
					rule.timeRules = append(rule.timeRules, tr)
				}
			}
			rr.Close()
		}
		return rule, nil
	})
	if err != nil {
		return pricingRule{}
	}
	return rule
}

// resolvePricing returns the effective prices for a model at the given time,
// applying time-based overrides first, then tiered pricing.
func (r pricingRule) resolvePricing(now time.Time) (input, cachedInput, output float64) {
	input, cachedInput, output = r.input, r.cachedInput, r.output
	if len(r.timeRules) > 0 {
		minute := now.Hour()*60 + now.Minute()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 6 // Sunday → index 6 (Mon=0 … Sun=6)
		} else {
			weekday-- // Go Sunday=0,Mon=1…Sat=6 → Mon=0…Sun=6
		}
		for _, tr := range r.timeRules {
			if len(tr.weekdays) != 7 || tr.weekdays[weekday] != '1' {
				continue
			}
			if tr.startMinute < tr.endMinute {
				if minute >= tr.startMinute && minute < tr.endMinute {
					input, cachedInput, output = tr.input, tr.cachedInput, tr.output
				}
			} else {
				// Wrap-around window (e.g. 22:00 → 06:00).
				if minute >= tr.startMinute || minute < tr.endMinute {
					input, cachedInput, output = tr.input, tr.cachedInput, tr.output
				}
			}
		}
	}
	return input, cachedInput, output
}

// resolveTier returns the tier-specific prices for a given total token count.
// When no tier matches, the provided fallback prices are returned unchanged.
func (r pricingRule) resolveTier(totalTokens int64, fbInput, fbCached, fbOutput float64) (input, cachedInput, output float64) {
	input, cachedInput, output = fbInput, fbCached, fbOutput
	for _, t := range r.tiers {
		if totalTokens >= t.fromTokens {
			input, cachedInput, output = t.input, t.cachedInput, t.output
		} else {
			break
		}
	}
	return input, cachedInput, output
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
