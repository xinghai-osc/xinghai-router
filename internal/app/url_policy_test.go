package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "127.0.0.2", "::1"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected %q to be loopback", host)
		}
	}
	for _, host := range []string{"", "example.com", "8.8.8.8", "0.0.0.0", "169.254.1.1", "10.0.0.1"} {
		if isLoopbackHost(host) {
			t.Fatalf("expected %q not to be loopback", host)
		}
	}
}

func TestValidPublicURL(t *testing.T) {
	for _, value := range []string{
		"https://pay.example.com",
		"https://pay.example.com/path",
		"https://8.8.8.8",
		"http://localhost",
		"http://localhost:3000",
		"http://127.0.0.1:8080",
		"http://[::1]:3000",
	} {
		if err := validPublicURL(value); err != nil {
			t.Fatalf("validPublicURL(%q) = %v", value, err)
		}
	}
	for _, value := range []string{
		"",
		"not-a-url",
		"ftp://example.com",
		"http://evil.example.com",
		"http://8.8.8.8",
		"https://",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/internal",
		"https://127.0.0.1/secret",
		"https://192.168.1.1/",
	} {
		if err := validPublicURL(value); err == nil {
			t.Fatalf("validPublicURL(%q) expected error", value)
		}
	}
}

func TestValidOutboundURL(t *testing.T) {
	for _, value := range []string{
		"https://api.openai.com",
		"http://127.0.0.1:11434",
		"http://localhost:11434",
	} {
		if err := validOutboundURL(value); err != nil {
			t.Fatalf("validOutboundURL(%q) = %v", value, err)
		}
	}
	for _, value := range []string{
		"https://169.254.169.254/",
		"https://10.1.2.3/",
		"https://172.16.0.1/",
		"https://[::1]/",
		"http://example.com",
	} {
		if err := validOutboundURL(value); err == nil {
			t.Fatalf("validOutboundURL(%q) expected error", value)
		}
	}
}

func TestIsNonPublicIP(t *testing.T) {
	nonPublic := []string{
		"127.0.0.1",
		"127.0.0.2",
		"::1",
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
		"169.254.169.254",
		"169.254.1.1",
		"0.0.0.0",
		"::",
		"224.0.0.1",
		"ff02::1",
		"fe80::1",
		"fc00::1",
		"fd12:3456:789a::1",
	}
	for _, s := range nonPublic {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("ParseIP(%q) = nil", s)
		}
		if !isNonPublicIP(ip) {
			t.Fatalf("expected %q to be non-public", s)
		}
	}
	public := []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888"}
	for _, s := range public {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("ParseIP(%q) = nil", s)
		}
		if isNonPublicIP(ip) {
			t.Fatalf("expected %q to be public", s)
		}
	}
	if !isNonPublicIP(nil) {
		t.Fatal("nil IP should be non-public")
	}
}

func TestAllowDialIP(t *testing.T) {
	type caseT struct {
		host string
		ip   string
		ok   bool
	}
	cases := []caseT{
		{"api.openai.com", "8.8.8.8", true},
		{"api.openai.com", "1.1.1.1", true},
		{"api.openai.com", "10.0.0.1", false},
		{"api.openai.com", "192.168.1.1", false},
		{"api.openai.com", "169.254.169.254", false},
		{"api.openai.com", "127.0.0.1", false},
		{"api.openai.com", "::1", false},
		{"api.openai.com", "0.0.0.0", false},
		{"api.openai.com", "fc00::1", false},
		{"evil.example.com", "169.254.169.254", false},
		{"evil.example.com", "10.1.2.3", false},
		{"evil.example.com", "127.0.0.1", false},
		{"localhost", "127.0.0.1", true},
		{"LOCALHOST", "127.0.0.1", true},
		{"localhost", "::1", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.2", "127.0.0.2", true},
		{"::1", "::1", true},
		{"localhost", "10.0.0.1", false},
		{"localhost", "169.254.169.254", false},
		{"localhost", "8.8.8.8", true},
		{"8.8.8.8", "8.8.8.8", true},
		{"10.0.0.1", "10.0.0.1", false},
		{"169.254.169.254", "169.254.169.254", false},
		{"", "8.8.8.8", true},
		{"", "127.0.0.1", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("ParseIP(%q) = nil", c.ip)
		}
		got := allowDialIP(c.host, ip)
		if got != c.ok {
			t.Fatalf("allowDialIP(%q, %q) = %v, want %v", c.host, c.ip, got, c.ok)
		}
	}
	if allowDialIP("example.com", nil) {
		t.Fatal("nil IP must be denied")
	}
}

func TestValidateRedirectURL(t *testing.T) {
	allow := []string{
		"https://api.openai.com/v1/models",
		"http://127.0.0.1:11434/v1/models",
		"http://localhost:11434/v1/models",
	}
	for _, value := range allow {
		u, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateRedirectURL(u); err != nil {
			t.Fatalf("validateRedirectURL(%q) = %v", value, err)
		}
	}
	deny := []string{
		"http://evil.example.com/x",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/internal",
		"https://127.0.0.1/secret",
		"ftp://example.com/x",
		"https://",
	}
	for _, value := range deny {
		u, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateRedirectURL(u); err == nil {
			t.Fatalf("validateRedirectURL(%q) expected error", value)
		}
	}
}

func TestHTTPClientBlocksUnsafeRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()
	// final.URL is http://127.0.0.1:port — loopback HTTP is allowed.
	// Build an intermediate that redirects to a non-loopback HTTP host.
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/steal", http.StatusFound)
	}))
	defer evil.Close()

	client := newHTTPClient(5 * time.Second)
	resp, err := client.Get(evil.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected redirect to non-loopback HTTP to fail")
	}

	// Public HTTPS redirect target simulation: use loopback HTTPS is hard without certs.
	// Safe path: redirect to another loopback HTTP server should succeed.
	safeTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer safeTarget.Close()
	safeRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, safeTarget.URL, http.StatusFound)
	}))
	defer safeRedirect.Close()
	resp, err = client.Get(safeRedirect.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
