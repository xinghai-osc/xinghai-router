package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsoleBodyLimitsAtRealRoutes(t *testing.T) {
	for _, path := range []string{"/account/keys", "/admin/channels", "/admin/nonexistent"} {
		for _, length := range []int64{-1, 16 << 20} {
			n := 0
			req := httptest.NewRequest("POST", path, nil)
			req.Body = &countingReadCloser{Reader: io.LimitReader(endlessSecurityBody{}, 16<<20), read: &n}
			req.ContentLength = length
			req.Header.Set("X-Xinghai-Request", "1")
			rec := httptest.NewRecorder()
			(&Service{}).routes().ServeHTTP(rec, req)
			expected := http.StatusUnauthorized
			if path == "/admin/nonexistent" {
				expected = http.StatusNotFound
			}
			if rec.Code != expected || n != 0 {
				t.Fatalf("%s length=%d status=%d read=%d", path, length, rec.Code, n)
			}
		}
	}
	for _, length := range []int64{-1, defaultConsoleBodyMaxBytes + 100} {
		n := 0
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.Body = &countingReadCloser{Reader: io.LimitReader(endlessSecurityBody{}, 16<<20), read: &n}
		req.ContentLength = length
		req.Header.Set("X-Xinghai-Request", "1")
		rec := httptest.NewRecorder()
		(&Service{ipLimiter: newMemoryLimiter(10)}).routes().ServeHTTP(rec, req)
		if rec.Code != 413 || !strings.Contains(rec.Body.String(), "request_too_large") || n > defaultConsoleBodyMaxBytes+1 {
			t.Fatalf("length=%d status=%d read=%d body=%s", length, rec.Code, n, rec.Body)
		}
	}
}

type endlessSecurityBody struct{}

func (endlessSecurityBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestDecodeMalformedJSONIsNotSizeError(t *testing.T) {
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":`))
	var in map[string]any
	err := decode(req, &in)
	status, _, _ := requestBodyError(err)
	if err == nil || status != http.StatusBadRequest {
		t.Fatalf("error=%v status=%d", err, status)
	}
}
