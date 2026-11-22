package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGatewayRequestScanParsesAllPricingFields(t *testing.T) {
	scan := scanGatewayRequest([]byte(`{"model":"m","stream":true,"max_tokens":128,"reasoning_effort":"high","service_tier":"priority"}`))
	if scan.Model != "m" || !scan.Stream || scan.MaxTokens != 128 {
		t.Fatalf("basic fields = %q %v %d", scan.Model, scan.Stream, scan.MaxTokens)
	}
	if scan.thinkingLevel() != "high" {
		t.Fatalf("thinkingLevel = %q, want high", scan.thinkingLevel())
	}
	if scan.serviceTier() != "priority" {
		t.Fatalf("serviceTier = %q, want priority", scan.serviceTier())
	}
	if got := requestThinkingLevel([]byte(`{"thinking":{"level":"Low"}}`)); got != "low" {
		t.Fatalf("requestThinkingLevel(thinking.level) = %q, want low", got)
	}
	if got := requestServiceTier([]byte(`{"service_tier":"auto"}`)); got != "" {
		t.Fatalf("requestServiceTier(auto) = %q, want empty", got)
	}
	// Invalid JSON keeps the existing helper contract: empty answers.
	if got := requestThinkingLevel([]byte("not json")); got != "" {
		t.Fatalf("requestThinkingLevel(invalid) = %q, want empty", got)
	}
	if got := requestServiceTier([]byte("not json")); got != "" {
		t.Fatalf("requestServiceTier(invalid) = %q, want empty", got)
	}
}

func TestIsGatewayPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/v1/chat/completions":         true,
		"/v1/messages":                 true,
		"/v1/responses":                true,
		"/v2/video_generation":         true,
		"/backend-api/codex/responses": true,
		"/me":                          true,
		"/me/keys":                     true,
		"/account/keys":                false,
		"/auth/login":                  false,
		"/admin/users":                 false,
		"/v1":                          false,
	} {
		if got := isGatewayPath(path); got != want {
			t.Fatalf("isGatewayPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRequestIDProvidesClientInfoOnGatewayPaths(t *testing.T) {
	var seen clientInfo
	var inner http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = clientInfoFromContext(r.Context())
		if requestID(r.Context()) == "" {
			t.Fatal("request id missing from context")
		}
	})
	handler := (&Service{}).requestID(inner)
	ua := strings.Repeat("a", 300)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("User-Agent", ua)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if seen.ip == "" || seen.userAgent == "" {
		t.Fatalf("gateway request lost client info: %+v", seen)
	}
	if len(seen.userAgent) > 256 {
		t.Fatalf("gateway client info keeps unbounded UA: %d", len(seen.userAgent))
	}
}

func TestMaintenanceModeEnabledWithoutDB(t *testing.T) {
	s := &Service{}
	if s.maintenanceModeEnabled(context.Background()) {
		t.Fatal("no database means maintenance is not enforced")
	}
}

func TestMaintenanceDataCacheRoundTrip(t *testing.T) {
	cache := newTTLCache[struct{}, bool](time.Minute)
	cache.store(struct{}{}, true)
	if v, ok := cache.lookup(struct{}{}); !ok || !v {
		t.Fatalf("lookup after store = %v %v", v, ok)
	}
	cache.clear()
	if _, ok := cache.lookup(struct{}{}); ok {
		t.Fatal("clear must drop the entry")
	}
}

func TestEvaluateContentPolicySkipsExtractionWhenNothingStored(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "test encryption key"}}
	snapshot := contentPolicySnapshot{Settings: contentPolicySettings{Mode: "off", StoreMode: "none"}}
	result := s.evaluateContentPolicy(snapshot, []byte(`{"messages":[{"role":"user","content":"anything"}]}`))
	if result.Decision != "allow" || result.ContentHash != "" || result.ContentLength != 0 {
		t.Fatalf("fast path result = %+v", result)
	}
}

func TestEvaluateContentPolicyPreNormalizedTerms(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "test encryption key"}}
	rule := contentPolicyRule{ID: "r1", Term: "Secret Phrase", Action: "block", Enabled: true, normalizedTerm: "secret phrase"}
	snapshot := contentPolicySnapshot{Settings: contentPolicySettings{Mode: "block", StoreMode: "none"}, Rules: []contentPolicyRule{rule}}
	result := s.evaluateContentPolicy(snapshot, []byte(`{"messages":[{"role":"user","content":"a SECRET PHRASE here"}]}`))
	if result.Decision != "block" {
		t.Fatalf("decision = %s, want block", result.Decision)
	}
}

func TestEvaluateContentPolicyDerivesMissingNormalizedTerm(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "test encryption key"}}
	// Directly constructed rules (tests) carry no pre-normalized term.
	rule := contentPolicyRule{ID: "r1", Term: "Secret Phrase", Action: "block", Enabled: true}
	snapshot := contentPolicySnapshot{Settings: contentPolicySettings{Mode: "block", StoreMode: "none"}, Rules: []contentPolicyRule{rule}}
	result := s.evaluateContentPolicy(snapshot, []byte(`{"messages":[{"role":"user","content":"a secret phrase here"}]}`))
	if result.Decision != "block" {
		t.Fatalf("decision = %s, want block", result.Decision)
	}
}

func TestValidScriptDigest(t *testing.T) {
	if !validScriptDigest("0f1e2d3c4b5a69788796a5b4c3d2e1f0a1b2c3d4") {
		t.Fatal("40-hex digest must be accepted")
	}
	for _, bad := range []string{"", "PONG", "0f1e2d3c4b5a69788796a5b4c3d2e1f0a1b2c3d", "0F1E2D3C4B5A69788796A5B4C3D2E1F0A1B2C3D4", "zz1e2d3c4b5a69788796a5b4c3d2e1f0a1b2c3d4"} {
		if validScriptDigest(bad) {
			t.Fatalf("digest %q must be rejected", bad)
		}
	}
}

func TestIsNoScriptError(t *testing.T) {
	if !isNoScriptError(errString("NOSCRIPT no script")) {
		t.Fatal("NOSCRIPT prefix must match")
	}
	if isNoScriptError(errString("WRONGTYPE something else")) {
		t.Fatal("other errors must not match")
	}
	if isNoScriptError(nil) {
		t.Fatal("nil must not match")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
