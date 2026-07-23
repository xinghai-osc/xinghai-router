package app

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateWindow struct {
	start time.Time
	count int
}
type limiter struct {
	mu        sync.Mutex
	perMinute int
	entries   map[string]rateWindow
}

func newLimiter(n int) *limiter { return &limiter{perMinute: n, entries: map[string]rateWindow{}} }
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w := l.entries[key]
	if now.Sub(w.start) >= time.Minute {
		w = rateWindow{start: now}
	}
	if w.count >= l.perMinute {
		l.entries[key] = w
		return false
	}
	w.count++
	l.entries[key] = w
	return true
}

// cleanup removes entries that have not been touched in over a minute.
// Call it periodically to prevent unbounded map growth.
func (l *limiter) cleanup() {
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
