package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestShutdownConfig(t *testing.T) {
	for _, tc := range []struct {
		name, delay, timeout   string
		wantDelay, wantTimeout time.Duration
		invalid                bool
	}{
		{name: "defaults", wantDelay: 10 * time.Second},
		{name: "unlimited", delay: "0s", timeout: "0s"},
		{name: "bounded", delay: "2s", timeout: "3m", wantDelay: 2 * time.Second, wantTimeout: 3 * time.Minute},
		{name: "negative delay", delay: "-1s", invalid: true},
		{name: "negative timeout", timeout: "-1s", invalid: true},
		{name: "bad delay", delay: "tomorrow", invalid: true},
		{name: "bad timeout", timeout: "tomorrow", invalid: true},
		{name: "timeout too short", timeout: "5s", invalid: true},
		{name: "timeout equals delay", timeout: "10s", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SHUTDOWN_DELAY", tc.delay)
			t.Setenv("SHUTDOWN_TIMEOUT", tc.timeout)
			var cfg Config
			err := loadShutdownConfig(&cfg)
			if (err != nil) != tc.invalid {
				t.Fatalf("loadShutdownConfig error = %v", err)
			}
			if !tc.invalid && (cfg.ShutdownDelay != tc.wantDelay || cfg.ShutdownTimeout != tc.wantTimeout) {
				t.Fatalf("delay=%s timeout=%s", cfg.ShutdownDelay, cfg.ShutdownTimeout)
			}
		})
	}
}

func TestDrainReadinessAndPropagation(t *testing.T) {
	s := &Service{}
	s.BeginDrain()
	s.BeginDrain()
	for _, path := range []string{"/readyz", "/healthz"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		want := http.StatusOK
		if path == "/readyz" {
			want = http.StatusServiceUnavailable
			if !strings.Contains(rec.Body.String(), "draining") {
				t.Fatalf("readiness body = %s", rec.Body.String())
			}
		}
		if rec.Code != want {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, want)
		}
	}
	called := false
	handler := s.trackRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatal("request rejected before load balancer propagation")
	}
	if err := s.Shutdown(context.Background(), &http.Server{}); err != nil {
		t.Fatal(err)
	}
	called = false
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	if called || rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "1" {
		t.Fatal("new request admitted after listener shutdown")
	}
}

func TestShutdownPreservesSSEAndWaitsForSettlement(t *testing.T) {
	s := &Service{}
	releaseStream := make(chan struct{})
	releaseSettlement := make(chan struct{})
	settling := make(chan struct{})
	server := httptest.NewServer(s.trackRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-releaseStream
		if r.Context().Err() != nil {
			t.Error("shutdown cancelled the in-flight stream")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
		close(settling)
		<-releaseSettlement
	})))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	stopping := make(chan struct{})
	server.Config.RegisterOnShutdown(func() { close(stopping) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Shutdown(ctx, server.Config) }()
	<-stopping
	close(releaseStream)
	<-settling
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before settlement: %v", err)
	default:
	}
	close(releaseSettlement)
	tail, err := io.ReadAll(response.Body)
	if err != nil || string(tail) != "data: [DONE]\n\n" {
		t.Fatalf("stream tail = %q, error = %v", tail, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestShutdownWaitsForHijackedConnection(t *testing.T) {
	s := &Service{}
	release := make(chan struct{})
	accepted := make(chan struct{})
	server := httptest.NewServer(s.trackRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		close(accepted)
		<-release
		if err := conn.Write(r.Context(), websocket.MessageText, []byte("generation completed")); err != nil {
			t.Error(err)
		}
	})))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	<-accepted
	stopping := make(chan struct{})
	server.Config.RegisterOnShutdown(func() { close(stopping) })
	done := make(chan error, 1)
	go func() { done <- s.Shutdown(ctx, server.Config) }()
	<-stopping
	select {
	case err := <-done:
		t.Fatalf("shutdown ignored hijacked handler: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	_, payload, err := conn.Read(ctx)
	if err != nil || string(payload) != "generation completed" {
		t.Fatalf("payload = %q, error = %v", payload, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestShutdownTimeoutIncludesPropagation(t *testing.T) {
	s := &Service{cfg: Config{ShutdownDelay: time.Hour}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Shutdown(ctx, &http.Server{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown error = %v", err)
	}
	if !s.isDraining() {
		t.Fatal("cancelled shutdown reset readiness")
	}
}

func TestShutdownDeadlineKeepsActiveHandlerTracked(t *testing.T) {
	s := &Service{}
	entered := make(chan struct{})
	release := make(chan struct{})
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		s.trackRequests(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			close(entered)
			<-release
		})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := s.Shutdown(ctx, &http.Server{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("shutdown error = %v", err)
	}
	s.lifecycle.mu.Lock()
	active := s.lifecycle.active
	s.lifecycle.mu.Unlock()
	if active != 1 {
		t.Errorf("active handlers after timeout = %d", active)
	}
	close(release)
	<-requestDone
	if err := s.Shutdown(context.Background(), &http.Server{}); err != nil {
		t.Fatal(err)
	}
}

func TestDrainingWebSocketRequiresReconnectBeforeGeneration(t *testing.T) {
	s := &Service{}
	server := httptest.NewServer(s.trackRequests(http.HandlerFunc(s.responsesWebSocket)))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	s.BeginDrain()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"model":"test","input":"hello"}`)); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusServiceRestart {
		t.Fatalf("expected service restart close, got %v", err)
	}
}
