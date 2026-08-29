package app

import (
	"hash/fnv"
	"net/http"
	"sync"
	"time"
)

const (
	authLoginPerMinute                = 10
	authRegisterPerMinute             = 5
	authEmailCodePerMinute            = 5
	authPasswordResetPerMinute        = 3
	authPasswordResetConfirmPerMinute = 5
	// rankingsPerMinute bounds the unauthenticated /rankings endpoint, which lands
	// an expensive multi-table aggregation on every direct hit. The 30s response
	// cache absorbs repeats, so the limiter need only stop deliberate floods.
	rankingsPerMinute = 60
	// performancePerMinute bounds the unauthenticated /model-performance endpoint,
	// which aggregates request_logs per group for a single model on every panel open.
	// The short cache absorbs repeats from the same model.
	performancePerMinute = 120

	// limiterShards splits the in-process limiter's map so the single mutex is not a
	// contention point for the memory fallback under concurrent gateway traffic.
	limiterShards = 16
)

type rateLimiter interface {
	allow(key string) bool
	allowN(key string, n int) bool
	cleanup()
	close()
}

type rateWindow struct {
	start time.Time
	count int
}

type memoryLimiter struct {
	perMinute int
	shards    [limiterShards]memoryLimiterShard
}

type memoryLimiterShard struct {
	mu      sync.Mutex
	entries map[string]rateWindow
}

func newMemoryLimiter(n int) *memoryLimiter {
	if n <= 0 {
		n = 60
	}
	l := &memoryLimiter{perMinute: n}
	for i := range l.shards {
		l.shards[i].entries = map[string]rateWindow{}
	}
	return l
}

func newLimiter(n int) *memoryLimiter { return newMemoryLimiter(n) }

func (l *memoryLimiter) shard(key string) *memoryLimiterShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return &l.shards[h.Sum32()%limiterShards]
}

func (l *memoryLimiter) allow(key string) bool {
	return l.allowN(key, l.perMinute)
}

func (l *memoryLimiter) allowN(key string, n int) bool {
	if n <= 0 {
		n = l.perMinute
	}
	sh := l.shard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	now := time.Now()
	w := sh.entries[key]
	if now.Sub(w.start) >= time.Minute {
		w = rateWindow{start: now}
	}
	if w.count >= n {
		sh.entries[key] = w
		return false
	}
	w.count++
	sh.entries[key] = w
	return true
}

// cleanup removes entries that have not been touched in over a minute.
// Call it periodically to prevent unbounded map growth.
func (l *memoryLimiter) cleanup() {
	for i := range l.shards {
		sh := &l.shards[i]
		sh.mu.Lock()
		for k, w := range sh.entries {
			if time.Since(w.start) > time.Minute {
				delete(sh.entries, k)
			}
		}
		sh.mu.Unlock()
	}
}

// clientIP extracts the real client IP only when the direct peer is a configured
// trusted proxy. Direct clients cannot rotate forwarded headers to bypass limits.
func clientIP(r *http.Request) string {
	trustedProxiesMu.RLock()
	nets := trustedProxyNets
	trustedProxiesMu.RUnlock()
	return clientIPFromRequest(r, nets)
}

// ipRateLimit is middleware that rate-limits by client IP.
func (s *Service) ipRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return s.ipRateLimitBy(s.ipLimiter, next)
}

// ipRateLimitBy rate-limits a handler by client IP using the given limiter, so
// public endpoints can carry a looser budget than login-style routes.
func (s *Service) ipRateLimitBy(limit rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limit.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests from this IP address")
			return
		}
		next(w, r)
	}
}

func (l *memoryLimiter) close() {}

type fallbackLimiter struct {
	primary *redisLimiter
	backup  *memoryLimiter
}

func (l *fallbackLimiter) allow(key string) bool {
	return l.allowN(key, l.backup.perMinute)
}

func (l *fallbackLimiter) allowN(key string, n int) bool {
	ok, err := l.primary.tryAllowN(key, n)
	if err != nil {
		return l.backup.allowN(key, n)
	}
	return ok
}

func (l *fallbackLimiter) cleanup() {
	l.backup.cleanup()
}

func (l *fallbackLimiter) close() {
	l.primary.close()
	l.backup.close()
}

func newRateLimiter(redisURL string, perMinute int) (rateLimiter, string) {
	mem := newMemoryLimiter(perMinute)
	if redisURL == "" {
		return mem, "memory"
	}
	redis, err := newRedisLimiter(redisURL, perMinute)
	if err != nil {
		return mem, "memory"
	}
	return &fallbackLimiter{primary: redis, backup: mem}, "redis"
}
