package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type limiterTestRedis struct {
	listener net.Listener
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	closed   bool
	workers  sync.WaitGroup
}

func startLimiterTestRedis(t *testing.T, addr string, handler func([]string) string) *limiterTestRedis {
	t.Helper()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	s := &limiterTestRedis{listener: listener, conns: make(map[net.Conn]struct{})}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				_ = conn.Close()
				return
			}
			s.conns[conn] = struct{}{}
			s.workers.Add(1)
			s.mu.Unlock()
			go func() {
				defer s.workers.Done()
				defer conn.Close()
				defer func() {
					s.mu.Lock()
					delete(s.conns, conn)
					s.mu.Unlock()
				}()
				reader := bufio.NewReader(conn)
				for {
					reply, err := readReply(reader)
					if err != nil {
						return
					}
					items, ok := reply.([]any)
					if !ok {
						return
					}
					args := make([]string, len(items))
					for i, item := range items {
						arg, ok := item.(string)
						if !ok {
							return
						}
						args[i] = arg
					}
					response := handler(args)
					if response == "" {
						return
					}
					if _, err := io.WriteString(conn, response); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(s.close)
	return s
}

func (s *limiterTestRedis) close() {
	s.mu.Lock()
	s.closed = true
	_ = s.listener.Close()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.workers.Wait()
}

func (s *limiterTestRedis) url() string {
	return "redis://" + s.listener.Addr().String()
}

func unusedLimiterTestAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func captureLimiterTestLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	writer, flags, prefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(writer)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
	return &output
}

func TestConfiguredRateLimiterPolicies(t *testing.T) {
	for _, policy := range []string{"", "memory", " MEMORY "} {
		limit, mode, err := newConfiguredRateLimiter("", 1, policy)
		if err != nil || mode != "memory" {
			t.Fatalf("policy %q: mode=%q err=%v", policy, mode, err)
		}
		if allowed, err := allowRateLimit(limit, "key", 1); !allowed || err != nil {
			t.Fatalf("memory first request: allowed=%v err=%v", allowed, err)
		}
		if allowed, err := allowRateLimit(limit, "key", 1); allowed || err != nil {
			t.Fatalf("memory rejection: allowed=%v err=%v", allowed, err)
		}
		limit.close()
	}
	for _, policy := range []string{"deny", "unsupported"} {
		limit, _, err := newConfiguredRateLimiter("", 1, policy)
		if err == nil || limit != nil {
			t.Fatalf("policy %q without redis: limit=%v err=%v", policy, limit, err)
		}
	}
	for _, redisURL := range []string{
		"http://localhost:6379",
		"redis://",
		"redis://localhost:6379/not-a-db",
		"redis://localhost:0",
		"redis://localhost:65536",
		"redis://localhost:",
		"redis://localhost:6379?password=fake-secret",
		"redis://localhost:6379#fake-secret",
		"redis://:fake-secret@localhost:invalid",
	} {
		for _, policy := range []string{"memory", "deny"} {
			limit, _, err := newConfiguredRateLimiter(redisURL, 1, policy)
			if err == nil || limit != nil {
				t.Fatalf("invalid URL under %s: limit=%v err=%v", policy, limit, err)
			}
			if strings.Contains(err.Error(), "fake-secret") || strings.Contains(err.Error(), redisURL) {
				t.Fatalf("configuration error exposes redis URL: %v", err)
			}
		}
	}
	limit, mode := newRateLimiter("not-a-url", 1)
	defer limit.close()
	if mode != "memory" || !limit.allow("legacy") {
		t.Fatal("legacy constructor no longer falls back on invalid redis URL")
	}
}

func TestConfiguredRateLimiterDenyInitialOutage(t *testing.T) {
	output := captureLimiterTestLog(t)
	limit, mode, err := newConfiguredRateLimiter("redis://"+unusedLimiterTestAddress(t), 1, "deny")
	if !errors.Is(err, errRedisUnavailable) || limit != nil || mode != "" {
		t.Fatalf("deny startup outage: limit=%v mode=%q err=%v", limit, mode, err)
	}
	if strings.Count(output.String(), "event=redis_degraded") != 1 {
		t.Fatalf("missing startup degraded alert: %q", output.String())
	}
}

func TestConfiguredRateLimiterInitialOutageRecovers(t *testing.T) {
	output := captureLimiterTestLog(t)
	addr := unusedLimiterTestAddress(t)
	limit, mode, err := newConfiguredRateLimiter("redis://"+addr, 1, "memory")
	if err != nil || mode != "memory" {
		t.Fatalf("memory startup outage: mode=%q err=%v", mode, err)
	}
	defer limit.close()
	if _, ok := limit.(*fallbackLimiter); !ok {
		t.Fatal("startup outage discarded reconnectable redis client")
	}
	if allowed, err := allowRateLimit(limit, "key", 1); !allowed || err != nil {
		t.Fatalf("memory fallback first request: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := allowRateLimit(limit, "key", 1); allowed || err != nil {
		t.Fatalf("memory fallback rejection: allowed=%v err=%v", allowed, err)
	}
	if strings.Count(output.String(), "event=redis_degraded") != 1 {
		t.Fatalf("duplicate degraded alert: %q", output.String())
	}
	var count atomic.Int64
	startLimiterTestRedis(t, addr, func(args []string) string {
		if args[0] == "EVAL" {
			return fmt.Sprintf(":%d\r\n", count.Add(1))
		}
		return "+PONG\r\n"
	})
	if allowed, err := allowRateLimit(limit, "key", 1); !allowed || err != nil {
		t.Fatalf("recovered Redis did not replace exhausted memory window: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := allowRateLimit(limit, "key", 1); allowed || err != nil {
		t.Fatalf("Redis actual rate rejection: allowed=%v err=%v", allowed, err)
	}
	if strings.Count(output.String(), "event=redis_recovered") != 1 {
		t.Fatalf("expected one recovery alert: %q", output.String())
	}
	if strings.Contains(output.String(), addr) {
		t.Fatalf("alert exposes redis address: %q", output.String())
	}
}

func TestConfiguredRateLimiterDenyRuntimeOutageAndAlerts(t *testing.T) {
	output := captureLimiterTestLog(t)
	var outage atomic.Bool
	var count atomic.Int64
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		if outage.Load() {
			return "-ERR fake-secret redis://private-redis/0\r\n"
		}
		if args[0] == "EVAL" {
			return fmt.Sprintf(":%d\r\n", count.Add(1))
		}
		return "+PONG\r\n"
	})
	limit, mode, err := newConfiguredRateLimiter(server.url(), 1, "deny")
	if err != nil || mode != "redis" {
		t.Fatalf("deny startup: mode=%q err=%v", mode, err)
	}
	defer limit.close()
	if !limit.allow("key") {
		t.Fatal("healthy Redis rejected first request")
	}
	if allowed, err := allowRateLimit(limit, "key", 1); allowed || err != nil {
		t.Fatalf("healthy Redis rate rejection: allowed=%v err=%v", allowed, err)
	}
	if output.Len() != 0 {
		t.Fatalf("healthy rate rejection emitted outage alert: %q", output.String())
	}
	outage.Store(true)
	for range 3 {
		if allowed, err := allowRateLimit(limit, "key", 1); allowed || !errors.Is(err, errRedisUnavailable) {
			t.Fatalf("deny outage must return availability error: allowed=%v err=%v", allowed, err)
		}
	}
	if limit.allow("legacy") || limit.allowN("legacy", 1) {
		t.Fatal("boolean compatibility methods must fail closed under deny")
	}
	if !limit.(*fallbackLimiter).backup.allow("key") {
		t.Fatal("deny policy used memory fallback")
	}
	outage.Store(false)
	for range 3 {
		if allowed, err := allowRateLimit(limit, "key", 1); allowed || err != nil {
			t.Fatalf("recovery rate rejection: allowed=%v err=%v", allowed, err)
		}
	}
	outage.Store(true)
	_, _ = allowRateLimit(limit, "key", 1)
	outage.Store(false)
	_, _ = allowRateLimit(limit, "key", 1)
	logs := output.String()
	if strings.Count(logs, "event=redis_degraded") != 2 || strings.Count(logs, "event=redis_recovered") != 2 {
		t.Fatalf("expected one alert per transition: %q", logs)
	}
	if strings.Contains(logs, "fake-secret") || strings.Contains(logs, "private-redis") || strings.Contains(logs, server.url()) {
		t.Fatalf("alert leaked redis details: %q", logs)
	}
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		if len(line) > 160 || !strings.Contains(line, "component=rate_limiter policy=deny") {
			t.Fatalf("unexpected unbounded or unstructured alert: %q", line)
		}
	}
}

func TestIPRateLimitRedisFailureIsUnavailable(t *testing.T) {
	captureLimiterTestLog(t)
	var outage atomic.Bool
	server := startLimiterTestRedis(t, "127.0.0.1:0", func(args []string) string {
		if outage.Load() {
			return "-ERR unavailable\r\n"
		}
		if args[0] == "EVAL" {
			return ":2\r\n"
		}
		return "+PONG\r\n"
	})
	limit, _, err := newConfiguredRateLimiter(server.url(), 1, "deny")
	if err != nil {
		t.Fatal(err)
	}
	defer limit.close()
	service := &Service{}
	handler := service.ipRateLimitBy(limit, func(w http.ResponseWriter, r *http.Request) {
		t.Error("rate-limited request reached protected handler")
	})
	for _, unavailable := range []bool{false, true} {
		outage.Store(unavailable)
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodGet, "/", nil))
		want := http.StatusTooManyRequests
		if unavailable {
			want = http.StatusServiceUnavailable
		}
		if response.Code != want {
			t.Fatalf("unavailable=%v status=%d want=%d body=%s", unavailable, response.Code, want, response.Body.String())
		}
	}
}

func TestRedisDependencyAlertConcurrentTransitions(t *testing.T) {
	output := captureLimiterTestLog(t)
	var alert redisDependencyAlert
	for _, degraded := range []bool{true, false} {
		var workers sync.WaitGroup
		for range 32 {
			workers.Go(func() { alert.setDegraded(degraded, "untrusted-secret", "untrusted-secret") })
		}
		workers.Wait()
	}
	logs := output.String()
	if strings.Count(logs, "event=redis_degraded") != 1 || strings.Count(logs, "event=redis_recovered") != 1 {
		t.Fatalf("concurrent duplicate transition alerts: %q", logs)
	}
	if strings.Contains(logs, "untrusted-secret") {
		t.Fatalf("alert allowed unbounded field content: %q", logs)
	}
}
