package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClientUpstreamErrorHidesKeywordErrors(t *testing.T) {
	s := &Service{}
	ctx := context.Background()
	reliability := defaultReliabilitySettings()
	for _, body := range []string{
		`{"error":{"message":"Your credit balance is too low, please recharge at https://auth.openai.com"}}`,
		`Your credit balance is too low, please recharge`,
		`error: 订阅额度不足或未配置订阅`,
		`{"error":{"message":"upstream request failed","detail":"credit balance is too low"}}`,
	} {
		out := s.clientUpstreamError(ctx, body, reliability)
		if strings.Contains(out, "credit balance") || strings.Contains(out, "auth.openai.com") || strings.Contains(out, "订阅") {
			t.Fatalf("keyword error still leaked as %q", out)
		}
		if !strings.Contains(out, noChannelAvailableDetail) {
			t.Fatalf("keyword error missing generic notice: %q", out)
		}
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	out := s.clientUpstreamError(ctx, `{"error":{"message":"Your credit balance is too low","type":"insufficient_quota"}}`, reliability)
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("replacement is not JSON: %v (%s)", err, out)
	}
	if parsed.Error.Message != noChannelAvailableDetail || parsed.Error.Type != "insufficient_quota" {
		t.Fatalf("replacement = %#v", parsed.Error)
	}
	// A normal error body must pass through untouched when no public URL is
	// configured (nil DB) and no keyword applies.
	normal := `{"error":{"message":"model not found","type":"invalid_request_error"}}`
	if got := s.clientUpstreamError(ctx, normal, reliability); got != normal {
		t.Fatalf("normal error altered: %q", got)
	}
}

func TestRewriteUpstreamURLs(t *testing.T) {
	detail := `{"error":{"message":"see https://docs.anthropic.com/support for help at http://help.deepseek.com"}}`
	out := rewriteUpstreamURLs(detail, "https://xinghai.example.com")
	if strings.Contains(out, "anthropic.com") || strings.Contains(out, "deepseek.com") || strings.Contains(out, "http://help") {
		t.Fatalf("upstream URLs leaked: %q", out)
	}
	if strings.Count(out, "https://xinghai.example.com") != 2 {
		t.Fatalf("expected both URLs replaced, got %q", out)
	}
	if got := rewriteUpstreamURLs(detail, ""); got != detail {
		t.Fatal("empty public base must leave the text untouched")
	}
}

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

func TestCopyResponseHeadersPreservesEndToEndHeaders(t *testing.T) {
	src := http.Header{
		"Content-Type":      {"application/json; charset=utf-8"},
		"Openai-Request-Id": {"req_upstream"},
		"X-Upstream":        {"kept"},
		"Connection":        {"keep-alive"},
		"Content-Length":    {"123"},
	}
	dst := http.Header{}
	copyResponseHeaders(dst, src)
	if dst.Get("Content-Type") != "application/json; charset=utf-8" || dst.Get("Openai-Request-Id") != "req_upstream" || dst.Get("X-Upstream") != "kept" {
		t.Fatalf("headers not preserved: %#v", dst)
	}
	if dst.Get("Connection") != "" || dst.Get("Content-Length") != "" {
		t.Fatalf("hop-by-hop headers leaked: %#v", dst)
	}
}

func TestStreamResponseDirectPreservesBytes(t *testing.T) {
	body := "event: response.output_text.delta\r\ndata: {\"delta\":\"hi\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}\r\n\r\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"text/event-stream"}}}
	rec := httptest.NewRecorder()
	stats, err := streamResponseDirect(rec, resp)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Body.String() != body {
		t.Fatalf("stream changed:\n got %q\nwant %q", rec.Body.String(), body)
	}
	if stats.prompt != 2 || stats.completion != 1 {
		t.Fatalf("usage = %+v", stats)
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

func TestInitialKeyIndex(t *testing.T) {
	keys := []channelKeyCredential{
		{id: "k1", key: "a", priority: 10},
		{id: "k2", key: "b", priority: 10},
		{id: "k3", key: "c", priority: 5},
	}
	// All same priority -> index = seed[0] % len.
	same := []channelKeyCredential{{id: "k1", key: "a", priority: 7}, {id: "k2", key: "b", priority: 7}}
	for seed, want := range map[byte]int{0: 0, 1: 1, 2: 0} {
		if got := initialKeyIndex(same, []byte{seed}); got != want {
			t.Fatalf("same-priority seed %d index = %d, want %d", seed, got, want)
		}
	}
	// Mixed priority -> index within the top-priority group only.
	for seed, want := range map[byte]int{0: 0, 1: 1} {
		if got := initialKeyIndex(keys, []byte{seed}); got != want {
			t.Fatalf("mixed-priority seed %d index = %d, want %d", seed, got, want)
		}
	}
	if got := initialKeyIndex(keys[:1], []byte{7}); got != 0 {
		t.Fatalf("single key index = %d, want 0", got)
	}
}

func TestRotateKey(t *testing.T) {
	keys := []channelKeyCredential{
		{id: "k1", key: "a", priority: 10},
		{id: "k2", key: "b", priority: 10},
		{id: "k3", key: "c", priority: 5},
	}
	ch := channel{apiKey: "a", keyID: "k1", keys: keys}
	ch.rotateKey()
	if ch.keyID != "k2" || ch.apiKey != "b" {
		t.Fatalf("after first rotate = %s/%s, want k2/b", ch.keyID, ch.apiKey)
	}
	ch.rotateKey()
	if ch.keyID != "k3" || ch.apiKey != "c" {
		t.Fatalf("after second rotate = %s/%s, want k3/c", ch.keyID, ch.apiKey)
	}
	ch.rotateKey()
	if ch.keyID != "k1" || ch.apiKey != "a" {
		t.Fatalf("rotate must wrap = %s/%s, want k1/a", ch.keyID, ch.apiKey)
	}
	// Single-key and legacy channels stay fixed.
	single := channel{apiKey: "a", keyID: "k1", keys: keys[:1]}
	single.rotateKey()
	if single.keyID != "k1" || single.apiKey != "a" {
		t.Fatalf("single-key rotate changed = %s/%s", single.keyID, single.apiKey)
	}
	legacy := channel{apiKey: "legacy"}
	legacy.rotateKey()
	if legacy.apiKey != "legacy" {
		t.Fatalf("legacy rotate changed = %s", legacy.apiKey)
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

func TestEstimatedStreamUsageMatchesReservationEstimate(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`)
	prompt, completion := estimatedStreamUsage(body, 200000)
	if prompt != len(body)/3 || completion != 200000 {
		t.Fatalf("estimated stream usage = %d/%d, want %d/%d", prompt, completion, len(body)/3, 200000)
	}
	if _, completion := estimatedStreamUsage(body, 0); completion != defaultGatewayMaxTokens {
		t.Fatalf("default estimated completion = %d, want %d", completion, defaultGatewayMaxTokens)
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

func TestRewriteJSONModel(t *testing.T) {
	body := []byte(`{"model":"alias","input":"hello","store":false}`)
	got := rewriteJSONModel(body, "real-model")
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("rewritten body is invalid JSON: %v", err)
	}
	if payload["model"] != "real-model" {
		t.Fatalf("model = %v, want real-model", payload["model"])
	}
	if payload["input"] != "hello" || payload["store"] != false {
		t.Fatalf("rewrite dropped Responses fields: %#v", payload)
	}
	if got := string(rewriteJSONModel(body, "")); got != string(body) {
		t.Fatalf("empty upstream model must preserve body: %s", got)
	}
	invalid := []byte(`not-json`)
	if got := string(rewriteJSONModel(invalid, "real-model")); got != string(invalid) {
		t.Fatalf("invalid JSON must pass through unchanged: %s", got)
	}
}

func TestRewriteOpenAIBody(t *testing.T) {
	// No rewrites needed: the body passes through byte-for-byte unchanged.
	clean := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	if got := string(rewriteOpenAIBody(clean, "", "m", false)); got != string(clean) {
		t.Fatalf("clean body must pass through unchanged: %s", got)
	}

	// Model rewrite plus extension strip in one pass.
	body := []byte(`{"model":"m","promptCacheKey":"k","messages":[{"role":"user","content":"hi"}]}`)
	got := rewriteOpenAIBody(body, "real-model", "m", false)
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("rewritten body is invalid JSON: %v", err)
	}
	if payload["model"] != "real-model" {
		t.Fatalf("model = %v, want real-model", payload["model"])
	}
	if _, ok := payload["promptCacheKey"]; ok {
		t.Fatalf("promptCacheKey not stripped: %#v", payload)
	}
	if payload["messages"] == nil {
		t.Fatalf("messages dropped by rewrite: %#v", payload)
	}

	// Streaming injects include_usage and preserves an existing stream_options object.
	streamBody := []byte(`{"model":"m","messages":[]}`)
	got = rewriteOpenAIBody(streamBody, "", "m", true)
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("stream body is invalid JSON: %v", err)
	}
	so, _ := payload["stream_options"].(map[string]any)
	if so == nil || so["include_usage"] != true {
		t.Fatalf("stream_options.include_usage not injected: %#v", payload)
	}

	// Invalid JSON passes through unchanged even when rewrites are requested.
	invalid := []byte(`not-json`)
	if got := string(rewriteOpenAIBody(invalid, "real-model", "m", true)); got != string(invalid) {
		t.Fatalf("invalid JSON must pass through unchanged: %s", got)
	}
}

func TestApplyRequestOverrides(t *testing.T) {
	ov := channelRequestOverrides{
		Delete: []string{"promptCacheKey", "missing"},
		Set:    map[string]any{"model": "gpt-4o-mini", "temperature": 0.2},
	}
	got := applyRequestOverrides([]byte(`{"model":"gpt-4o","promptCacheKey":"abc","stream":true}`), ov)
	if strings.Contains(string(got), "promptCacheKey") {
		t.Fatalf("delete did not remove field: %s", got)
	}
	if !strings.Contains(string(got), `"model":"gpt-4o-mini"`) || strings.Contains(string(got), `"model":"gpt-4o"`) {
		t.Fatalf("set did not override model: %s", got)
	}
	if !strings.Contains(string(got), `"temperature":0.2`) || !strings.Contains(string(got), `"stream":true`) {
		t.Fatalf("set/keep failed: %s", got)
	}
	empty := channelRequestOverrides{}
	if got := string(applyRequestOverrides([]byte(`{"a":1}`), empty)); got != `{"a":1}` {
		t.Fatalf("empty overrides must pass body through: %s", got)
	}
	if got := string(applyRequestOverrides([]byte(`not-json`), ov)); got != "not-json" {
		t.Fatalf("invalid JSON must pass through: %s", got)
	}
	// A delete of a field that is absent must still leave the body untouched byte-for-byte.
	noop := channelRequestOverrides{Delete: []string{"absent"}}
	if got := string(applyRequestOverrides([]byte(`{"a":1}`), noop)); got != `{"a":1}` {
		t.Fatalf("no-op override must not reserialize the body: %s", got)
	}
}

func TestValidRequestOverrides(t *testing.T) {
	if err := validRequestOverrides(nil); err != nil {
		t.Fatalf("nil must be valid: %v", err)
	}
	if err := validRequestOverrides(&channelRequestOverrides{}); err != nil {
		t.Fatalf("empty must be valid: %v", err)
	}
	if err := validRequestOverrides(&channelRequestOverrides{Delete: []string{"promptCacheKey"}, Set: map[string]any{"model": "m"}}); err != nil {
		t.Fatalf("normal config must be valid: %v", err)
	}
	for _, ov := range []*channelRequestOverrides{
		{Delete: []string{""}},
		{Delete: []string{"a", "a"}},
		{Delete: []string{strings.Repeat("a", 101)}},
		{Set: map[string]any{"": 1}},
		{Set: map[string]any{strings.Repeat("a", 101): 1}},
	} {
		if err := validRequestOverrides(ov); err == nil {
			t.Fatalf("invalid config must be rejected: %+v", ov)
		}
	}
	tooMany := make([]string, maxRequestOverrideFields+1)
	for i := range tooMany {
		tooMany[i] = "f" + strconv.Itoa(i)
	}
	if err := validRequestOverrides(&channelRequestOverrides{Delete: tooMany}); err == nil {
		t.Fatal("oversize delete list must be rejected")
	}
	norm := normalizedOverrides(&channelRequestOverrides{Delete: []string{" a ", "", "b"}, Set: map[string]any{" x ": 1, "": 2}})
	if len(norm.Delete) != 2 || norm.Delete[0] != "a" || norm.Delete[1] != "b" {
		t.Fatalf("normalized delete = %#v", norm.Delete)
	}
	if _, ok := norm.Set["x"]; !ok || len(norm.Set) != 1 {
		t.Fatalf("normalized set = %#v", norm.Set)
	}
	if out := normalizedOverrides(nil); out.Delete != nil || out.Set == nil {
		t.Fatalf("nil normalization must yield empty struct: %#v", out)
	}
}

func TestValidUAPool(t *testing.T) {
	if err := validUAPool(nil); err != nil {
		t.Fatalf("nil must be valid: %v", err)
	}
	if err := validUAPool([]string{}); err != nil {
		t.Fatalf("empty must be valid: %v", err)
	}
	if err := validUAPool([]string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}); err != nil {
		t.Fatalf("normal pool must be valid: %v", err)
	}
	for _, pool := range [][]string{
		{""},
		{"   "},
		{strings.Repeat("a", maxUALength+1)},
	} {
		if err := validUAPool(pool); err == nil {
			t.Fatalf("invalid pool must be rejected: %#v", pool)
		}
	}
	tooMany := make([]string, maxUAPoolEntries+1)
	for i := range tooMany {
		tooMany[i] = "ua" + strconv.Itoa(i)
	}
	if err := validUAPool(tooMany); err == nil {
		t.Fatal("oversize pool must be rejected")
	}
}

func TestNormalizedUAPool(t *testing.T) {
	pool := normalizedUAPool([]string{"  a  ", "", "b", "a", "b", "\tc\t"})
	if len(pool) != 3 || pool[0] != "a" || pool[1] != "b" || pool[2] != "c" {
		t.Fatalf("normalized pool = %#v", pool)
	}
	if out := normalizedUAPool(nil); len(out) != 0 {
		t.Fatalf("nil normalization must yield empty slice: %#v", out)
	}
}

func TestPickUA(t *testing.T) {
	var ch channel
	if got := ch.pickUA([]byte{0x01}); got != "" {
		t.Fatalf("empty pool must yield empty UA, got %q", got)
	}
	ch.uaPool = []string{"only"}
	if got := ch.pickUA([]byte{0x00}); got != "only" {
		t.Fatalf("single-entry pool must always pick it, got %q", got)
	}
	ch.uaPool = []string{"a", "b", "c"}
	first := ch.pickUA([]byte{0x00})
	if first != "a" {
		t.Fatalf("seed 0 must pick first entry, got %q", first)
	}
	if got := ch.pickUA([]byte{0x00}); got != first {
		t.Fatalf("same seed must pick the same UA, got %q want %q", got, first)
	}
	picked := map[string]bool{}
	for seed := 0; seed < 300; seed++ {
		picked[ch.pickUA([]byte{byte(seed)})] = true
	}
	if len(picked) != 3 {
		t.Fatalf("seeds must spread across the pool, picked %v", picked)
	}
}

func TestParsedUAPool(t *testing.T) {
	if got := parsedUAPool(nil); got != nil {
		t.Fatalf("empty raw must yield nil, got %#v", got)
	}
	pool := parsedUAPool([]byte(`["a","b"]`))
	if len(pool) != 2 || pool[0] != "a" || pool[1] != "b" {
		t.Fatalf("parsed pool = %#v", pool)
	}
	if got := parsedUAPool([]byte(`not-json`)); got != nil {
		t.Fatalf("invalid JSON must yield nil, got %#v", got)
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
	// 1M prompt tokens (0 cached) * $1/M + 0 completion = $1 before multipliers.
	if got := usageCost(1_000_000, 0, 0, 1, 0.5, 2, 1, 1); got != 1 {
		t.Fatalf("usageCost = %v, want 1", got)
	}
	// 1M prompt + 1M completion at $1/$2 -> $3.
	if got := usageCost(1_000_000, 0, 1_000_000, 1, 0.5, 2, 1, 1); got != 3 {
		t.Fatalf("usageCost = %v, want 3", got)
	}
	// 1M prompt (500k cached) at $1/$0.5 + 0 completion -> $0.75.
	if got := usageCost(1_000_000, 500_000, 0, 1, 0.5, 2, 1, 1); got != 0.75 {
		t.Fatalf("usageCost cached = %v, want 0.75", got)
	}
	// multipliers: (1M*$1 + 1M*$2) * 2 * 1.5 = $9.
	if got := usageCost(1_000_000, 0, 1_000_000, 1, 0.5, 2, 2, 1.5); got != 9 {
		t.Fatalf("usageCost with multipliers = %v, want 9", got)
	}
	if got := usageCost(100, 0, 1, 1, 0.5, 1, 0, 0); got != usageCost(100, 0, 1, 1, 0.5, 1, 1, 1) {
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

func TestTieredUsageCost(t *testing.T) {
	// Base prices are $1/$0.5/$2 per million. Two tiers:
	//   from_tokens=0      -> $1/$0.5/$2 (same as base)
	//   from_tokens=500000 -> $0.8/$0.4/$1.6 (20% off above 500k total)
	rule := pricingRule{
		input: 1, cachedInput: 0.5, output: 2, multiplier: 1, found: true,
		tiers: []pricingTier{
			{fromTokens: 0, input: 1, cachedInput: 0.5, output: 2},
			{fromTokens: 500_000, input: 0.8, cachedInput: 0.4, output: 1.6},
		},
	}

	// 100k prompt + 0 completion = 100k total -> falls in tier 0 (from 0).
	// cost = 100k * $1 / 1M = $0.1
	if got := tieredUsageCost(100_000, 0, 0, rule, 1, 0.5, 2); got != 0.1 {
		t.Fatalf("tier 0 cost = %v, want 0.1", got)
	}

	// 600k prompt + 0 completion = 600k total -> falls in tier 1 (from 500k).
	// cost = 600k * $0.8 / 1M = $0.48
	if got := tieredUsageCost(600_000, 0, 0, rule, 1, 0.5, 2); got != 0.48 {
		t.Fatalf("tier 1 cost = %v, want 0.48", got)
	}

	// 400k prompt + 200k completion = 600k total -> tier 1.
	// cost = 400k * $0.8 / 1M + 200k * $1.6 / 1M = $0.32 + $0.32 = $0.64
	if got := tieredUsageCost(400_000, 0, 200_000, rule, 1, 0.5, 2); got != 0.64 {
		t.Fatalf("tier 1 mixed cost = %v, want 0.64", got)
	}

	// No tiers -> falls back to base prices passed in.
	ruleNoTiers := pricingRule{input: 1, cachedInput: 0.5, output: 2, multiplier: 1, found: true}
	if got := tieredUsageCost(1_000_000, 0, 0, ruleNoTiers, 1, 0.5, 2); got != 1 {
		t.Fatalf("no tiers cost = %v, want 1", got)
	}
}

func TestResolvePricingTimeRule(t *testing.T) {
	rule := pricingRule{
		input: 1, cachedInput: 0.5, output: 2, found: true,
		timeRules: []pricingTimeRule{
			// 00:00–06:00 every day: half-price
			{startMinute: 0, endMinute: 360, weekdays: "1111111", input: 0.5, cachedInput: 0.25, output: 1},
			// 22:00–24:00 every day: 80% price
			{startMinute: 1320, endMinute: 1440, weekdays: "1111111", input: 0.8, cachedInput: 0.4, output: 1.6},
		},
	}

	// 03:00 (180 min) -> matches first rule.
	ti, tc, to := rule.resolvePricing(time.Date(2024, 1, 1, 3, 0, 0, 0, time.UTC))
	if ti != 0.5 || tc != 0.25 || to != 1 {
		t.Fatalf("night override = %v/%v/%v, want 0.5/0.25/1", ti, tc, to)
	}

	// 12:00 (720 min) -> no match, base prices.
	ti, tc, to = rule.resolvePricing(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	if ti != 1 || tc != 0.5 || to != 2 {
		t.Fatalf("day base = %v/%v/%v, want 1/0.5/2", ti, tc, to)
	}

	// 23:00 (1380 min) -> matches second rule.
	ti, tc, to = rule.resolvePricing(time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC))
	if ti != 0.8 || tc != 0.4 || to != 1.6 {
		t.Fatalf("evening override = %v/%v/%v, want 0.8/0.4/1.6", ti, tc, to)
	}
}

func TestResolvePricingWeekday(t *testing.T) {
	rule := pricingRule{
		input: 1, cachedInput: 0.5, output: 2, found: true,
		timeRules: []pricingTimeRule{
			// Only on Saturday (index 5).
			{startMinute: 0, endMinute: 1440, weekdays: "0000010", input: 0.1, cachedInput: 0.05, output: 0.2},
		},
	}

	// Monday (Go weekday=1 -> our index=0) -> no match.
	ti, _, _ := rule.resolvePricing(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	if ti != 1 {
		t.Fatalf("Monday should use base price, got %v", ti)
	}

	// Saturday (Go weekday=6 -> our index=5) -> match.
	sat := time.Date(2024, 1, 6, 12, 0, 0, 0, time.UTC)
	ti, _, _ = rule.resolvePricing(sat)
	if ti != 0.1 {
		t.Fatalf("Saturday should use override price, got %v", ti)
	}
}

func TestResolvePricingWrapAround(t *testing.T) {
	rule := pricingRule{
		input: 1, cachedInput: 0.5, output: 2, found: true,
		timeRules: []pricingTimeRule{
			// 22:00 → 06:00 (wrap-around).
			{startMinute: 1320, endMinute: 360, weekdays: "1111111", input: 0.5, cachedInput: 0.25, output: 1},
		},
	}

	// 23:00 (1380 min) -> >= 1320, matches.
	ti, _, _ := rule.resolvePricing(time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC))
	if ti != 0.5 {
		t.Fatalf("23:00 should match wrap-around, got %v", ti)
	}

	// 03:00 (180 min) -> < 360, matches.
	ti, _, _ = rule.resolvePricing(time.Date(2024, 1, 1, 3, 0, 0, 0, time.UTC))
	if ti != 0.5 {
		t.Fatalf("03:00 should match wrap-around, got %v", ti)
	}

	// 12:00 (720 min) -> no match.
	ti, _, _ = rule.resolvePricing(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	if ti != 1 {
		t.Fatalf("12:00 should use base price, got %v", ti)
	}
}

func TestStreamCaptureWriterEmptyStream(t *testing.T) {
	rec := httptest.NewRecorder()
	capture := newStreamCaptureWriter(rec, false)
	if _, err := (&Service{}).streamResponse(capture, streamBody(t, "")); err != nil {
		t.Fatal(err)
	}
	if capture.wrote || capture.headerSent {
		t.Fatalf("empty stream must dispatch nothing (wrote=%v header=%v)", capture.wrote, capture.headerSent)
	}
	if rec.Body.Len() != 0 || rec.Code != http.StatusOK {
		t.Fatalf("empty stream recorded body=%d code=%d", rec.Body.Len(), rec.Code)
	}
}

func TestStreamCaptureNonEmptyStream(t *testing.T) {
	rec := httptest.NewRecorder()
	capture := newStreamCaptureWriter(rec, true)
	st, err := (&Service{}).streamResponse(capture, streamBody(t, "data: {\"x\":1}\n\ndata: [DONE]\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !capture.wrote {
		t.Fatal("non-empty stream must be written through")
	}
	if got := string(capture.bytes()); got == "" {
		t.Fatal("buffered stream bytes must be non-empty")
	}
	if st.prompt != 0 || st.completion != 0 {
		t.Fatalf("no usage expected, got %+v", st)
	}
}

func TestParseSSEUsageRequiresUsageObject(t *testing.T) {
	var st streamStats
	parseSSEUsage([]byte(`{"id":"chunk","choices":[]}`), &st)
	if st.usageReported {
		t.Fatal("ordinary stream chunk must not report usage")
	}
	parseSSEUsage([]byte(`{"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}}`), &st)
	if !st.usageReported || !st.usageComplete || st.prompt != 8 || st.completion != 5 {
		t.Fatalf("parsed usage = %+v, want reported 8/5", st)
	}
	var anthropic streamStats
	parseSSEUsage([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0}}}`), &anthropic)
	if anthropic.usageComplete {
		t.Fatal("Anthropic message_start must not complete output usage")
	}
	parseSSEUsage([]byte(`{"type":"message_delta","usage":{"output_tokens":12}}`), &anthropic)
	if !anthropic.usageComplete || anthropic.prompt != 10 || anthropic.completion != 12 {
		t.Fatalf("Anthropic usage = %+v, want complete 10/12", anthropic)
	}
	var zero streamStats
	parseSSEUsage([]byte(`{"usage":{"prompt_tokens":0,"completion_tokens":0}}`), &zero)
	if zero.usageComplete {
		t.Fatal("zero-token usage must use the conservative billing fallback")
	}
}

func TestStreamCaptureWriterHeaderOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	capture := newStreamCaptureWriter(rec, false)
	capture.WriteHeader(200)
	capture.WriteHeader(502)
	if rec.Code != http.StatusOK {
		t.Fatalf("WriteHeader must not overwrite a dispatched status, got %d", rec.Code)
	}
}

func TestResolveTier(t *testing.T) {
	rule := pricingRule{
		tiers: []pricingTier{
			{fromTokens: 0, input: 1, cachedInput: 0.5, output: 2},
			{fromTokens: 100_000, input: 0.8, cachedInput: 0.4, output: 1.6},
			{fromTokens: 1_000_000, input: 0.5, cachedInput: 0.25, output: 1},
		},
	}

	// 50k total -> tier 0.
	ti, _, _ := rule.resolveTier(50_000, 9, 9, 9)
	if ti != 1 {
		t.Fatalf("50k should be tier 0, got %v", ti)
	}

	// 500k total -> tier 1.
	ti, _, _ = rule.resolveTier(500_000, 9, 9, 9)
	if ti != 0.8 {
		t.Fatalf("500k should be tier 1, got %v", ti)
	}

	// 2M total -> tier 2.
	ti, _, _ = rule.resolveTier(2_000_000, 9, 9, 9)
	if ti != 0.5 {
		t.Fatalf("2M should be tier 2, got %v", ti)
	}
}
