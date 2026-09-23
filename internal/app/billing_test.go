package app

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func billingTestRate(value float64) *float64 { return &value }

func billingTestPricing() pricingRule {
	return pricingRule{input: 2, cachedInput: 0.5, output: 4, multiplier: 1, exchangeRate: 1, currency: "CNY", found: true}
}

func TestCalculateBillDimensions(t *testing.T) {
	pricing := billingTestPricing()
	pricing.currency, pricing.exchangeRate, pricing.multiplier = "USD", 7.2, 1.25
	pricing.dimensions = DimensionPrices{
		CacheWritePerMillion: billingTestRate(3), CacheWrite5mPerMillion: billingTestRate(5), CacheWrite1hPerMillion: billingTestRate(7),
		AudioInputPerMillion: billingTestRate(11), AudioOutputPerMillion: billingTestRate(13),
		ImageInputPerMillion: billingTestRate(17), ImageOutputPerMillion: billingTestRate(19),
		AudioPerSecond: billingTestRate(0.1), VideoPerSecond: billingTestRate(0.2), ToolPerCall: billingTestRate(0.3),
		Images: []ImagePrice{{Size: "*", Quality: "*", Price: 0.4}},
	}
	facts := UsageFacts{
		InputTokens: 100, OutputTokens: 200, CacheReadTokens: 50,
		CacheWriteTokens: 60, CacheWrite5mTokens: 10, CacheWrite1hTokens: 20,
		AudioInputTokens: 30, AudioOutputTokens: 40, ImageInputTokens: 50, ImageOutputTokens: 60,
		AudioSeconds: 1.25, VideoSeconds: 2.5, ToolCalls: 2, ImageCount: 3,
	}
	snapshot, err := calculateBill(facts, pricing, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Amount != "17.48984400" || snapshot.Cost != 17.489844 || snapshot.Status != "settled" {
		t.Fatalf("unexpected bill: %+v", snapshot)
	}
	if snapshot.Usage.CacheWriteTokens != 60 || snapshot.Pricing.Version == "" || snapshot.Usage.PricingVersion != snapshot.Pricing.Version {
		t.Fatalf("missing usage or version: %+v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var restored BillingSnapshot
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	recomputed, err := restored.Recompute()
	if err != nil || recomputed != snapshot.Amount {
		t.Fatalf("recompute = %q, %v", recomputed, err)
	}
}

func TestCalculateBillTokenFallbacks(t *testing.T) {
	facts := UsageFacts{InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 10, CacheWrite5mTokens: 3, CacheWrite1hTokens: 4, AudioInputTokens: 5, AudioOutputTokens: 6, ImageInputTokens: 7, ImageOutputTokens: 8}
	snapshot, err := calculateBill(facts, billingTestPricing(), 1)
	if err != nil || snapshot.Amount != "0.00011150" {
		t.Fatalf("fallback bill = %s, %v", snapshot.Amount, err)
	}
	if *snapshot.Pricing.DimensionPrices.CacheWrite5mPerMillion != 2 || *snapshot.Pricing.DimensionPrices.AudioOutputPerMillion != 4 {
		t.Fatalf("fallback rates not frozen: %+v", snapshot.Pricing.DimensionPrices)
	}
	pricing := billingTestPricing()
	pricing.dimensions.CacheWritePerMillion = billingTestRate(6)
	snapshot, err = calculateBill(UsageFacts{CacheWriteTokens: 10, CacheWrite5mTokens: 3, CacheWrite1hTokens: 4}, pricing, 1)
	if err != nil || snapshot.Amount != "0.00006000" {
		t.Fatalf("TTL fallback bill = %s, %v", snapshot.Amount, err)
	}
}

func TestCalculateBillRoundsOnceHalfUp(t *testing.T) {
	for _, test := range []struct {
		name   string
		input  float64
		output float64
		want   string
	}{
		{"half", 0.005, 0, "0.00000001"},
		{"below half", 0.004999999, 0, "0.00000000"},
		{"combined halves", 0.005, 0.005, "0.00000001"},
		{"decimal exact", 0.014999999999999998, 0.000000000000000002, "0.00000002"},
	} {
		t.Run(test.name, func(t *testing.T) {
			pricing := billingTestPricing()
			pricing.input, pricing.output = test.input, test.output
			snapshot, err := calculateBill(UsageFacts{InputTokens: 1, OutputTokens: 1}, pricing, 1)
			if err != nil || snapshot.Amount != test.want {
				t.Fatalf("amount = %s, want %s: %v", snapshot.Amount, test.want, err)
			}
		})
	}
}

func TestCalculateBillMissingTariffs(t *testing.T) {
	for _, test := range []struct {
		name  string
		facts UsageFacts
	}{
		{"audio", UsageFacts{AudioSeconds: 1}},
		{"video", UsageFacts{VideoSeconds: 1}},
		{"tool", UsageFacts{ToolCalls: 1}},
		{"image", UsageFacts{ImageCount: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := calculateBill(test.facts, billingTestPricing(), 1)
			if err == nil || snapshot.Status != "unpriced" || snapshot.Error == "" || snapshot.Usage.PricingVersion == "" || snapshot.Pricing.PricedAt.IsZero() {
				t.Fatalf("missing tariff not preserved: %+v, %v", snapshot, err)
			}
			if _, err := json.Marshal(snapshot); err != nil {
				t.Fatal(err)
			}
		})
	}
	pricing := billingTestPricing()
	pricing.dimensions = DimensionPrices{AudioPerSecond: billingTestRate(0), VideoPerSecond: billingTestRate(0), ToolPerCall: billingTestRate(0), Images: []ImagePrice{{Size: "*", Quality: "*", Price: 0}}}
	if snapshot, err := calculateBill(UsageFacts{AudioSeconds: 1, VideoSeconds: 1, ToolCalls: 1, ImageCount: 1}, pricing, 1); err != nil || snapshot.Cost != 0 {
		t.Fatalf("explicit free tariffs = %+v, %v", snapshot, err)
	}
}

func TestCalculateBillImageModePreservesFacts(t *testing.T) {
	pricing := billingTestPricing()
	pricing.billingMode = "image"
	pricing.dimensions.Images = []ImagePrice{
		{Size: "*", Quality: "*", Price: 1},
		{Size: "*", Quality: "hd", Price: 2},
		{Size: "1024x1024", Quality: "*", Price: 3},
		{Size: "1024x1024", Quality: "hd", Price: 4},
	}
	for _, test := range []struct {
		size, quality, amount string
	}{
		{"1024x1024", "hd", "8.00000000"},
		{"1024x1024", "standard", "6.00000000"},
		{"512x512", "hd", "4.00000000"},
		{"512x512", "standard", "2.00000000"},
	} {
		facts := UsageFacts{ImageCount: 2, InputTokens: 1000, ImageOutputTokens: 2000, ImageSize: test.size, ImageQuality: test.quality}
		snapshot, err := calculateBill(facts, pricing, 1)
		if err != nil || snapshot.Amount != test.amount || snapshot.Usage.InputTokens != 1000 || snapshot.Usage.ImageOutputTokens != 2000 {
			t.Fatalf("image bill = %+v, %v", snapshot, err)
		}
	}
	pricing.dimensions.Images = []ImagePrice{{Size: "1024x1024", Quality: "hd", Price: 4}}
	if _, err := calculateBill(UsageFacts{ImageCount: 1, ImageSize: "512x512", ImageQuality: "hd"}, pricing, 1); err == nil {
		t.Fatal("unmatched image accepted")
	}
}

func TestFreezePricingTimeTierAndCopy(t *testing.T) {
	pricing := billingTestPricing()
	pricing.dimensions.AudioInputPerMillion = billingTestRate(7)
	pricing.timeRules = []pricingTimeRule{{startMinute: 0, endMinute: 720, weekdays: "1111111", input: 5, cachedInput: 1, output: 9}}
	pricing.tiers = []pricingTier{{fromTokens: 10, input: 6, cachedInput: 2, output: 10}}
	at := time.Date(2026, 1, 1, 11, 59, 0, 0, time.FixedZone("UTC+8", 8*3600))
	frozen := freezePricing(pricing, at)
	if len(frozen.timeRules) != 0 || frozen.input != 5 || frozen.pricedAt != at {
		t.Fatalf("not frozen: %+v", frozen)
	}
	*pricing.dimensions.AudioInputPerMillion = 999
	pricing.tiers[0].input = 999
	if *frozen.dimensions.AudioInputPerMillion != 7 || frozen.tiers[0].input != 6 {
		t.Fatal("frozen rule shares mutable prices")
	}
	frozen = freezePricing(frozen, at.Add(time.Hour))
	snapshot, err := calculateBill(UsageFacts{InputTokens: 1}, frozen, 1)
	if err != nil || snapshot.Amount != "0.00000500" || !snapshot.Pricing.PricedAt.Equal(at) {
		t.Fatalf("time price = %+v, %v", snapshot, err)
	}
	snapshot, err = calculateBill(UsageFacts{CacheWriteTokens: 5, CacheWrite5mTokens: 5, AudioInputTokens: 4}, frozen, 1)
	if err != nil || snapshot.Pricing.InputPerMillion != 5 {
		t.Fatalf("TTL subdivisions counted toward tier: %+v, %v", snapshot, err)
	}
	snapshot, err = calculateBill(UsageFacts{CacheWriteTokens: 5, CacheWrite5mTokens: 5, AudioInputTokens: 5}, frozen, 1)
	if err != nil || snapshot.Pricing.InputPerMillion != 6 || *snapshot.Pricing.DimensionPrices.CacheWrite5mPerMillion != 6 {
		t.Fatalf("tier/fallback price = %+v, %v", snapshot, err)
	}
}

func TestBillingSnapshotCapAndRecompute(t *testing.T) {
	snapshot, err := calculateBill(UsageFacts{InputTokens: 1_000_000}, billingTestPricing(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.ApplyCap(1.25); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "hold_capped" || snapshot.Cost != 1.25 || snapshot.Amount != "1.25000000" || snapshot.ComputedAmount != "2.00000000" || snapshot.Adjustment == nil || snapshot.Adjustment.Amount != "0.75000000" {
		t.Fatalf("cap not audited: %+v", snapshot)
	}
	amount, err := snapshot.Recompute()
	if err != nil || amount != "2.00000000" {
		t.Fatalf("cap changed original computation: %s, %v", amount, err)
	}
	if err := snapshot.ApplyCap(math.NaN()); err == nil {
		t.Fatal("invalid cap accepted")
	}
	if err := snapshot.ApplyCap(1); err != nil || snapshot.Adjustment.Amount != "1.00000000" {
		t.Fatalf("repeated cap loses total adjustment: %+v, %v", snapshot, err)
	}
}

func TestBillingPricingVersionStableAcrossTime(t *testing.T) {
	pricing := billingTestPricing()
	first, err := calculateBill(UsageFacts{InputTokens: 1}, freezePricing(pricing, time.Unix(100, 0)), 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := calculateBill(UsageFacts{InputTokens: 2}, freezePricing(pricing, time.Unix(200, 0)), 1)
	if err != nil || first.Pricing.Version != second.Pricing.Version {
		t.Fatalf("usage/time changed price hash: %s / %s, %v", first.Pricing.Version, second.Pricing.Version, err)
	}
	pricing.input = 3
	changed, err := calculateBill(UsageFacts{InputTokens: 1}, pricing, 1)
	if err != nil || first.Pricing.Version == changed.Pricing.Version {
		t.Fatalf("price change not versioned: %+v, %v", changed, err)
	}
	pricing.version = "explicit-v2"
	changed, err = calculateBill(UsageFacts{InputTokens: 1}, pricing, 1)
	if err != nil || changed.Pricing.Version != "explicit-v2" {
		t.Fatalf("explicit version lost: %+v, %v", changed, err)
	}
}

func TestCalculateBillInvalidFactsAndRates(t *testing.T) {
	for _, facts := range []UsageFacts{
		{InputTokens: -1}, {CacheWriteTokens: 1, CacheWrite5mTokens: 2},
		{CacheWriteTokens: 2, CacheWrite5mTokens: 1, CacheWrite1hTokens: 2},
		{InputTokens: math.MaxInt64, OutputTokens: 1}, {AudioSeconds: math.NaN()},
		{VideoSeconds: math.Inf(1)}, {ImageCount: -1}, {ToolCalls: -1},
	} {
		if _, err := calculateBill(facts, billingTestPricing(), 1); err == nil {
			t.Fatalf("invalid facts accepted: %+v", facts)
		}
	}
	pricing := billingTestPricing()
	pricing.input = math.NaN()
	if _, err := calculateBill(UsageFacts{InputTokens: 1}, pricing, 1); err == nil {
		t.Fatal("invalid price accepted")
	}
	pricing = billingTestPricing()
	if _, err := calculateBill(UsageFacts{InputTokens: 1}, pricing, math.Inf(1)); err == nil {
		t.Fatal("invalid multiplier accepted")
	}
	if _, err := calculateBill(UsageFacts{InputTokens: math.MaxInt64}, pricing, 1); err == nil {
		t.Fatal("numeric(20,8) overflow accepted")
	}
	if snapshot, err := calculateBill(UsageFacts{}, pricing, 1); err != nil || snapshot.Status != "missing_usage" {
		t.Fatalf("missing usage = %+v, %v", snapshot, err)
	}
}

func TestParseDimensionPricesValidation(t *testing.T) {
	for _, raw := range []string{
		`null`, `[]`, `{} {}`, `{"unknown":1}`, `{"tool_per_call":-1}`,
		`{"images":[{"size":"","quality":"*","price":1}]}`,
		`{"images":[{"size":"*","quality":"*"}]}`,
		`{"images":[{"size":"*","quality":"*","price":null}]}`,
		`{"images":[{"size":"*","quality":"*","price":1},{"size":"*","quality":"*","price":2}]}`,
	} {
		if _, err := parseDimensionPrices(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	prices, err := parseDimensionPrices(json.RawMessage(`{"audio_per_second":0,"images":[{"size":"*","quality":"*","price":0.1}]}`))
	if err != nil || prices.AudioPerSecond == nil || *prices.AudioPerSecond != 0 {
		t.Fatalf("explicit zero lost: %+v, %v", prices, err)
	}
}

func TestUpsertPricingRejectsInvalidDimensions(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"audio_per_second":-1}`, `{"typo":1}`} {
		req := httptest.NewRequest(http.MethodPost, "/admin/pricing", strings.NewReader(`{"model":"test","input_per_million":1,"output_per_million":2,"multiplier":1,"currency":"CNY","dimension_prices":`+raw+`}`))
		rec := httptest.NewRecorder()
		(&Service{}).upsertPricing(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("dimensions %s: status = %d, body = %s", raw, rec.Code, rec.Body.String())
		}
	}
}
