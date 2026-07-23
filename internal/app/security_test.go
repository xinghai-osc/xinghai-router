package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCryptRoundTrip(t *testing.T) {
	encoded, err := crypt("a sufficiently long test encryption key", "provider-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "provider-secret") {
		t.Fatal("ciphertext contains plaintext")
	}
	decoded, err := crypt("a sufficiently long test encryption key", encoded, true)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "provider-secret" {
		t.Fatalf("got %q", decoded)
	}
}

func TestLimiter(t *testing.T) {
	l := newLimiter(2)
	if !l.allow("key") || !l.allow("key") || l.allow("key") {
		t.Fatal("unexpected rate-limit result")
	}
	l.entries["key"] = rateWindow{start: time.Now().Add(-time.Minute)}
	if !l.allow("key") {
		t.Fatal("window did not reset")
	}
}

func TestUsage(t *testing.T) {
	prompt, completion, total := usage([]byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
	if prompt != 2 || completion != 3 || total != 5 {
		t.Fatalf("got %d, %d, %d", prompt, completion, total)
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !passwordMatches(hash, "correct horse battery staple") || passwordMatches(hash, "incorrect password") {
		t.Fatal("unexpected password verification result")
	}
}

func TestValidAccountInput(t *testing.T) {
	if !validAccountInput("user@example.com", "Example User", "password1") {
		t.Fatal("expected valid account input")
	}
	for _, input := range []struct{ email, name, password string }{
		{"not-an-email", "Example User", "password1"},
		{"user@example.com", "", "password1"},
		{"user@example.com", "Example User", "short"},
	} {
		if validAccountInput(input.email, input.name, input.password) {
			t.Fatalf("expected invalid account input: %#v", input)
		}
	}
}

func TestDecodeDisallowUnknownFields(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	var ok sample
	if err := decode(req, &ok); err != nil {
		t.Fatalf("known fields must decode: %v", err)
	}
	if ok.Name != "ok" {
		t.Fatalf("name = %q", ok.Name)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok","extra":true}`))
	var bad sample
	if err := decode(req, &bad); err == nil {
		t.Fatal("unknown fields must be rejected")
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not-json`))
	if err := decode(req, &sample{}); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}
