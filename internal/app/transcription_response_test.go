package app

import "testing"

func TestTranscriptionTextResponse(t *testing.T) {
	const subtitle = "1\r\n00:00:00,000 --> 00:00:01,000\r\n你好\r\n"
	for _, tc := range []struct {
		name, format, upstreamType, body, responseType string
	}{
		{"text", "text", "text/plain; charset=utf-8", "你好", "text/plain"},
		{"silent", "text", "text/plain", "", "text/plain"},
		{"srt", "srt", "application/x-subrip", subtitle, "application/x-subrip"},
		{"srt_as_text", "srt", "text/plain; charset=UTF-8", subtitle, "application/x-subrip"},
		{"vtt", "vtt", "text/vtt", "WEBVTT\n\n", "text/vtt"},
		{"vtt_as_file", "vtt", "application/octet-stream", "\ufeffWEBVTT\n\n", "text/vtt"},
		{"literal_usage", "text", "text/plain", "data: {\"usage\":{\"input_tokens\":999}}\n\n", "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := mediaGatewayOptions{kind: "transcription", responseFormat: tc.format}
			if got := options.transcriptContentType(); got != tc.responseType {
				t.Fatalf("response content type = %q, want %q", got, tc.responseType)
			}
			facts, err := options.responseFacts([]byte(tc.body), tc.upstreamType, billingTestPricing())
			if err != nil || facts.HasUsage() {
				t.Fatalf("facts=%+v err=%v", facts, err)
			}
		})
	}
}

func TestTranscriptionRejectsUnexpectedTextResponses(t *testing.T) {
	for _, tc := range []struct{ name, contentType, body string }{
		{"json", "application/json", `{"error":{"message":"failure"}}`},
		{"mislabeled_json_error", "text/plain", `{"error":{"message":"failure"}}`},
		{"html", "text/html", "server error"},
		{"mislabeled_html", "text/plain", "\ufeff<!doctype html><title>Server error</title>"},
		{"invalid_utf8", "application/octet-stream", "\xff"},
		{"binary", "text/plain", "hello\x00"},
		{"sse", "text/event-stream", "data: {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := mediaGatewayOptions{kind: "transcription", responseFormat: "text"}
			if _, err := options.responseFacts([]byte(tc.body), tc.contentType, billingTestPricing()); err == nil {
				t.Fatal("accepted invalid upstream response")
			}
		})
	}
}

func TestDiarizedTranscriptionUsesReportedTokens(t *testing.T) {
	options := mediaGatewayOptions{kind: "transcription", responseFormat: "diarized_json", maxAudioSeconds: 10}
	if got := options.transcriptContentType(); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
	body := []byte(`{"text":"Hi.","duration":2.5,"segments":[{"speaker":"A","start":0,"end":2.5,"text":"Hi."}],"usage":{"input_tokens":11,"output_tokens":2}}`)
	facts, err := options.responseFacts(body, "application/json", billingTestPricing())
	if err != nil || facts.InputTokens != 11 || facts.OutputTokens != 2 || facts.AudioSeconds != 0 {
		t.Fatalf("facts=%+v err=%v", facts, err)
	}
}
