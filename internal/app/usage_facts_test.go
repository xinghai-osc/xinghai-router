package app

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestUsageFactsNormalization(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		format string
		want   UsageFacts
		legacy [3]int
	}{
		{
			name: "anthropic cache only",
			body: `{"type":"message","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":700,"cache_creation_input_tokens":300}}`,
			want: UsageFacts{CacheReadTokens: 700, CacheWriteTokens: 300}, legacy: [3]int{1000, 700, 0},
		},
		{
			name: "mixed cache lifetimes",
			body: `{"type":"message","usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":40,"cache_creation_input_tokens":90,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":60}}}`,
			want: UsageFacts{InputTokens: 10, OutputTokens: 20, CacheReadTokens: 40, CacheWriteTokens: 90, CacheWrite5mTokens: 30, CacheWrite1hTokens: 60}, legacy: [3]int{140, 40, 20},
		},
		{
			name: "cache total from lifetimes",
			body: `{"usage":{"input_tokens":10,"output_tokens":2,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":60}}}`,
			want: UsageFacts{InputTokens: 10, OutputTokens: 2, CacheWriteTokens: 90, CacheWrite5mTokens: 30, CacheWrite1hTokens: 60}, legacy: [3]int{100, 0, 2},
		},
		{
			name: "openai cache audio and image details",
			body: `{"usage":{"prompt_tokens":100,"completion_tokens":70,"prompt_tokens_details":{"cached_tokens":20,"audio_tokens":15,"image_tokens":5},"completion_tokens_details":{"audio_tokens":30,"image_tokens":10}}}`,
			want: UsageFacts{InputTokens: 60, OutputTokens: 30, CacheReadTokens: 20, AudioInputTokens: 15, ImageInputTokens: 5, AudioOutputTokens: 30, ImageOutputTokens: 10}, legacy: [3]int{100, 20, 70},
		},
		{
			name: "nested responses",
			body: `{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":70,"input_tokens_details":{"cached_tokens":20,"audio_tokens":15,"image_tokens":5},"output_tokens_details":{"audio_tokens":30,"image_tokens":10}}}}`,
			want: UsageFacts{InputTokens: 60, OutputTokens: 30, CacheReadTokens: 20, AudioInputTokens: 15, ImageInputTokens: 5, AudioOutputTokens: 30, ImageOutputTokens: 10}, legacy: [3]int{100, 20, 70},
		},
		{
			name: "explicit openai format avoids anthropic cache inference",
			body: `{"usage":{"input_tokens":100,"output_tokens":10,"cache_read_input_tokens":30}}`, format: "openai",
			want: UsageFacts{InputTokens: 70, OutputTokens: 10, CacheReadTokens: 30}, legacy: [3]int{100, 30, 10},
		},
		{
			name: "native openai shape overrides channel hint",
			body: `{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":30}}}`, format: "anthropic",
			want: UsageFacts{InputTokens: 70, OutputTokens: 10, CacheReadTokens: 30}, legacy: [3]int{100, 30, 10},
		},
		{
			name: "native anthropic shape overrides channel hint",
			body: `{"type":"message","usage":{"input_tokens":100,"output_tokens":10,"cache_read_input_tokens":30}}`, format: "openai",
			want: UsageFacts{InputTokens: 100, OutputTokens: 10, CacheReadTokens: 30}, legacy: [3]int{130, 30, 10},
		},
		{
			name: "gemini metadata",
			body: `{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":20,"cachedContentTokenCount":30,"promptTokensDetails":[{"modality":"AUDIO","tokenCount":10},{"modality":"IMAGE","tokenCount":5}]}}`,
			want: UsageFacts{InputTokens: 55, OutputTokens: 20, CacheReadTokens: 30, AudioInputTokens: 10, ImageInputTokens: 5}, legacy: [3]int{100, 30, 20},
		},
		{
			name: "legacy camel aliases",
			body: `{"usage":{"promptTokens":80,"completionTokens":20,"prompt_tokens_details":{"cachedTokens":30}}}`,
			want: UsageFacts{InputTokens: 50, OutputTokens: 20, CacheReadTokens: 30}, legacy: [3]int{80, 30, 20},
		},
		{
			name: "legacy total derives missing output",
			body: `{"usage":{"inputTokens":80,"totalTokens":100,"input_tokens_details":{"cachedTokens":30}}}`,
			want: UsageFacts{InputTokens: 50, OutputTokens: 20, CacheReadTokens: 30}, legacy: [3]int{80, 30, 20},
		},
		{
			name: "commandcode native usage",
			body: `{"type":"finish","totalUsage":{"inputTokens":100,"outputTokens":15,"inputTokenDetails":{"noCacheTokens":20,"cacheReadTokens":50,"cacheWriteTokens":30}}}`,
			want: UsageFacts{InputTokens: 20, OutputTokens: 15, CacheReadTokens: 50, CacheWriteTokens: 30}, legacy: [3]int{100, 50, 15},
		},
		{
			name: "explicit dimensions and server tools",
			body: `{"size":"1024x1024","quality":"high","usage":{"image_count":2,"audio_seconds":1.25,"video_seconds":2.5,"server_tool_use":{"web_search_requests":3,"web_fetch_requests":2},"pricing_version":"v1"},"choices":[{"message":{"tool_calls":[{},{}]}}]}`,
			want: UsageFacts{ImageCount: 2, AudioSeconds: 1.25, VideoSeconds: 2.5, ToolCalls: 5, PricingVersion: "v1", ImageSize: "1024x1024", ImageQuality: "high"},
		},
		{
			name: "duration transcription",
			body: `{"usage":{"type":"duration","seconds":5.25}}`, want: UsageFacts{AudioSeconds: 5.25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUsageFactsForFormat([]byte(tt.body), tt.format)
			if !got.HasUsage() || got.UsageSource != "upstream" {
				t.Fatalf("explicit usage not recognized: %+v", got)
			}
			tt.want.UsageSource = "upstream"
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("facts = %s, want %s", gotJSON, wantJSON)
			}
			prompt, cached, completion := got.LegacyTokens()
			if actual := [3]int{prompt, cached, completion}; actual != tt.legacy {
				t.Fatalf("legacy = %v, want %v", actual, tt.legacy)
			}
		})
	}
}

func TestUsageFactsMissingZeroAndInvalid(t *testing.T) {
	for _, body := range []string{``, `null`, `{}`, `{"usage":null}`, `{"usage":{}}`, `{"usage":{"input_tokens":"100"}}`, `{"choices":[{"message":{"tool_calls":[{},{}]}}]}`} {
		facts := parseUsageFacts([]byte(body))
		if facts.HasUsage() || facts.UsageSource != "missing" {
			t.Fatalf("missing usage %s became %+v", body, facts)
		}
	}
	zero := parseUsageFacts([]byte(`{"usage":{"prompt_tokens":0,"completion_tokens":0}}`))
	if !zero.HasUsage() || zero.UsageSource != "upstream" {
		t.Fatalf("explicit zero usage lost: %+v", zero)
	}
	invalid := parseUsageFacts([]byte(`{"usage":{"input_tokens":-1,"output_tokens":1e100,"audio_seconds":-2,"video_seconds":1e999,"cache_read_input_tokens":-10,"image_count":2.5}}`))
	if invalid.InputTokens != 0 || invalid.CacheReadTokens != 0 || invalid.AudioSeconds != 0 || invalid.OutputTokens != maxUsageFact || invalid.VideoSeconds != float64(maxUsageFact) || invalid.ImageCount != 2 {
		t.Fatalf("invalid dimensions not bounded: %+v", invalid)
	}
	facts := usageFactsFromLegacy(100, 30, 15, "local_estimate")
	if facts.InputTokens != 70 || facts.CacheReadTokens != 30 || facts.OutputTokens != 15 || facts.UsageSource != "local_estimate" {
		t.Fatalf("legacy estimate normalization = %+v", facts)
	}
	if facts := usageFactsFromLegacy(0, 0, 0, ""); facts.HasUsage() || facts.UsageSource == "upstream" {
		t.Fatalf("source-free legacy zero became upstream usage: %+v", facts)
	}
	bounded := parseUsageFacts([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":100,"audio_tokens":100},"completion_tokens_details":{"audio_tokens":100,"image_tokens":100}}}`))
	prompt, cached, completion := bounded.LegacyTokens()
	if prompt != 10 || cached != 10 || completion != 5 {
		t.Fatalf("inconsistent inclusive totals inflated legacy counts: %+v", bounded)
	}
	large := UsageFacts{InputTokens: maxUsageFact, CacheReadTokens: maxUsageFact, CacheWriteTokens: maxUsageFact, AudioInputTokens: maxUsageFact, OutputTokens: maxUsageFact}
	prompt, _, completion = large.LegacyTokens()
	if int64(prompt)+int64(completion) > 2_147_483_647 || prompt < 0 || completion < 0 {
		t.Fatalf("legacy totals overflow PostgreSQL integer: %d/%d", prompt, completion)
	}
}

func TestUsageFactsSSECumulativePresence(t *testing.T) {
	var st streamStats
	start := `{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":40,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":60}}}}`
	parseSSEUsage([]byte(start), &st)
	if st.usageComplete {
		t.Fatal("message_start completed output usage")
	}
	for _, body := range []string{start, `{"type":"message_delta","usage":{"output_tokens":5}}`, `{"type":"message_delta","usage":{"output_tokens":12}}`, `{"type":"message_delta","usage":{"output_tokens":12}}`, `{"type":"message_delta","usage":{"output_tokens":8}}`} {
		parseSSEUsage([]byte(body), &st)
	}
	if !st.usageComplete || st.prompt != 140 || st.cached != 40 || st.completion != 12 || st.facts.CacheWriteTokens != 90 || st.facts.InputTokens != 10 {
		t.Fatalf("cumulative facts = %+v", st)
	}
	var delayed streamStats
	parseSSEUsage([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":70}}`), &delayed)
	parseSSEUsage([]byte(`{"usage":{"prompt_tokens_details":{"cached_tokens":20,"audio_tokens":15},"completion_tokens_details":{"audio_tokens":30}}}`), &delayed)
	if delayed.prompt != 100 || delayed.completion != 70 || delayed.facts.InputTokens != 65 || delayed.facts.OutputTokens != 40 {
		t.Fatalf("late details double counted tokens: %+v", delayed)
	}
	var zero streamStats
	parseSSEUsage([]byte(`{"usage":{"prompt_tokens":0,"completion_tokens":0}}`), &zero)
	if !zero.usageComplete || !zero.usageReported || !zero.facts.HasUsage() {
		t.Fatalf("explicit complete zero usage = %+v", zero)
	}
	var zeroAnthropic streamStats
	parseSSEUsage([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":0,"output_tokens":0}}}`), &zeroAnthropic)
	parseSSEUsage([]byte(`{"type":"message_delta","usage":{"output_tokens":0}}`), &zeroAnthropic)
	if !zeroAnthropic.usageComplete {
		t.Fatalf("explicit complete zero Anthropic usage = %+v", zeroAnthropic)
	}
	var missing streamStats
	parseSSEUsage([]byte(`{"choices":[],"usage":null}`), &missing)
	if missing.usageReported || missing.usageComplete || missing.facts.HasUsage() {
		t.Fatalf("missing SSE usage became reported: %+v", missing)
	}
}

func TestUsageFactsAdaptersPreserveRawUsage(t *testing.T) {
	body := `{"id":"msg-1","type":"message","model":"test","content":[],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":40,"cache_creation_input_tokens":90,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":60},"server_tool_use":{"web_search_requests":2}}}`
	converted, err := anthropicResponseToOpenAI([]byte(body), "")
	if err != nil {
		t.Fatal(err)
	}
	got := parseUsageFacts(converted)
	if got.InputTokens != 10 || got.CacheReadTokens != 40 || got.CacheWriteTokens != 90 || got.CacheWrite1hTokens != 60 || got.ToolCalls != 2 {
		t.Fatalf("Anthropic conversion lost facts: %s: %+v", converted, got)
	}
	roundtrip, err := openAIToAnthropic(converted)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(parseUsageFacts([]byte(body)))
	actual, _ := json.Marshal(parseUsageFacts(roundtrip))
	if string(original) != string(actual) {
		t.Fatalf("adapter roundtrip usage = %s, want %s", actual, original)
	}
	stream := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-1\",\"model\":\"test\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"cache_read_input_tokens\":40,\"cache_creation_input_tokens\":90}}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":20}}\n\n"
	stats, err := streamAnthropicToOpenAI(httptest.NewRecorder(), streamBody(t, stream), "")
	if err != nil || stats.prompt != 140 || stats.facts.CacheWriteTokens != 90 || stats.facts.InputTokens != 10 || !stats.usageComplete {
		t.Fatalf("raw Anthropic stream facts = %+v, %v", stats, err)
	}
	ccStream := "data: {\"type\":\"finish\",\"totalUsage\":{\"inputTokens\":100,\"outputTokens\":15,\"inputTokenDetails\":{\"noCacheTokens\":20,\"cacheReadTokens\":50,\"cacheWriteTokens\":30}}}\n\n"
	ccStats, err := streamCommandCodeToOpenAI(httptest.NewRecorder(), streamBody(t, ccStream))
	if err != nil || ccStats.prompt != 100 || ccStats.facts.InputTokens != 20 || ccStats.facts.CacheWriteTokens != 30 || !ccStats.usageComplete {
		t.Fatalf("raw CommandCode stream facts = %+v, %v", ccStats, err)
	}
	ccBody, err := commandCodeStreamToOpenAI([]byte(ccStream))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte(ccStream), ccBody} {
		facts := parseUsageFacts(data)
		if facts.InputTokens != 20 || facts.CacheWriteTokens != 30 || facts.CacheReadTokens != 50 {
			t.Fatalf("CommandCode buffered usage lost: %s: %+v", data, facts)
		}
	}
	openAIStream := `data: {"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":70,"prompt_tokens_details":{"cached_tokens":20,"audio_tokens":15},"completion_tokens_details":{"audio_tokens":30}}}` + "\n\ndata: [DONE]\n\n"
	openAIStats, err := streamOpenAIToAnthropic(httptest.NewRecorder(), streamBody(t, openAIStream))
	if err != nil || openAIStats.facts.InputTokens != 65 || openAIStats.facts.OutputTokens != 40 || openAIStats.facts.AudioInputTokens != 15 || openAIStats.facts.AudioOutputTokens != 30 {
		t.Fatalf("raw OpenAI stream facts = %+v, %v", openAIStats, err)
	}
}

func TestUsageFactsJSONFields(t *testing.T) {
	value := reflect.TypeOf(UsageFacts{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || strings.ToLower(tag) != tag {
			t.Fatalf("field %s has non snake_case JSON tag %q", field.Name, tag)
		}
	}
}
