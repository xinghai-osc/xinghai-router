package app

import (
	"bytes"
	"mime/multipart"
	"testing"
	"time"
)

func TestImageBillingOptionsAndResponseCount(t *testing.T) {
	options, err := imageBillingOptions([]byte(`{"n":3,"size":"1024x1024","quality":"hd"}`), "application/json")
	if err != nil || options.count != 3 || options.size != "1024x1024" || options.quality != "hd" {
		t.Fatalf("options=%+v err=%v", options, err)
	}
	facts, err := imageResponseFacts([]byte(`{"data":[{"url":"https://example.invalid/image.png"},{"b64_json":"abc"}]}`), options, parseUsageFacts([]byte(`{}`)))
	if err != nil || facts.ImageCount != 2 || facts.UsageSource != "response_count" {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
	price := pricingRule{found: true, multiplier: 1, exchangeRate: 7, currency: "USD", billingMode: "image", dimensions: DimensionPrices{Images: []ImagePrice{{Size: "1024x1024", Quality: "hd", Price: 0.04}}}}
	reserve, err := calculateBill(options.reservationFacts(), price, 1)
	if err != nil || reserve.Amount != "0.84000000" {
		t.Fatalf("reservation=%+v err=%v", reserve, err)
	}
	bill, err := calculateBill(facts, price, 1)
	if err != nil || bill.Amount != "0.56000000" {
		t.Fatalf("bill=%+v err=%v", bill, err)
	}
	for _, body := range []string{`{}`, `{"data":[]}`, `{"data":[{}]}`, `{"data":[{"url":"a"},{"url":"b"},{"url":"c"},{"url":"d"}]}`} {
		if _, err := imageResponseFacts([]byte(body), options, UsageFacts{}); err == nil {
			t.Fatalf("accepted invalid response %s", body)
		}
	}
}

func TestImageBillingRejectsInvalidSelectors(t *testing.T) {
	for _, body := range []string{`{"n":0}`, `{"n":-1}`, `{"n":101}`, `{"n":1.5}`, `{"stream":true}`, `{"quality":"*"}`, `{"size":"*"}`} {
		if _, err := imageBillingOptions([]byte(body), "application/json"); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	options, err := imageBillingOptions([]byte(`{}`), "application/json")
	if err != nil || options.count != 1 || options.size != "default" || options.quality != "default" {
		t.Fatalf("defaults=%+v err=%v", options, err)
	}
}

func TestImageUploadBytesDoNotAffectReservation(t *testing.T) {
	makeOptions := func(size int) imageGatewayOptions {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("n", "2"); err != nil {
			t.Fatal(err)
		}
		part, err := writer.CreateFormFile("image", "source.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(bytes.Repeat([]byte{0}, size)); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		options, err := imageBillingOptions(body.Bytes(), writer.FormDataContentType())
		if err != nil {
			t.Fatal(err)
		}
		return options
	}
	small, large := makeOptions(10), makeOptions(1<<20)
	if small.reservationFacts() != large.reservationFacts() {
		t.Fatal("upload size changed image reservation facts")
	}
	if small.count != 2 {
		t.Fatalf("count=%d", small.count)
	}
}

func TestPartialStreamFactsEstimateOnlyMissingOutput(t *testing.T) {
	var st streamStats
	parseSSEUsage([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":30,"output_tokens":0}}}`), &st)
	completeStreamFacts(&st, []byte(`{"messages":[]}`), 100)
	if st.facts.InputTokens != 10 || st.facts.CacheReadTokens != 20 || st.facts.CacheWriteTokens != 30 || st.facts.OutputTokens != 100 || st.facts.UsageSource != "upstream_and_request_estimate" {
		t.Fatalf("partial facts=%+v", st.facts)
	}
}

func TestNonTokenReservationAdmissionFloor(t *testing.T) {
	rate := 1.0
	pricing := pricingRule{found: true, multiplier: 1, exchangeRate: 1, dimensions: DimensionPrices{ToolPerCall: &rate, AudioPerSecond: &rate, VideoPerSecond: &rate}}
	amount, err := reservationAmount(30, 100, pricing, 1)
	if err != nil || amount != 3 {
		t.Fatalf("amount=%v err=%v", amount, err)
	}
}

func TestReservationUsesMaximumDimensionAndTier(t *testing.T) {
	cacheWrite, audioOutput := 12.0, 15.0
	pricing := freezePricing(pricingRule{found: true, input: 1, output: 2, multiplier: 1, exchangeRate: 1,
		dimensions: DimensionPrices{CacheWrite1hPerMillion: &cacheWrite, AudioOutputPerMillion: &audioOutput},
		tiers:      []pricingTier{{fromTokens: 100, input: 20, output: 3}, {fromTokens: 10000, input: 2, output: 2}},
	}, time.Now())
	amount, err := reservationAmount(300, 1000, pricing, 1)
	if err != nil || amount != 0.017 {
		t.Fatalf("amount=%v err=%v", amount, err)
	}
}
