package app

import "testing"

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
