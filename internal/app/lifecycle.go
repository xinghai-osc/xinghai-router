package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type requestLifecycle struct {
	mu       sync.Mutex
	draining bool
	stopping bool
	active   int
	idle     chan struct{}
}

func (s *Service) BeginDrain() {
	s.lifecycle.mu.Lock()
	s.lifecycle.draining = true
	s.lifecycle.mu.Unlock()
}

func (s *Service) isDraining() bool {
	s.lifecycle.mu.Lock()
	defer s.lifecycle.mu.Unlock()
	return s.lifecycle.draining
}

func (s *Service) trackRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := &s.lifecycle
		l.mu.Lock()
		if l.stopping {
			l.mu.Unlock()
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, "draining", "server is draining")
			return
		}
		if l.active == 0 {
			l.idle = make(chan struct{})
		}
		l.active++
		l.mu.Unlock()
		defer func() {
			l.mu.Lock()
			l.active--
			if l.active == 0 {
				close(l.idle)
			}
			l.mu.Unlock()
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Service) Shutdown(ctx context.Context, server *http.Server) error {
	s.BeginDrain()
	log.Printf("event=server_draining propagation_delay=%s", s.cfg.ShutdownDelay)
	if s.cfg.ShutdownDelay > 0 {
		timer := time.NewTimer(s.cfg.ShutdownDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	l := &s.lifecycle
	l.mu.Lock()
	l.stopping = true
	idle := l.idle
	active := l.active
	l.mu.Unlock()
	log.Printf("event=server_waiting active_requests=%d", active)
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if idle != nil {
		select {
		case <-idle:
		case <-ctx.Done():
			return fmt.Errorf("wait for active requests: %w", ctx.Err())
		}
	}
	log.Printf("event=server_drained")
	return nil
}

func loadShutdownConfig(c *Config) error {
	c.ShutdownDelay = 10 * time.Second
	c.ShutdownTimeout = 0
	for _, item := range []struct {
		name string
		dest *time.Duration
	}{
		{"SHUTDOWN_DELAY", &c.ShutdownDelay},
		{"SHUTDOWN_TIMEOUT", &c.ShutdownTimeout},
	} {
		if raw := os.Getenv(item.name); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value < 0 {
				return fmt.Errorf("%s must be a non-negative duration", item.name)
			}
			*item.dest = value
		}
	}
	if c.ShutdownTimeout > 0 && c.ShutdownTimeout <= c.ShutdownDelay {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must exceed SHUTDOWN_DELAY or be 0s for unlimited draining")
	}
	return nil
}
