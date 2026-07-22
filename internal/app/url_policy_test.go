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
	} {
		if err := validPublicURL(value); err == nil {
			t.Fatalf("validPublicURL(%q) expected error", value)
		}
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
