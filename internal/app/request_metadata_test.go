package app

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRequestMetadataFromUA(t *testing.T) {
	browser, version, os, _, device, bot := requestMetadataFromUA("Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 Version/17.4 Mobile/15E148 Safari/604.1")
	if browser != "Safari" || version != "17.4" || os != "iOS" || device != "mobile" || bot {
		t.Fatalf("unexpected metadata: browser=%q version=%q os=%q device=%q bot=%v", browser, version, os, device, bot)
	}
	_, _, _, _, _, bot = requestMetadataFromUA("ExampleBot/1.0")
	if !bot {
		t.Fatal("expected bot user agent")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	nets, err := parseTrustedProxies("loopback, 10.0.0.0/8, 203.0.113.10")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) < 3 {
		t.Fatalf("expected multiple prefixes, got %d", len(nets))
	}
	if _, err := parseTrustedProxies("not-an-ip"); err == nil {
		t.Fatal("expected invalid proxy spec error")
	}
	empty, err := parseTrustedProxies("  ")
	if err != nil || empty != nil {
		t.Fatalf("empty = %v %v", empty, err)
	}
}

func TestClientIPIgnoresSpoofedHeadersWithoutTrustedProxy(t *testing.T) {
	if err := setTrustedProxies(""); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.20:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	req.Header.Set("X-Real-IP", "203.0.113.99")
	meta := requestMetadata(req)
	if meta.clientIP != "198.51.100.20" {
		t.Fatalf("clientIP = %q, want remote address without trusted proxy", meta.clientIP)
	}
}

func TestClientIPUsesHeadersFromTrustedProxy(t *testing.T) {
	if err := setTrustedProxies("loopback"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = setTrustedProxies("") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.2")
	meta := requestMetadata(req)
	if meta.clientIP != "203.0.113.50" {
		t.Fatalf("clientIP = %q, want first X-Forwarded-For hop", meta.clientIP)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:54322"
	req2.Header.Set("X-Real-IP", "198.51.100.7")
	meta2 := requestMetadata(req2)
	if meta2.clientIP != "198.51.100.7" {
		t.Fatalf("clientIP = %q, want X-Real-IP", meta2.clientIP)
	}
}

func TestIsTrustedProxyCIDR(t *testing.T) {
	nets := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if !isTrustedProxy("10.1.2.3:80", nets) {
		t.Fatal("expected 10.1.2.3 trusted")
	}
	if isTrustedProxy("198.51.100.1:80", nets) {
		t.Fatal("expected public address untrusted")
	}
}
