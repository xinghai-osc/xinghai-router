package app

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	authLoginPerMinute     = 10
	authRegisterPerMinute  = 5
	authEmailCodePerMinute = 5
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
	mu        sync.Mutex
	perMinute int
	entries   map[string]rateWindow
}

func newMemoryLimiter(n int) *memoryLimiter {
	if n <= 0 {
		n = 60
	}
	return &memoryLimiter{perMinute: n, entries: map[string]rateWindow{}}
}

func newLimiter(n int) *memoryLimiter { return newMemoryLimiter(n) }

func (l *memoryLimiter) allow(key string) bool {
	return l.allowN(key, l.perMinute)
}

func (l *memoryLimiter) allowN(key string, n int) bool {
	if n <= 0 {
		n = l.perMinute
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w := l.entries[key]
	if now.Sub(w.start) >= time.Minute {
		w = rateWindow{start: now}
	}
	if w.count >= n {
		l.entries[key] = w
		return false
	}
	w.count++
	l.entries[key] = w
	return true
}

// cleanup removes entries that have not been touched in over a minute.
// Call it periodically to prevent unbounded map growth.
func (l *memoryLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, w := range l.entries {
		if time.Since(w.start) > time.Minute {
			delete(l.entries, k)
		}
	}
}

// clientIP extracts the real client IP from headers or the remote address.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := firstIP(fwd); ip != "" {
			return ip
		}
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		if ip := firstIP(real); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func firstIP(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			s = s[i:]
			break
		}
	}
	if idx := commaIndex(s); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func commaIndex(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

// ipRateLimit is middleware that rate-limits by client IP.
func (s *Service) ipRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.ipLimiter.allow(clientIP(r)) {
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
