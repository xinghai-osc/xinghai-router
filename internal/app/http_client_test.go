package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpstreamClientsShareTransport(t *testing.T) {
	client := newHTTPClient(time.Second)
	streamClient := newStreamClient()
	if client.Transport != upstreamTransport() || streamClient.Transport != client.Transport {
		t.Fatal("upstream clients must share their connection pool")
	}
	if client.Timeout != time.Second || streamClient.Timeout != 0 {
		t.Fatalf("timeouts: regular=%v stream=%v", client.Timeout, streamClient.Timeout)
	}
}

func TestUpstreamClientsAvoidHTTP2StreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		client      func() *http.Client
		contentType string
		response    string
	}{
		{"json", func() *http.Client { return newHTTPClient(3 * time.Second) }, "application/json", `{"choices":[],"usage":{"total_tokens":3}}`},
		{"sse", newStreamClient, "text/event-stream", "data: first\n\ndata: last\n\ndata: [DONE]\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const requestBody = `{"model":"test","messages":[]}`
			var calls atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != requestBody || r.Method != http.MethodPost {
					t.Errorf("request: method=%s body=%q err=%v", r.Method, body, err)
				}
				w.Header().Set("Content-Type", tc.contentType)
				if r.ProtoMajor == 2 {
					w.(http.Flusher).Flush()
					panic(http.ErrAbortHandler)
				}
				if r.Proto != "HTTP/1.1" {
					t.Errorf("protocol = %s, want HTTP/1.1", r.Proto)
				}
				for _, part := range strings.SplitAfter(tc.response, "\n\n") {
					if part != "" {
						_, _ = io.WriteString(w, part)
						w.(http.Flusher).Flush()
					}
				}
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()

			client := tc.client()
			transport := client.Transport.(*http.Transport).Clone()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			transport.TLSClientConfig = &tls.Config{RootCAs: roots}
			defer transport.CloseIdleConnections()
			client.Transport = transport

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			for i := 0; i < 2; i++ {
				reused := false
				requestCtx := httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
					GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused },
				})
				request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, server.URL, strings.NewReader(requestBody))
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept", tc.contentType)
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || string(body) != tc.response {
					t.Fatalf("response: protocol=%s body=%q err=%v", response.Proto, body, err)
				}
				if response.StatusCode != http.StatusOK || response.Proto != "HTTP/1.1" {
					t.Fatalf("response: status=%d protocol=%s", response.StatusCode, response.Proto)
				}
				if i > 0 && !reused {
					t.Fatal("upstream connection was not reused")
				}
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("upstream calls = %d, want 2 without request replay", got)
			}
		})
	}
}

func TestStreamClientHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := newStreamClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(response.Body, first); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := io.ReadAll(response.Body); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestStreamClientDoesNotLimitResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		_, _ = io.WriteString(w, "data: first\n\n")
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(w, "data: second\n\n")
	}))
	defer server.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := newStreamClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != "data: first\n\ndata: second\n\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestClientWithTimeoutLimitsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(w, "}")
	}))
	defer server.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := clientWithTimeout(server.Client(), 10*time.Millisecond).Do(request)
	if response != nil {
		_, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if err == nil {
			err = readErr
		}
	}
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}
