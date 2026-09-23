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
	for _, tier := range pricing.tiers {
		input = math.Max(input, tier.input)
		cached = math.Max(cached, tier.cachedInput)
		output = math.Max(output, tier.output)
	}
	input = math.Max(input, cached)
	for _, rate := range []*float64{pricing.dimensions.CacheWritePerMillion, pricing.dimensions.CacheWrite5mPerMillion, pricing.dimensions.CacheWrite1hPerMillion, pricing.dimensions.AudioInputPerMillion, pricing.dimensions.ImageInputPerMillion} {
		if rate != nil {
			input = math.Max(input, *rate)
		}
	}
	for _, rate := range []*float64{pricing.dimensions.AudioOutputPerMillion, pricing.dimensions.ImageOutputPerMillion} {
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

func completeStreamFacts(st *streamStats, body []byte, maxTokens int) {
	prompt, output := estimatedStreamUsage(body, maxTokens)
	if !st.facts.HasUsage() {
		st.facts = usageFactsFromLegacy(prompt, 0, output, "request_estimate")
		return
	}
	if !st.promptReported {
		st.facts.InputTokens = int64(prompt)
	}
	if !st.completionReported {
		st.facts.OutputTokens = int64(max(0, output-int(st.facts.AudioOutputTokens+st.facts.ImageOutputTokens)))
	}
	st.facts.UsageSource = "upstream_and_request_estimate"
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
