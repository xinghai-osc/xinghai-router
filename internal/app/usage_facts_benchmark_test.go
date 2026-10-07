package app

import "testing"

func BenchmarkParseUsageFacts(b *testing.B) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"OpenAITextDelta", `{"id":"chatcmpl-test","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`},
		{"AnthropicTextDelta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`},
		{"OpenAIUsage", `{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":70,"prompt_tokens_details":{"cached_tokens":20,"audio_tokens":15,"image_tokens":5},"completion_tokens_details":{"audio_tokens":30,"image_tokens":10}}}`},
		{"AnthropicUsage", `{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":40,"cache_creation_input_tokens":90,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":60}}}}`},
		{"ResponsesUsage", `{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":70,"input_tokens_details":{"cached_tokens":20,"audio_tokens":15,"image_tokens":5},"output_tokens_details":{"audio_tokens":30,"image_tokens":10}}}}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			body := []byte(tc.body)
			b.ReportAllocs()
			for b.Loop() {
				parseUsageFacts(body)
			}
		})
	}
}
