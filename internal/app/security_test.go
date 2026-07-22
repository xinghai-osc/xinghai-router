package app

import (
	"net/http"
	"net/http/httptest"
	"regexp"
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

func TestCryptRejectsWrongKeyAndCorruptCiphertext(t *testing.T) {
	encoded, err := crypt("a sufficiently long test encryption key", "provider-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := crypt("a different long encryption key value", encoded, true); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
	if _, err := crypt("a sufficiently long test encryption key", "not-valid-ciphertext", true); err == nil {
		t.Fatal("expected corrupt ciphertext to fail")
	}
	if _, err := crypt("a sufficiently long test encryption key", "", true); err == nil {
		t.Fatal("expected empty ciphertext to fail")
	}
	empty, err := crypt("a sufficiently long test encryption key", "", false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := crypt("a sufficiently long test encryption key", empty, true)
	if err != nil || plain != "" {
		t.Fatalf("empty plaintext round-trip failed: %q %v", plain, err)
	}
}

func TestHashSecret(t *testing.T) {
	a := hashSecret("sk-xh-test")
	b := hashSecret("sk-xh-test")
	c := hashSecret("sk-xh-other")
	if a == "" || a != b {
		t.Fatal("hashSecret must be deterministic and non-empty")
	}
	if a == c {
		t.Fatal("different secrets must not collide")
	}
	if hashSecret("") == a {
		t.Fatal("empty secret must not match non-empty hash")
	}
}

func TestEqualSecret(t *testing.T) {
	if !equalSecret("abc", "abc") {
		t.Fatal("expected equal secrets to match")
	}
	if equalSecret("abc", "abd") || equalSecret("abc", "ab") || equalSecret("abc", "") {
		t.Fatal("expected unequal secrets to differ")
	}
}

func TestRandomID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 20 {
		id, err := randomID()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(id) {
			t.Fatalf("unexpected id format %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestRandomSecret(t *testing.T) {
	a, err := randomSecret("sk-xh-")
	if err != nil {
		t.Fatal(err)
	}
	b, err := randomSecret("sk-xh-")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "sk-xh-") || !strings.HasPrefix(b, "sk-xh-") {
		t.Fatalf("missing prefix: %q %q", a, b)
	}
	if a == b {
		t.Fatal("expected distinct secrets")
	}
	if len(a) <= len("sk-xh-") {
		t.Fatal("secret entropy missing")
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

func TestNewRateLimiterFallsBackWithoutRedis(t *testing.T) {
	l, mode := newRateLimiter("", 10)
	if mode != "memory" {
		t.Fatalf("mode = %q, want memory", mode)
	}
	if !l.allow("a") {
		t.Fatal("expected allow")
	}
	l.close()

	l, mode = newRateLimiter("redis://127.0.0.1:1/0", 10)
	if mode != "memory" {
		t.Fatalf("unreachable redis should fall back to memory, got %q", mode)
	}
	l.close()
}

func TestUsage(t *testing.T) {
	prompt, completion, total := usage([]byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
	if prompt != 2 || completion != 3 || total != 5 {
		t.Fatalf("got %d, %d, %d", prompt, completion, total)
	}
	prompt, completion, total = usage([]byte(`{"usage":{}}`))
	if prompt != 0 || completion != 0 || total != 0 {
		t.Fatalf("empty usage got %d, %d, %d", prompt, completion, total)
	}
	prompt, completion, total = usage([]byte(`not-json`))
	if prompt != 0 || completion != 0 || total != 0 {
		t.Fatalf("invalid json got %d, %d, %d", prompt, completion, total)
	}
	prompt, completion, total = usage([]byte(`{"model":"x"}`))
	if prompt != 0 || completion != 0 || total != 0 {
		t.Fatalf("missing usage got %d, %d, %d", prompt, completion, total)
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

func TestBearerAuthorization(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sk-from-auth")
	if got := bearer(req); got != "sk-from-auth" {
		t.Fatalf("bearer = %q, want sk-from-auth", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "sk-from-header")
	if got := bearer(req); got != "sk-from-header" {
		t.Fatalf("x-api-key bearer = %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sk-prefer")
	req.Header.Set("X-API-Key", "sk-other")
	if got := bearer(req); got != "sk-prefer" {
		t.Fatalf("authorization should win, got %q", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	if got := bearer(req); got != "" {
		t.Fatalf("empty headers should yield empty bearer, got %q", got)
	}
}

func TestHashEmailCodeAndGenerate(t *testing.T) {
	a := hashEmailCode(" User@Example.COM ", "123456")
	b := hashEmailCode("user@example.com", "123456")
	c := hashEmailCode("user@example.com", "000000")
	if a == "" || a != b {
		t.Fatal("hashEmailCode must normalize email case/space")
	}
	if a == c {
		t.Fatal("different codes must not collide")
	}
	code, err := generateEmailCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit in code %q", code)
		}
	}
}

func TestGeetestPayloadComplete(t *testing.T) {
	if (geetestPayload{}).complete() {
		t.Fatal("empty payload must be incomplete")
	}
	if !(geetestPayload{LotNumber: "a", CaptchaOutput: "b", PassToken: "c", GenTime: "d"}).complete() {
		t.Fatal("full payload must be complete")
	}
}

func TestValidSMTPPort(t *testing.T) {
	for _, port := range []string{"1", "25", "465", "587", "65535"} {
		if !validSMTPPort(port) {
			t.Fatalf("expected %q valid", port)
		}
	}
	for _, port := range []string{"", "0", "65536", "25a", "123456", "-1"} {
		if validSMTPPort(port) {
			t.Fatalf("expected %q invalid", port)
		}
	}
}
