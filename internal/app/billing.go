package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const billingScale = 8

const billingMaxAmount = "999999999999.99999999"

type ImagePrice struct {
	Size    string  `json:"size"`
	Quality string  `json:"quality"`
	Price   float64 `json:"price"`
}

type DimensionPrices struct {
	CacheWritePerMillion   *float64     `json:"cache_write_per_million,omitempty"`
	CacheWrite5mPerMillion *float64     `json:"cache_write_5m_per_million,omitempty"`
	CacheWrite1hPerMillion *float64     `json:"cache_write_1h_per_million,omitempty"`
	AudioInputPerMillion   *float64     `json:"audio_input_per_million,omitempty"`
	AudioOutputPerMillion  *float64     `json:"audio_output_per_million,omitempty"`
	ImageInputPerMillion   *float64     `json:"image_input_per_million,omitempty"`
	ImageOutputPerMillion  *float64     `json:"image_output_per_million,omitempty"`
	AudioPerSecond         *float64     `json:"audio_per_second,omitempty"`
	VideoPerSecond         *float64     `json:"video_per_second,omitempty"`
	ToolPerCall            *float64     `json:"tool_per_call,omitempty"`
	Images                 []ImagePrice `json:"images,omitempty"`
}

type BillingPricing struct {
	Version               string          `json:"version"`
	PricedAt              time.Time       `json:"priced_at"`
	BillingMode           string          `json:"billing_mode"`
	Currency              string          `json:"currency"`
	InputPerMillion       float64         `json:"input_per_million"`
	CachedInputPerMillion float64         `json:"cached_input_per_million"`
	OutputPerMillion      float64         `json:"output_per_million"`
	DimensionPrices       DimensionPrices `json:"dimension_prices"`
}

type BillingExchange struct {
	Currency     string  `json:"currency"`
	BaseCurrency string  `json:"base_currency"`
	RateToBase   float64 `json:"rate_to_base"`
}

type BillingMultipliers struct {
	Model float64 `json:"model"`
	Group float64 `json:"group"`
}

type BillingRounding struct {
	Mode  string `json:"mode"`
	Scale int    `json:"scale"`
}

type BillingAdjustment struct {
	Reason string `json:"reason"`
	Limit  string `json:"limit"`
	Amount string `json:"amount"`
}

type BillingSnapshot struct {
	Usage          UsageFacts         `json:"usage"`
	Cost           float64            `json:"cost"`
	Amount         string             `json:"amount"`
	ComputedAmount string             `json:"computed_amount"`
	Status         string             `json:"status"`
	Error          string             `json:"error,omitempty"`
	Pricing        BillingPricing     `json:"pricing"`
	Exchange       BillingExchange    `json:"exchange"`
	Multipliers    BillingMultipliers `json:"multipliers"`
	Rounding       BillingRounding    `json:"rounding"`
	Adjustment     *BillingAdjustment `json:"adjustment,omitempty"`
}

func parseDimensionPrices(raw json.RawMessage) (DimensionPrices, error) {
	var prices DimensionPrices
	if len(raw) == 0 || len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return prices, fmt.Errorf("dimension_prices must be an object")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&prices); err != nil {
		return prices, fmt.Errorf("invalid dimension_prices: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return prices, fmt.Errorf("dimension_prices must contain one object")
	}
	var required struct {
		Images []struct {
			Price *float64 `json:"price"`
		} `json:"images"`
	}
	if err := json.Unmarshal(raw, &required); err != nil {
		return prices, fmt.Errorf("invalid image prices: %w", err)
	}
	for _, image := range required.Images {
		if image.Price == nil {
			return prices, fmt.Errorf("image price must be explicitly configured")
		}
	}
	return prices, validateDimensionPrices(prices)
}

func validateDimensionPrices(prices DimensionPrices) error {
	rates := []struct {
		name  string
		value *float64
	}{
		{"cache_write_per_million", prices.CacheWritePerMillion},
		{"cache_write_5m_per_million", prices.CacheWrite5mPerMillion},
		{"cache_write_1h_per_million", prices.CacheWrite1hPerMillion},
		{"audio_input_per_million", prices.AudioInputPerMillion},
		{"audio_output_per_million", prices.AudioOutputPerMillion},
		{"image_input_per_million", prices.ImageInputPerMillion},
		{"image_output_per_million", prices.ImageOutputPerMillion},
		{"audio_per_second", prices.AudioPerSecond},
		{"video_per_second", prices.VideoPerSecond},
		{"tool_per_call", prices.ToolPerCall},
	}
	for _, rate := range rates {
		if rate.value != nil && !validPricingRate(*rate.value) {
			return fmt.Errorf("invalid %s", rate.name)
		}
	}
	if len(prices.Images) > 256 {
		return fmt.Errorf("too many image prices")
	}
	seen := make(map[[2]string]bool, len(prices.Images))
	for _, price := range prices.Images {
		if price.Size == "" || price.Quality == "" || len(price.Size) > 100 || len(price.Quality) > 100 || strings.TrimSpace(price.Size) != price.Size || strings.TrimSpace(price.Quality) != price.Quality || !validPricingRate(price.Price) {
			return fmt.Errorf("invalid image price")
		}
		key := [2]string{price.Size, price.Quality}
		if seen[key] {
			return fmt.Errorf("duplicate image price for size %q and quality %q", price.Size, price.Quality)
		}
		seen[key] = true
	}
	return nil
}

func dimensionImagePrice(prices DimensionPrices, size, quality string) (ImagePrice, error) {
	var selected ImagePrice
	best := -1
	for _, price := range prices.Images {
		if price.Size != "*" && price.Size != size || price.Quality != "*" && price.Quality != quality {
			continue
		}
		score := 0
		if price.Size != "*" {
			score += 2
		}
		if price.Quality != "*" {
			score++
		}
		if score > best {
			selected, best = price, score
		}
	}
	if best < 0 {
		return ImagePrice{}, fmt.Errorf("image price is not configured for size %q and quality %q", size, quality)
	}
	if !validPricingRate(selected.Price) {
		return ImagePrice{}, fmt.Errorf("invalid image price")
	}
	return selected, nil
}

func freezePricing(pricing pricingRule, at time.Time) pricingRule {
	if !pricing.pricedAt.IsZero() {
		return pricing
	}
	pricing.input, pricing.cachedInput, pricing.output = pricing.resolvePricing(at)
	pricing.pricedAt = at
	pricing.timeRules = nil
	pricing.tiers = append([]pricingTier(nil), pricing.tiers...)
	pricing.dimensions = cloneDimensionPrices(pricing.dimensions)
	return pricing
}

func cloneDimensionPrices(prices DimensionPrices) DimensionPrices {
	clone := func(value *float64) *float64 {
		if value == nil {
			return nil
		}
		v := *value
		return &v
	}
	prices.CacheWritePerMillion = clone(prices.CacheWritePerMillion)
	prices.CacheWrite5mPerMillion = clone(prices.CacheWrite5mPerMillion)
	prices.CacheWrite1hPerMillion = clone(prices.CacheWrite1hPerMillion)
	prices.AudioInputPerMillion = clone(prices.AudioInputPerMillion)
	prices.AudioOutputPerMillion = clone(prices.AudioOutputPerMillion)
	prices.ImageInputPerMillion = clone(prices.ImageInputPerMillion)
	prices.ImageOutputPerMillion = clone(prices.ImageOutputPerMillion)
	prices.AudioPerSecond = clone(prices.AudioPerSecond)
	prices.VideoPerSecond = clone(prices.VideoPerSecond)
	prices.ToolPerCall = clone(prices.ToolPerCall)
	prices.Images = append([]ImagePrice(nil), prices.Images...)
	return prices
}

func effectiveDimensionPrices(prices DimensionPrices, input, output float64) DimensionPrices {
	prices = cloneDimensionPrices(prices)
	fallback := func(value *float64, otherwise float64) *float64 {
		if value != nil {
			return value
		}
		return &otherwise
	}
	prices.CacheWritePerMillion = fallback(prices.CacheWritePerMillion, input)
	prices.CacheWrite5mPerMillion = fallback(prices.CacheWrite5mPerMillion, *prices.CacheWritePerMillion)
	prices.CacheWrite1hPerMillion = fallback(prices.CacheWrite1hPerMillion, *prices.CacheWritePerMillion)
	prices.AudioInputPerMillion = fallback(prices.AudioInputPerMillion, input)
	prices.AudioOutputPerMillion = fallback(prices.AudioOutputPerMillion, output)
	prices.ImageInputPerMillion = fallback(prices.ImageInputPerMillion, input)
	prices.ImageOutputPerMillion = fallback(prices.ImageOutputPerMillion, output)
	return prices
}

func billingUsageTokens(facts UsageFacts) (int64, error) {
	counts := []int64{facts.InputTokens, facts.OutputTokens, facts.CacheReadTokens, facts.CacheWriteTokens, facts.AudioInputTokens, facts.AudioOutputTokens, facts.ImageInputTokens, facts.ImageOutputTokens}
	var total int64
	for _, count := range counts {
		if count < 0 || count > math.MaxInt64-total {
			return 0, fmt.Errorf("invalid or overflowing token usage")
		}
		total += count
	}
	if facts.CacheWrite5mTokens < 0 || facts.CacheWrite1hTokens < 0 || facts.CacheWrite5mTokens > facts.CacheWriteTokens || facts.CacheWrite1hTokens > facts.CacheWriteTokens-facts.CacheWrite5mTokens {
		return 0, fmt.Errorf("cache write TTL usage exceeds total cache write tokens")
	}
	if facts.ImageCount < 0 || facts.ToolCalls < 0 || !validNonNegativeFinite(facts.AudioSeconds) || !validNonNegativeFinite(facts.VideoSeconds) {
		return 0, fmt.Errorf("invalid dimension usage")
	}
	return total, nil
}

func calculateBill(facts UsageFacts, pricing pricingRule, groupMultiplier float64) (BillingSnapshot, error) {
	pricing = freezePricing(pricing, time.Now())
	currency := pricing.currency
	if currency == "" {
		currency = "CNY"
	}
	snapshot := BillingSnapshot{
		Usage: facts, Status: "unpriced", Amount: "0.00000000", ComputedAmount: "0.00000000",
		Pricing: BillingPricing{
			Version: pricing.version, PricedAt: pricing.pricedAt, BillingMode: pricing.billingMode, Currency: currency,
			InputPerMillion: pricing.input, CachedInputPerMillion: pricing.cachedInput, OutputPerMillion: pricing.output,
			DimensionPrices: cloneDimensionPrices(pricing.dimensions),
		},
		Exchange:    BillingExchange{Currency: currency, BaseCurrency: "CNY", RateToBase: pricing.exchangeRate},
		Multipliers: BillingMultipliers{Model: pricing.multiplier, Group: groupMultiplier},
		Rounding:    BillingRounding{Mode: "HALF_UP", Scale: billingScale},
	}
	fail := func(err error) (BillingSnapshot, error) {
		snapshot.Error = err.Error()
		return snapshot, err
	}
	if !pricing.found {
		return fail(fmt.Errorf("pricing rule is not configured"))
	}
	tokens, err := billingUsageTokens(facts)
	if err != nil {
		return fail(err)
	}
	input, cached, output := pricing.resolveTier(tokens, pricing.input, pricing.cachedInput, pricing.output)
	snapshot.Pricing.InputPerMillion, snapshot.Pricing.CachedInputPerMillion, snapshot.Pricing.OutputPerMillion = input, cached, output
	snapshot.Pricing.DimensionPrices = effectiveDimensionPrices(pricing.dimensions, input, output)
	if err := snapshot.validatePricing(); err != nil {
		return fail(err)
	}
	if snapshot.Pricing.Version == "" {
		versionInput := snapshot
		versionInput.Usage = UsageFacts{}
		versionInput.Pricing.PricedAt = time.Time{}
		encoded, err := json.Marshal(versionInput)
		if err != nil {
			return fail(fmt.Errorf("encode pricing version: %w", err))
		}
		hash := sha256.Sum256(encoded)
		snapshot.Pricing.Version = hex.EncodeToString(hash[:])
	}
	snapshot.Usage.PricingVersion = snapshot.Pricing.Version
	amount, err := snapshot.Recompute()
	if err != nil {
		return fail(err)
	}
	snapshot.Amount, snapshot.ComputedAmount = amount, amount
	snapshot.Cost, err = strconv.ParseFloat(amount, 64)
	if err != nil {
		return fail(fmt.Errorf("convert billed amount: %w", err))
	}
	snapshot.Status = "settled"
	if !facts.HasUsage() || pricing.billingMode == "image" && facts.ImageCount == 0 {
		snapshot.Status = "missing_usage"
	}
	return snapshot, nil
}

func (snapshot BillingSnapshot) validatePricing() error {
	prices := snapshot.Pricing
	if prices.BillingMode != "" && prices.BillingMode != "image" {
		return fmt.Errorf("unsupported billing mode %q", prices.BillingMode)
	}
	if !validCurrencyCode(prices.Currency) || snapshot.Exchange.Currency != prices.Currency || snapshot.Exchange.BaseCurrency != "CNY" || !validPositiveFinite(snapshot.Exchange.RateToBase) {
		return fmt.Errorf("invalid billing exchange rate")
	}
	if !validPricingRate(prices.InputPerMillion) || !validPricingRate(prices.CachedInputPerMillion) || !validPricingRate(prices.OutputPerMillion) || !validPricingMultiplier(snapshot.Multipliers.Model) || !validNonNegativeFinite(snapshot.Multipliers.Group) {
		return fmt.Errorf("invalid billing prices or multipliers")
	}
	if snapshot.Rounding.Mode != "HALF_UP" || snapshot.Rounding.Scale != billingScale {
		return fmt.Errorf("unsupported billing rounding")
	}
	return validateDimensionPrices(prices.DimensionPrices)
}

func billingDecimal(value float64) *big.Rat {
	decimal, _ := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
	return decimal
}

func (snapshot BillingSnapshot) Recompute() (string, error) {
	if err := snapshot.validatePricing(); err != nil {
		return "", err
	}
	facts := snapshot.Usage
	if _, err := billingUsageTokens(facts); err != nil {
		return "", err
	}
	prices := snapshot.Pricing
	dimensions := effectiveDimensionPrices(prices.DimensionPrices, prices.InputPerMillion, prices.OutputPerMillion)
	total := new(big.Rat)
	add := func(quantity *big.Rat, rate float64, unit int64) {
		amount := new(big.Rat).Mul(quantity, billingDecimal(rate))
		amount.Quo(amount, new(big.Rat).SetInt64(unit))
		total.Add(total, amount)
	}
	if prices.BillingMode != "image" {
		counts := []struct {
			quantity int64
			rate     float64
		}{
			{facts.InputTokens, prices.InputPerMillion},
			{facts.OutputTokens, prices.OutputPerMillion},
			{facts.CacheReadTokens, prices.CachedInputPerMillion},
			{facts.CacheWriteTokens - facts.CacheWrite5mTokens - facts.CacheWrite1hTokens, *dimensions.CacheWritePerMillion},
			{facts.CacheWrite5mTokens, *dimensions.CacheWrite5mPerMillion},
			{facts.CacheWrite1hTokens, *dimensions.CacheWrite1hPerMillion},
			{facts.AudioInputTokens, *dimensions.AudioInputPerMillion},
			{facts.AudioOutputTokens, *dimensions.AudioOutputPerMillion},
			{facts.ImageInputTokens, *dimensions.ImageInputPerMillion},
			{facts.ImageOutputTokens, *dimensions.ImageOutputPerMillion},
		}
		for _, count := range counts {
			add(new(big.Rat).SetInt64(count.quantity), count.rate, 1_000_000)
		}
		seconds := []struct {
			name     string
			quantity float64
			rate     *float64
		}{
			{"audio_per_second", facts.AudioSeconds, dimensions.AudioPerSecond},
			{"video_per_second", facts.VideoSeconds, dimensions.VideoPerSecond},
		}
		for _, dimension := range seconds {
			if dimension.quantity == 0 {
				continue
			}
			if dimension.rate == nil {
				return "", fmt.Errorf("%s is not configured", dimension.name)
			}
			add(billingDecimal(dimension.quantity), *dimension.rate, 1)
		}
		if facts.ToolCalls > 0 {
			if dimensions.ToolPerCall == nil {
				return "", fmt.Errorf("tool_per_call is not configured")
			}
			add(new(big.Rat).SetInt64(facts.ToolCalls), *dimensions.ToolPerCall, 1)
		}
	}
	if facts.ImageCount > 0 {
		image, err := dimensionImagePrice(dimensions, facts.ImageSize, facts.ImageQuality)
		if err != nil {
			return "", err
		}
		add(new(big.Rat).SetInt64(facts.ImageCount), image.Price, 1)
	}
	total.Mul(total, billingDecimal(snapshot.Exchange.RateToBase))
	total.Mul(total, billingDecimal(snapshot.Multipliers.Model))
	total.Mul(total, billingDecimal(snapshot.Multipliers.Group))
	amount := billingRound(total)
	value, _ := new(big.Rat).SetString(amount)
	maximum, _ := new(big.Rat).SetString(billingMaxAmount)
	if value.Cmp(maximum) > 0 {
		return "", fmt.Errorf("billing amount exceeds numeric(20,8)")
	}
	return amount, nil
}

func billingRound(amount *big.Rat) string {
	scaled := new(big.Rat).Mul(amount, big.NewRat(100_000_000, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(scaled.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return new(big.Rat).SetFrac(quotient, big.NewInt(100_000_000)).FloatString(billingScale)
}

func (snapshot *BillingSnapshot) ApplyCap(limit float64) error {
	if !validNonNegativeFinite(limit) {
		return fmt.Errorf("invalid billing cap")
	}
	amount, ok := new(big.Rat).SetString(snapshot.Amount)
	if !ok || amount.Sign() < 0 {
		return fmt.Errorf("invalid billed amount")
	}
	capAmount := billingRound(billingDecimal(limit))
	capValue, _ := new(big.Rat).SetString(capAmount)
	if amount.Cmp(capValue) <= 0 {
		return nil
	}
	computed, ok := new(big.Rat).SetString(snapshot.ComputedAmount)
	if !ok || computed.Cmp(amount) < 0 {
		return fmt.Errorf("invalid computed amount")
	}
	difference := new(big.Rat).Sub(computed, capValue)
	snapshot.Adjustment = &BillingAdjustment{Reason: "hold_cap", Limit: capAmount, Amount: difference.FloatString(billingScale)}
	snapshot.Amount = capAmount
	snapshot.Cost, _ = strconv.ParseFloat(capAmount, 64)
	snapshot.Status = "hold_capped"
	return nil
}
