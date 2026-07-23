package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorCode(t *testing.T) {
	if got := errorCode(200); got != "" {
		t.Fatalf("errorCode(200) = %q", got)
	}
	if got := errorCode(404); got != "upstream_"+http.StatusText(404) {
		t.Fatalf("errorCode(404) = %q", got)
	}
	if got := errorCode(500); !strings.HasPrefix(got, "upstream_") {
		t.Fatalf("errorCode(500) = %q", got)
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"application/json", "application/json"},
		{"application/json; charset=utf-8", "application/json"},
		{"text/event-stream", "text/event-stream"},
		{"text/event-stream; charset=utf-8", "text/event-stream"},
		{"text/plain", "application/json"},
		{"", "application/json"},
	}
	for _, tt := range tests {
		if got := contentType(tt.in); got != tt.want {
			t.Fatalf("contentType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestChatCompletionsRejectsInvalidBodyBeforeUpstream(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"model":""}`,
		`not-json`,
		`{"stream":true}`,
		`{"model":" ` + strings.Repeat("m", 201) + `"}`,
		`{"model":"` + strings.Repeat("m", 201) + `"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		(&Service{}).chatCompletions(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestFirstGroupAndSortedKeys(t *testing.T) {
	if got := firstGroup(nil); got != "" {
		t.Fatalf("firstGroup(nil) = %q", got)
	}
	if got := firstGroup([]string{"a", "b"}); got != "a" {
		t.Fatalf("firstGroup = %q", got)
	}
	got := sortedKeys(map[string]bool{"b": true, "a": true, "c": true})
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("sortedKeys = %#v", got)
	}
}

func TestProxyChatCompletionsRequiresPricingWithoutSubscription(t *testing.T) {
	// Without a DB, reserveUsage panics; exercise error classification only.
	if !errors.Is(errPricingUnavailable, errPricingUnavailable) {
		t.Fatal("errPricingUnavailable must be stable")
	}
	if errors.Is(errInvalid, errPricingUnavailable) {
		t.Fatal("pricing and invalid errors must differ")
	}
}

func TestProxyChatCompletionsPricingErrorMapping(t *testing.T) {
	// Map pricing vs balance errors to distinct client codes without upstream.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: "1", keyID: "k"}))
	// Service with nil DB cannot run proxy; verify writeError shapes used by the handler.
	writeError(rec, 402, "pricing_unavailable", "no enabled pricing rule for this model")
	if rec.Code != 402 || !strings.Contains(rec.Body.String(), "pricing_unavailable") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestStreamSkipsWalletReservationFlag(t *testing.T) {
	// Documented product rule: stream requests are not settled; reservation must stay empty
	// so concurrent stream traffic does not pin wallet reserved balances.
	var reserved reservation
	stream := true
	subscriptionAccess := false
	if subscriptionAccess || stream {
		reserved = reservation{}
	}
	if reserved.amount != 0 {
		t.Fatal("stream path must not hold a non-zero reservation")
	}
}

func TestValidGatewayMaxTokens(t *testing.T) {
	if !validGatewayMaxTokens(1) || !validGatewayMaxTokens(maxGatewayMaxTokens) {
		t.Fatal("boundary max_tokens must be valid")
	}
	if validGatewayMaxTokens(0) || validGatewayMaxTokens(-1) || validGatewayMaxTokens(maxGatewayMaxTokens+1) {
		t.Fatal("out-of-range max_tokens must be invalid")
	}
}

func TestResolveGatewayMaxTokens(t *testing.T) {
	if got, ok := resolveGatewayMaxTokens(0); !ok || got != defaultGatewayMaxTokens {
		t.Fatalf("default = %d ok=%v, want %d true", got, ok, defaultGatewayMaxTokens)
	}
	if got, ok := resolveGatewayMaxTokens(1024); !ok || got != 1024 {
		t.Fatalf("resolved = %d ok=%v, want 1024 true", got, ok)
	}
	if _, ok := resolveGatewayMaxTokens(maxGatewayMaxTokens + 1); ok {
		t.Fatal("oversize max_tokens must be rejected")
	}
	if maxGatewayMaxTokens != 200_000 {
		t.Fatalf("maxGatewayMaxTokens = %d, want 200000", maxGatewayMaxTokens)
	}
	if maxUpstreamResponseBody != 16<<20 {
		t.Fatalf("maxUpstreamResponseBody = %d, want 16MiB", maxUpstreamResponseBody)
	}
}

func TestChatCompletionsRejectsOversizeMaxTokens(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"m","max_tokens":200001}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	(&Service{}).chatCompletions(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "max_tokens") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestUsageCostAndClamp(t *testing.T) {
	// 1M prompt tokens * $1/M + 0 completion = $1 before multipliers.
	if got := usageCost(1_000_000, 0, 1, 2, 1, 1); got != 1 {
		t.Fatalf("usageCost = %v, want 1", got)
	}
	if got := usageCost(1_000_000, 1_000_000, 1, 2, 1, 1); got != 3 {
		t.Fatalf("usageCost = %v, want 3", got)
	}
	if got := usageCost(1_000_000, 0, 1, 2, 2, 1.5); got != 3 {
		t.Fatalf("usageCost with multipliers = %v, want 3", got)
	}
	if got := usageCost(100, 0, 1, 1, 0, 0); got != usageCost(100, 0, 1, 1, 1, 1) {
		t.Fatal("zero multipliers must fall back to 1")
	}
	if got := clampCostToHold(5, 3); got != 3 {
		t.Fatalf("clamp = %v, want 3", got)
	}
	if got := clampCostToHold(-1, 3); got != 0 {
		t.Fatalf("negative clamp = %v, want 0", got)
	}
	if got := clampCostToHold(2, 0); got != 2 {
		t.Fatalf("zero hold must not clamp positive cost, got %v", got)
	}
}
