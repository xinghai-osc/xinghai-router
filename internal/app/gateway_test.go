package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
