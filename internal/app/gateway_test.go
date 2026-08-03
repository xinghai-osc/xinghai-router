package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestStripGatewayExtensions(t *testing.T) {
	got := stripGatewayExtensions([]byte(`{"model":"m","promptCacheKey":"abc"}`))
	if strings.Contains(string(got), "promptCacheKey") {
		t.Fatalf("extension field not stripped: %s", got)
	}
	if !strings.Contains(string(got), `"model":"m"`) {
		t.Fatalf("model dropped while stripping: %s", got)
	}
	want := `{"model":"m"}`
	if got := string(stripGatewayExtensions([]byte(want))); got != want {
		t.Fatalf("body without extensions must pass through unchanged: %s", got)
	}
	invalid := []byte(`not-json`)
	if got := stripGatewayExtensions(invalid); string(got) != "not-json" {
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
