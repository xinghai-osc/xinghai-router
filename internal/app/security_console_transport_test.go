package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConsoleEarlyRejectionClosesUnfinishedBody(t *testing.T) {
	for _, route := range []struct {
		name    string
		path    string
		status  int
		marked  bool
		limited bool
	}{
		{"unauthenticated", "/account/keys", http.StatusUnauthorized, true, false},
		{"unrouted", "/admin/nonexistent", http.StatusNotFound, true, false},
		{"rate-limited", "/auth/login", http.StatusTooManyRequests, true, true},
		{"csrf", "/admin/channels", http.StatusForbidden, false, false},
	} {
		for _, body := range []struct {
			name    string
			headers string
			prefix  string
		}{
			{"chunked-no-data", "Transfer-Encoding: chunked\r\n", ""},
			{"chunked-partial-data", "Transfer-Encoding: chunked\r\n", "10\r\n{"},
			{"large-content-length", "Content-Length: 16777216\r\n", ""},
		} {
			t.Run(route.name+"/"+body.name, func(t *testing.T) {
				s := &Service{cfg: Config{RequestBodyTimeout: 30 * time.Second}, ipLimiter: newMemoryLimiter(1)}
				if route.limited && !s.ipLimiter.allow("127.0.0.1") {
					t.Fatal("could not consume the IP rate limit")
				}
				server := httptest.NewServer(s.routes())
				defer server.Close()
				conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 2*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
					t.Fatal(err)
				}
				marker := ""
				if route.marked {
					marker = "X-Xinghai-Request: 1\r\n"
				}
				if _, err := fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: test\r\n%s%s\r\n%s", route.path, body.headers, marker, body.prefix); err != nil {
					t.Fatal(err)
				}
				reader := bufio.NewReader(conn)
				response, err := http.ReadResponse(reader, nil)
				if err != nil {
					t.Fatalf("early rejection waited for the unfinished body: %v", err)
				}
				defer response.Body.Close()
				payload, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatalf("response did not finish: %v", err)
				}
				if response.StatusCode != route.status || !response.Close {
					t.Fatalf("status=%d want=%d close=%v body=%s", response.StatusCode, route.status, response.Close, payload)
				}
				if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
					t.Fatalf("connection remained open after rejection: %v", err)
				}
			})
		}
	}
}
