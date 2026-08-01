package app

import (
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
		if err := validOutboundURL(value); err != nil {
			t.Fatalf("validOutboundURL(%q) = %v", value, err)
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
		if err := validOutboundURL(value); err == nil {
			t.Fatalf("validOutboundURL(%q) expected error", value)
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

func TestValidateRedirectURL(t *testing.T) {
	allow := []string{
		"https://api.openai.com/v1/models",
		"http://127.0.0.1:11434/v1/models",
		"http://localhost:11434/v1/models",
		"http://evil.example.com/x",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/internal",
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

func TestValidUpstreamURL(t *testing.T) {
	for _, value := range []string{
		"https://api.openai.com",
		"http://api.example.com",
		"http://10.0.0.5:8080",
		"http://192.168.1.10/v1",
		"http://127.0.0.1:11434",
		"https://169.254.169.254/",
	} {
		if err := validUpstreamURL(value); err != nil {
			t.Fatalf("validUpstreamURL(%q) = %v", value, err)
		}
	}
	for _, value := range []string{
		"",
		"not-a-url",
		"ftp://example.com",
		"https://",
	} {
		if err := validUpstreamURL(value); err == nil {
			t.Fatalf("validUpstreamURL(%q) expected error", value)
		}
	}
}

func TestHTTPClientFollowsPlainHTTPRedirects(t *testing.T) {
	// Redirect to another loopback HTTP server should succeed; non-loopback
	// HTTP targets are covered by TestValidateRedirectURL (no network here).
	safeTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer safeTarget.Close()
	safeRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, safeTarget.URL, http.StatusFound)
	}))
	defer safeRedirect.Close()
	client := newHTTPClient(5 * time.Second)
	resp, err := client.Get(safeRedirect.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
