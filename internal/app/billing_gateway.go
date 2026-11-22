package app

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"time"
)

func reservationAmount(bodyLen, maxTokens int, pricing pricingRule, groupMultiplier float64) (float64, error) {
	if pricing.pricedAt.IsZero() {
		pricing = freezePricing(pricing, time.Now())
	}
	input, cached, output := pricing.input, pricing.cachedInput, pricing.output
	if pricing.serviceTier != "" && pricing.serviceTier != "default" && pricing.serviceTier != "auto" {
		servicePrice, ok := pricing.serviceTiers[pricing.serviceTier]
		if !ok {
			return 0, errPricingUnavailable
		}
		input, cached, output = servicePrice.InputPerMillion, servicePrice.CachedInputPerMillion, servicePrice.OutputPerMillion
	}
	for _, tier := range pricing.tiers {
		input = math.Max(input, tier.input)
		cached = math.Max(cached, tier.cachedInput)
		output = math.Max(output, tier.output)
	}
	input = math.Max(input, cached)
	for _, rate := range []*float64{pricing.dimensions.CacheWritePerMillion, pricing.dimensions.CacheWrite5mPerMillion, pricing.dimensions.CacheWrite1hPerMillion, pricing.dimensions.ImageInputPerMillion} {
		if rate != nil {
			input = math.Max(input, *rate)
		}
	}
	for _, rate := range []*float64{pricing.dimensions.ImageOutputPerMillion} {
		if rate != nil {
			output = math.Max(output, *rate)
		}
	}
	pricing.input, pricing.output, pricing.tiers = input, output, nil
	facts := UsageFacts{InputTokens: int64(max(0, bodyLen/3)), OutputTokens: int64(max(0, maxTokens)), UsageSource: "request_estimate"}
	if pricing.dimensions.ToolPerCall != nil && *pricing.dimensions.ToolPerCall > 0 {
		facts.ToolCalls = 1
	}
	if pricing.dimensions.AudioPerSecond != nil && *pricing.dimensions.AudioPerSecond > 0 {
		facts.AudioSeconds = 1
	}
	if pricing.dimensions.VideoPerSecond != nil && *pricing.dimensions.VideoPerSecond > 0 {
		facts.VideoSeconds = 1
	}
	bill, err := calculateBill(facts, pricing, groupMultiplier)
	return bill.Cost, err
}

func subscriptionUsageStatus(bill BillingSnapshot) string {
	if bill.Status == "subscription" {
		return "settled"
	}
	return bill.Status
}

type streamObservation struct {
	InputReported  bool   `json:"input_reported"`
	OutputReported bool   `json:"output_reported"`
	DeliveredBytes int64  `json:"delivered_bytes"`
	ErrorCode      string `json:"error_code,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
}

func incompleteStreamBill(st streamStats, pricing pricingRule, groupMultiplier float64, deliveredBytes int64, errorCode string, access subscriptionAccess) BillingSnapshot {
	facts := st.facts
	if !facts.HasUsage() {
		facts.UsageSource = "missing"
	}
	bill, _ := calculateBill(facts, pricing, groupMultiplier)
	bill.Status, bill.Cost, bill.Amount = "missing_usage", 0, "0.00000000"
	bill.StreamObservation = &streamObservation{InputReported: st.promptReported, OutputReported: st.completionReported, DeliveredBytes: deliveredBytes, ErrorCode: errorCode}
	if access.Covered {
		bill.StreamObservation.SubscriptionID = access.SubscriptionID
	}
	return bill
}

func (s *Service) recordUnsettledUsage(ctx context.Context, key keyContext, model string, bill BillingSnapshot) {
	usageID, err := randomID()
	if err != nil {
		log.Printf("recordUnsettledUsage id failed: %v", err)
		return
	}
	bill.Cost, bill.Amount = 0, "0.00000000"
	factsJSON, _ := json.Marshal(bill.Usage)
	billJSON, _ := json.Marshal(bill)
	prompt, cached, completion := bill.Usage.LegacyTokens()
	settleCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, err = s.db.Exec(settleCtx, `insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost,status,usage_facts,billing_snapshot)
		values($1::uuid,$2,$3,$4::uuid,$5,$6,$7,$8,0,$9,$10::jsonb,$11::jsonb)
		on conflict(request_id) do nothing`, usageID, requestID(ctx), key.userID, key.keyID, model, prompt, cached, completion, bill.Status, factsJSON, billJSON)
	if err != nil {
		log.Printf("recordUnsettledUsage failed request=%s: %v", requestID(ctx), err)
	}
}
