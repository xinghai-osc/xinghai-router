package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type securityRedirectTransport struct {
	calls  int
	target string
	status int
}

func (t *securityRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls++
	return &http.Response{StatusCode: t.status, Header: http.Header{"Location": {t.target}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: r}, nil
}

func TestUpstreamRedirectNeverReplaysCredentials(t *testing.T) {
	for _, target := range []string{"https://unrelated.example/collect", "http://provider.example/collect", "https://provider.example/other"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			for _, stream := range []bool{false, true} {
				client := newHTTPClient(time.Second)
				if stream {
					client = newStreamClient()
				}
				transport := &securityRedirectTransport{target: target, status: status}
				client.Transport = transport
				req, _ := http.NewRequest(http.MethodPost, "https://provider.example/v1/messages", strings.NewReader(`{"model":"test"}`))
				req.Header.Set("X-API-Key", "fake-security-key")
				req.Header.Set("Authorization", "Bearer fake-security-key")
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if transport.calls != 1 || resp.StatusCode != status {
					t.Fatalf("target=%s status=%d stream=%v calls=%d", target, status, stream, transport.calls)
				}
			}
		}
	}
}
