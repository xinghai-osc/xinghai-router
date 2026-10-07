package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

const (
	// maxCacheEntries bounds a ttlCache so an attacker cannot grow it without limit by
	// requesting unknown models. The oldest entry is evicted once the bound is reached.
	maxCacheEntries = 4096

	pricingCacheTTL     = 10 * time.Second
	groupCacheTTL       = 30 * time.Second
	reliabilityCacheTTL = 10 * time.Second
	// channelCacheTTL bounds how long a per-(group,model) channel list and a
	// channel's decrypted keys are reused before being re-read from the database.
	// The gateway hot path would otherwise issue 1+N queries per proxied request.
	// Writers that change channel configuration invalidate the cache eagerly, so
	// the TTL is only a safety net for missed invalidation paths.
	channelCacheTTL = 30 * time.Second
	// subscriptionCacheTTL bounds how long a user's subscription coverage answer is
	// reused. Coverage is derived from per-period usage aggregates that would cost a
	// query per request otherwise. A short TTL means a subscription limit is
	// enforced up to TTL seconds late, which is acceptable soft-quota behavior.
	subscriptionCacheTTL = 10 * time.Second
	// quotaCacheTTL bounds how long key/channel quota verdicts are reused. The
	// aggregation they avoid runs on every proxied request otherwise; the short
	// window keeps enforcement lag small while still removing the per-request
	// database round-trips for keys and channels without quota limits.
	quotaCacheTTL = 5 * time.Second
	// rankingsCacheTTL short-circuits repeated multi-table rankings aggregations.
	// Leaders change slowly, so the drift from a 30s window is acceptable for a
	// public page and it protects the database against request floods.
	rankingsCacheTTL = 30 * time.Second
	// performanceCacheTTL short-circuits the per-model request_logs aggregation
	// behind /model-performance while a detail panel stays open.
	performanceCacheTTL = 30 * time.Second
	// keyTouchInterval is how often api_keys.last_used_at is refreshed for a busy key.
	keyTouchInterval = time.Minute
)

type cacheEntry[V any] struct {
	value   V
	expires time.Time
}

type cacheFlight[V any] struct {
	done       chan struct{}
	generation uint64
	value      V
	err        error
	retry      bool
}

// ttlCache memoises short-lived reads of slow-changing configuration rows. Values may
// be up to ttl stale; every writer that changes the underlying row calls invalidate.
// When the cache reaches maxCacheEntries, the oldest entry is evicted (FIFO) rather
// than dropping the entire map, which avoids a thundering-herd of cache misses.
type ttlCache[K comparable, V any] struct {
	mu         sync.Mutex
	ttl        time.Duration
	entries    map[K]cacheEntry[V]
	flights    map[K]*cacheFlight[V]
	generation uint64
	order      []K // FIFO eviction order; updated on store/invalidate
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{ttl: ttl, entries: map[K]cacheEntry[V]{}, order: make([]K, 0, 64)}
}

func (c *ttlCache[K, V]) lookup(key K) (V, bool) {
	value, ok, _ := c.lookupGeneration(key)
	return value, ok
}

func (c *ttlCache[K, V]) lookupGeneration(key K) (V, bool, uint64) {
	var zero V
	if c == nil {
		return zero, false, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !time.Now().Before(entry.expires) {
		return zero, false, c.generation
	}
	return entry.value, true, c.generation
}

func (c *ttlCache[K, V]) store(key K, value V) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storeLocked(key, value)
}

func (c *ttlCache[K, V]) storeIfGeneration(key K, value V, generation uint64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation == generation {
		c.storeLocked(key, value)
	}
}

func (c *ttlCache[K, V]) storeLocked(key K, value V) {
	if _, exists := c.entries[key]; !exists {
		if len(c.entries) >= maxCacheEntries && len(c.order) > 0 {
			oldest := c.order[0]
			delete(c.entries, oldest)
			c.order = c.order[1:]
		}
		c.order = append(c.order, key)
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
	c.storeLocked(key, value)
	return true
}

// get returns the cached value for key, loading and caching it on a miss. Failed loads
// are not cached, so a transient database error does not stick for the whole ttl.
func (c *ttlCache[K, V]) get(ctx context.Context, key K, load func(context.Context) (V, error)) (V, error) {
	if c == nil {
		return load(ctx)
	}
	for {
		c.mu.Lock()
		if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expires) {
			c.mu.Unlock()
			return entry.value, nil
		}
		if err := ctx.Err(); err != nil {
			c.mu.Unlock()
			var zero V
			return zero, err
		}
		if flight := c.flights[key]; flight != nil && flight.generation == c.generation {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				var zero V
				return zero, ctx.Err()
			case <-flight.done:
				if flight.retry {
					continue
				}
				return flight.value, flight.err
			}
		}
		flight := &cacheFlight[V]{done: make(chan struct{}), generation: c.generation, retry: true}
		if c.flights == nil {
			c.flights = make(map[K]*cacheFlight[V])
		}
		c.flights[key] = flight
		c.mu.Unlock()
		return c.loadFlight(ctx, key, flight, load)
	}
}

func (c *ttlCache[K, V]) loadFlight(ctx context.Context, key K, flight *cacheFlight[V], load func(context.Context) (V, error)) (V, error) {
	defer func() {
		c.mu.Lock()
		if c.flights[key] == flight {
			delete(c.flights, key)
			if !flight.retry && flight.err == nil && c.generation == flight.generation {
				c.storeLocked(key, flight.value)
			}
		}
		close(flight.done)
		c.mu.Unlock()
	}()
	flight.value, flight.err = load(ctx)
	flight.retry = flight.err != nil && ctx.Err() != nil
	return flight.value, flight.err
}

func (c *ttlCache[K, V]) invalidate(key K) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	if _, ok := c.entries[key]; ok {
		delete(c.entries, key)
		for i, k := range c.order {
			if k == key {
				copy(c.order[i:], c.order[i+1:])
				c.order = c.order[:len(c.order)-1]
				break
			}
		}
	}
}

func (c *ttlCache[K, V]) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.generation++
	c.entries = map[K]cacheEntry[V]{}
	c.order = c.order[:0]
	c.mu.Unlock()
}

// pricingTier is a single volume band in a tiered pricing rule.
type pricingTier struct {
	fromTokens                 int64
	input, cachedInput, output float64
}

// pricingTimeRule is a time-windowed price override.
type pricingTimeRule struct {
	startMinute, endMinute     int
	weekdays                   string
	input, cachedInput, output float64
}

// pricingRule is the cached form of a row in pricing_rules. found is false when no
// enabled rule exists, which is cached too so unpriced models cost one query per ttl.
// tiers, when non-empty, override the flat input/cachedInput/output for requests
// whose total token count falls in a tier band. timeRules, when non-empty, are
// evaluated at request time; the last matching rule overrides the base prices.
type pricingRule struct {
	input, cachedInput, output, multiplier, exchangeRate float64
	currency                                             string
	tiers                                                []pricingTier
	timeRules                                            []pricingTimeRule
	dimensions                                           DimensionPrices
	pricedAt                                             time.Time
	version                                              string
	billingMode                                          string
	found                                                bool
}

func (s *Service) pricingFor(ctx context.Context, model string) pricingRule {
	rule, err := s.pricingCache.get(ctx, model, func(ctx context.Context) (pricingRule, error) {
		var rule pricingRule
		var dimensions json.RawMessage
		err := s.db.QueryRow(ctx, `select p.input_per_million,p.cached_input_per_million,p.output_per_million,p.multiplier,p.currency,coalesce(e.rate_to_base,0),p.dimension_prices from pricing_rules p left join exchange_rates e on e.currency=p.currency and e.enabled where p.model=$1 and p.enabled`, model).Scan(&rule.input, &rule.cachedInput, &rule.output, &rule.multiplier, &rule.currency, &rule.exchangeRate, &dimensions)
		if err != nil {
			// A missing row is a valid, cacheable answer; anything else is not cached.
			if isNoRows(err) {
				return pricingRule{}, nil
			}
			return pricingRule{}, err
		}
		rule.dimensions, err = parseDimensionPrices(dimensions)
		if err != nil {
			return pricingRule{}, err
		}
		if rule.currency == "" {
			rule.currency = "CNY"
		}
		if rule.exchangeRate <= 0 {
			return pricingRule{}, nil
		}
		rule.found = true
		// Load tiered pricing bands (ordered by from_tokens ascending).
		tr, err := s.db.Query(ctx, `select from_tokens,input_per_million,cached_input_per_million,output_per_million from pricing_tiers where model=$1 order by from_tokens`, model)
		if err != nil {
			return pricingRule{}, err
		}
		for tr.Next() {
			var t pricingTier
			if err := tr.Scan(&t.fromTokens, &t.input, &t.cachedInput, &t.output); err != nil {
				tr.Close()
				return pricingRule{}, err
			}
			rule.tiers = append(rule.tiers, t)
		}
		tr.Close()
		if err := tr.Err(); err != nil {
			return pricingRule{}, err
		}
		// Load time-based pricing overrides (ordered by created_at ascending so the
		// last match wins when multiple rules overlap).
		rr, err := s.db.Query(ctx, `select start_minute,end_minute,weekdays,input_per_million,cached_input_per_million,output_per_million from pricing_time_rules where model=$1 and enabled order by created_at`, model)
		if err != nil {
			return pricingRule{}, err
		}
		for rr.Next() {
			var tr pricingTimeRule
			if err := rr.Scan(&tr.startMinute, &tr.endMinute, &tr.weekdays, &tr.input, &tr.cachedInput, &tr.output); err != nil {
				rr.Close()
				return pricingRule{}, err
			}
			rule.timeRules = append(rule.timeRules, tr)
		}
		rr.Close()
		if err := rr.Err(); err != nil {
			return pricingRule{}, err
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

func (s *Service) userConcurrencyLimitFor(ctx context.Context, userID string) int {
	if userID == "" {
		return 0
	}
	limit, err := s.userConcurrencyCache.get(ctx, userID, func(ctx context.Context) (int, error) {
		var value sql.NullInt64
		if err := s.db.QueryRow(ctx, `select max_concurrency from users where id=$1`, userID).Scan(&value); err != nil {
			if isNoRows(err) {
				return 0, nil
			}
			return 0, err
		}
		if !value.Valid || value.Int64 <= 0 {
			return 0, nil
		}
		return int(value.Int64), nil
	})
	if err != nil {
		return 0
	}
	return limit
}
