package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func requestLimitsTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://router:fake-password@localhost/router")
	t.Setenv("ENCRYPTION_KEY", "request-limits-unit-test-fake-key")
	for _, key := range []string{
		"GATEWAY_MAX_BODY_BYTES", "IMAGE_MAX_BODY_BYTES", "WS_MAX_MESSAGE_BYTES",
		"REQUEST_BODY_TIMEOUT", "WS_IDLE_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT",
		"HTTP_IDLE_TIMEOUT", "HTTP_MAX_HEADER_BYTES", "CHANNEL_CREDENTIAL_STORAGE", "SESSION_COOKIE_SECURE",
		"DEPLOYMENT_MODE", "REDIS_FAILURE_POLICY", "CONCURRENCY_LEASE_TTL", "TRUSTED_PROXIES",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRequestLimitsConfigDefaults(t *testing.T) {
	requestLimitsTestEnv(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayMaxBodyBytes != 2<<20 || cfg.ImageMaxBodyBytes != 50<<20 || cfg.WSMaxMessageBytes != 2<<20 || cfg.HTTPMaxHeaderBytes != 1<<20 {
		t.Fatalf("unexpected byte limits: gateway=%d image=%d websocket=%d headers=%d", cfg.GatewayMaxBodyBytes, cfg.ImageMaxBodyBytes, cfg.WSMaxMessageBytes, cfg.HTTPMaxHeaderBytes)
	}
	if cfg.RequestBodyTimeout != 30*time.Second || cfg.WSIdleTimeout != 2*time.Minute || cfg.HTTPReadHeaderTimeout != 10*time.Second || cfg.HTTPIdleTimeout != 2*time.Minute {
		t.Fatalf("unexpected timeouts: body=%s websocket=%s headers=%s idle=%s", cfg.RequestBodyTimeout, cfg.WSIdleTimeout, cfg.HTTPReadHeaderTimeout, cfg.HTTPIdleTimeout)
	}
	if cfg.ChannelCredentialStorage != "plaintext" || !cfg.SessionCookieSecure {
		t.Fatal("unexpected credential or cookie defaults")
	}
}

func TestRequestLimitsConfigOverrides(t *testing.T) {
	requestLimitsTestEnv(t)
	for key, value := range map[string]string{
		"GATEWAY_MAX_BODY_BYTES": "123", "IMAGE_MAX_BODY_BYTES": "456", "WS_MAX_MESSAGE_BYTES": "789",
		"REQUEST_BODY_TIMEOUT": "1500ms", "WS_IDLE_TIMEOUT": "3m", "HTTP_READ_HEADER_TIMEOUT": "12s",
		"HTTP_IDLE_TIMEOUT": "4m", "HTTP_MAX_HEADER_BYTES": "8192", "CHANNEL_CREDENTIAL_STORAGE": "encrypted", "SESSION_COOKIE_SECURE": "false",
	} {
		t.Setenv(key, value)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayMaxBodyBytes != 123 || cfg.ImageMaxBodyBytes != 456 || cfg.WSMaxMessageBytes != 789 || cfg.HTTPMaxHeaderBytes != 8192 {
		t.Fatal("byte overrides were not applied")
	}
	if cfg.RequestBodyTimeout != 1500*time.Millisecond || cfg.WSIdleTimeout != 3*time.Minute || cfg.HTTPReadHeaderTimeout != 12*time.Second || cfg.HTTPIdleTimeout != 4*time.Minute {
		t.Fatal("timeout overrides were not applied")
	}
	if cfg.ChannelCredentialStorage != "encrypted" || cfg.SessionCookieSecure {
		t.Fatal("credential or cookie overrides were not applied")
	}
}

func TestRequestLimitsConfigRejectsInvalid(t *testing.T) {
	requestLimitsTestEnv(t)
	for _, key := range []string{"GATEWAY_MAX_BODY_BYTES", "IMAGE_MAX_BODY_BYTES", "WS_MAX_MESSAGE_BYTES", "HTTP_MAX_HEADER_BYTES"} {
		for _, value := range []string{"", "0", "-1", "nonsense", "9223372036854775807", "999999999999999999999999"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				t.Setenv(key, value)
				if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("expected %s validation error, got %v", key, err)
				}
			})
		}
	}
	for _, key := range []string{"REQUEST_BODY_TIMEOUT", "WS_IDLE_TIMEOUT", "HTTP_READ_HEADER_TIMEOUT", "HTTP_IDLE_TIMEOUT"} {
		for _, value := range []string{"", "0s", "-1s", "nonsense", "99999999999999999999h"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				t.Setenv(key, value)
				if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("expected %s validation error, got %v", key, err)
				}
			})
		}
	}
	for key, value := range map[string]string{"CHANNEL_CREDENTIAL_STORAGE": "unknown", "SESSION_COOKIE_SECURE": "yes"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected %s validation error, got %v", key, err)
			}
		})
	}
}

func TestReadRequestBodyLimitBoundaries(t *testing.T) {
	for _, unknownLength := range []bool{false, true} {
		for _, size := range []int{0, 7, 8, 9, 1024} {
			t.Run(fmt.Sprintf("unknown=%v/bytes=%d", unknownLength, size), func(t *testing.T) {
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", size)))
				if unknownLength {
					r.ContentLength = -1
				}
				body, err := readRequestBody(httptest.NewRecorder(), r, 8, time.Second)
				if size <= 8 {
					if err != nil || len(body) != size {
						t.Fatalf("len(body)=%d err=%v", len(body), err)
					}
				} else {
					var oversized *http.MaxBytesError
					if !errors.As(err, &oversized) || oversized.Limit != 8 || body != nil {
						t.Fatalf("expected explicit overflow, body=%q err=%v", body, err)
					}
				}
			})
		}
	}
}

func TestRequestLimitHandlersReturn413(t *testing.T) {
	s := &Service{cfg: Config{GatewayMaxBodyBytes: 8, ImageMaxBodyBytes: 16}}
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		limit   int
	}{
		{"chat", s.chatCompletions, 8},
		{"responses", s.responsesCompletions, 8},
		{"anthropic", s.anthropicMessages, 8},
		{"jev", s.jevCompletions, 8},
		{"images/generations", s.imageGenerations, 16},
		{"images/edits", s.imageEdits, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, unknownLength := range []bool{false, true} {
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", tc.limit+1)))
				r.Header.Set("Anthropic-Version", "2023-06-01")
				r.Header.Set("Content-Type", "multipart/form-data; boundary=example")
				if unknownLength {
					r.ContentLength = -1
				}
				w := httptest.NewRecorder()
				tc.handler(w, r)
				if w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), "request_too_large") {
					t.Fatalf("unknown=%v status=%d body=%s", unknownLength, w.Code, w.Body)
				}
			}
		})
	}
}

type requestDeadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *requestDeadlineRecorder) SetReadDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestRequestBodyDeadlineClearedAfterRead(t *testing.T) {
	w := &requestDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	start := time.Now()
	if _, err := readRequestBody(w, r, 8, time.Second); err != nil {
		t.Fatal(err)
	}
	if len(w.deadlines) != 2 || w.deadlines[0].Before(start.Add(time.Second)) || !w.deadlines[1].IsZero() {
		t.Fatalf("unexpected deadlines: %v", w.deadlines)
	}
	if r.Context().Err() != nil {
		t.Fatalf("request context was canceled after reading: %v", r.Context().Err())
	}
}

func TestRequestBodyErrorStatus(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{fmt.Errorf("wrapped: %w", &http.MaxBytesError{Limit: 8}), http.StatusRequestEntityTooLarge},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), http.StatusRequestTimeout},
		{&net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}, http.StatusRequestTimeout},
		{io.ErrUnexpectedEOF, http.StatusBadRequest},
	} {
		status, _, _ := requestBodyError(tc.err)
		if status != tc.status {
			t.Fatalf("error %v: status=%d, want %d", tc.err, status, tc.status)
		}
	}
}

func TestRequestBodySlowReadAndEarlyOversize(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int
		status int
	}{
		{"slow-body", 8, http.StatusRequestTimeout},
		{"oversized-without-body", 17, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := readRequestBody(w, r, 16, 40*time.Millisecond)
				if err == nil {
					t.Error("expected rejected request")
					return
				}
				status, code, message := requestBodyError(err)
				writeError(w, status, code, message)
			}))
			defer server.Close()
			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\n\r\n", tc.length); err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status || !response.Close {
				t.Fatalf("status=%d close=%v", response.StatusCode, response.Close)
			}
		})
	}
}

func TestRequestBodyTimeoutDoesNotTruncateSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := readRequestBody(w, r, 16, 40*time.Millisecond); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		timer := time.NewTimer(120 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			t.Errorf("stream request canceled: %v", r.Context().Err())
			return
		}
		fmt.Fprint(w, "data: last\n\n")
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	response, err := client.Post(server.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "data: first\n\ndata: last\n\n" {
		t.Fatalf("stream=%q err=%v", body, err)
	}
}

func TestRequestBodyHTTP2ReadTimeout(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("expected HTTP/2, got %s", r.Proto)
		}
		_, err := readRequestBody(w, r, 16, 40*time.Millisecond)
		if err == nil {
			t.Error("expected body timeout")
			return
		}
		status, code, message := requestBodyError(err)
		writeError(w, status, code, message)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL, reader)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.Timeout = 3 * time.Second
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("status=%d, want 408", response.StatusCode)
	}
}

func TestResponsesWSMessageLimit(t *testing.T) {
	for _, compression := range []websocket.CompressionMode{websocket.CompressionDisabled, websocket.CompressionContextTakeover} {
		t.Run(fmt.Sprint(compression), func(t *testing.T) {
			s := &Service{cfg: Config{WSMaxMessageBytes: 64, WSIdleTimeout: time.Second}}
			server := httptest.NewServer(http.HandlerFunc(s.responsesWebSocket))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(ctx, server.URL, &websocket.DialOptions{CompressionMode: compression, CompressionThreshold: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			if err := conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", 256))); err != nil {
				t.Fatal(err)
			}
			_, _, err = conn.Read(ctx)
			if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
				t.Fatalf("expected websocket close 1009, got %v", err)
			}
		})
	}
}

func TestResponsesWSIdleTimeout(t *testing.T) {
	s := &Service{cfg: Config{WSIdleTimeout: 40 * time.Millisecond}}
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		s.responsesWebSocket(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("idle websocket was not closed")
	}
}

func TestResponsesWSIdleTimeoutDoesNotInterruptActiveResponse(t *testing.T) {
	s := &Service{cfg: Config{WSIdleTimeout: 40 * time.Millisecond}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		for i := 0; i < 2; i++ {
			if _, _, err := s.readResponsesWSMessage(r.Context(), conn); err != nil {
				t.Errorf("read: %v", err)
				return
			}
			timer := time.NewTimer(120 * time.Millisecond)
			<-timer.C
			adapter := &wsResponsesAdapter{conn: conn, ctx: r.Context()}
			if _, err := adapter.Write([]byte("data: {\"type\":\"response.completed\"}\n\n")); err != nil {
				t.Errorf("active response interrupted: %v", err)
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	for i := 0; i < 2; i++ {
		if err := conn.Write(ctx, websocket.MessageText, []byte(`{"model":"test"}`)); err != nil {
			t.Fatal(err)
		}
		_, body, err := conn.Read(ctx)
		if err != nil || string(body) != `{"type":"response.completed"}` {
			t.Fatalf("active response=%q err=%v", body, err)
		}
	}
}
