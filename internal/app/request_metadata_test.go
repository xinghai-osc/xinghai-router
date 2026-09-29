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
	if err := setTrustedProxies("loopback,10.0.0.2"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = setTrustedProxies("") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.2")
	meta := requestMetadata(req)
	if meta.clientIP != "203.0.113.50" {
		t.Fatalf("clientIP = %q, want client beyond trusted proxy chain", meta.clientIP)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:54322"
	req2.Header.Set("X-Real-IP", "198.51.100.7")
	meta2 := requestMetadata(req2)
	if meta2.clientIP != "198.51.100.7" {
		t.Fatalf("clientIP = %q, want X-Real-IP", meta2.clientIP)
	}
}

func TestClientIPTrustedChainBoundaries(t *testing.T) {
	tests := []struct {
		name, trusted, remote, realIP, want string
		forwarded                           []string
	}{
		{name: "untrusted peer", trusted: "loopback", remote: "198.51.100.2:123", forwarded: []string{"203.0.113.1"}, realIP: "203.0.113.2", want: "198.51.100.2"},
		{name: "spoofed left prefix", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"203.0.113.1, 198.51.100.2"}, want: "198.51.100.2"},
		{name: "untrusted intermediate", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"203.0.113.1, 10.0.0.2"}, want: "10.0.0.2"},
		{name: "trusted intermediates", trusted: "loopback,10.0.0.2", remote: "127.0.0.1:123", forwarded: []string{"203.0.113.1, 198.51.100.2, 10.0.0.2"}, want: "198.51.100.2"},
		{name: "real IP cannot override chain", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"198.51.100.2"}, realIP: "203.0.113.1", want: "198.51.100.2"},
		{name: "malformed rightmost", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"198.51.100.2, bad"}, realIP: "203.0.113.1", want: "127.0.0.1"},
		{name: "empty XFF prevents fallback", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{""}, realIP: "203.0.113.1", want: "127.0.0.1"},
		{name: "malformed prefix beyond boundary", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"bad, 198.51.100.2"}, want: "198.51.100.2"},
		{name: "multiple header fields", trusted: "loopback,10.0.0.2", remote: "127.0.0.1:123", forwarded: []string{"203.0.113.1, 198.51.100.2", "10.0.0.2"}, want: "198.51.100.2"},
		{name: "real IP fallback", trusted: "loopback", remote: "127.0.0.1:123", realIP: "198.51.100.2", want: "198.51.100.2"},
		{name: "mapped socket", trusted: "loopback", remote: "[::ffff:127.0.0.1]:123", forwarded: []string{"::ffff:198.51.100.2"}, want: "198.51.100.2"},
		{name: "mapped trusted range", trusted: "::ffff:10.0.0.0/104", remote: "10.0.0.2:123", forwarded: []string{"198.51.100.2"}, want: "198.51.100.2"},
		{name: "mapped trusted address", trusted: "::ffff:10.0.0.2", remote: "10.0.0.2:123", forwarded: []string{"198.51.100.2"}, want: "198.51.100.2"},
		{name: "ipv6 chain", trusted: "::1,2001:db8:1::/48", remote: "[::1]:123", forwarded: []string{"2001:db8:2::9, [2001:db8:1::2]"}, want: "2001:db8:2::9"},
		{name: "ipv6 canonical", remote: "[2001:0db8:0002::9]:123", want: "2001:db8:2::9"},
		{name: "header zone rejected", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"fe80::1%eth0"}, want: "127.0.0.1"},
		{name: "header port rejected", trusted: "loopback", remote: "127.0.0.1:123", forwarded: []string{"198.51.100.2:80"}, want: "127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nets, err := parseTrustedProxies(tt.trusted)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, value := range tt.forwarded {
				req.Header.Add("X-Forwarded-For", value)
			}
			req.Header.Set("X-Real-IP", tt.realIP)
			if got := clientIPFromRequest(req, nets); got != tt.want {
				t.Fatalf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIPRejectsOversizedForwardedChain(t *testing.T) {
	nets, err := parseTrustedProxies("loopback")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:123"
	req.Header.Add("X-Forwarded-For", "198.51.100.2")
	for range 32 {
		req.Header.Add("X-Forwarded-For", "127.0.0.1")
	}
	if got := clientIPFromRequest(req, nets); got != "127.0.0.1" {
		t.Fatalf("clientIP = %q, want socket peer", got)
	}
}

func TestTrustedProxiesRejectAmbiguousAddresses(t *testing.T) {
	for _, spec := range []string{"::ffff:10.0.0.0/80", "fe80::1%eth0", "198.51.100.2:80"} {
		if _, err := parseTrustedProxies(spec); err == nil {
			t.Fatalf("expected invalid proxy specification: %q", spec)
		}
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
