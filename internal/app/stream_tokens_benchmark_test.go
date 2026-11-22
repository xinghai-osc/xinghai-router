package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func BenchmarkFirstTokenFrame(b *testing.B) {
	const frame = "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"
	for _, tc := range []struct {
		name  string
		frame string
		step  int
	}{
		{"Chat", frame, len(frame)},
		{"Fragmented", frame, 1},
		{"Batched", strings.Repeat(frame, 128), len(frame) * 128},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data := []byte(tc.frame)
			sink := &quotaFailWriter{header: make(http.Header)}
			b.ReportAllocs()
			for b.Loop() {
				var tracker firstTokenTracker
				w := newFirstTokenWriter(sink, &tracker)
				for off := 0; off < len(data); off += tc.step {
					if _, err := w.Write(data[off:min(off+tc.step, len(data))]); err != nil {
						b.Fatal(err)
					}
				}
				if tracker.at.IsZero() {
					b.Fatal("missing first token")
				}
			}
		})
	}
}

func BenchmarkResponsesStreamTokens(b *testing.B) {
	for _, tc := range []struct{ name, start, delta, end string }{
		{"Chat", "", `data: {"id":"chunk","model":"test","choices":[{"delta":{"content":"hello"},"finish_reason":null}],"usage":null}`, `data: {"choices":[],"usage":{"prompt_tokens":40,"completion_tokens":128}}`},
		{"Anthropic", "data: {\"type\":\"message_start\",\"message\":{\"model\":\"test\",\"usage\":{\"input_tokens\":40}}}\n\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n", `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":128}}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			body := tc.start + strings.Repeat(tc.delta+"\n\n", 128) + tc.end + "\n\n"
			sink := &quotaFailWriter{header: make(http.Header)}
			b.ReportAllocs()
			for b.Loop() {
				resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
				st, err := streamChatCompletionsToResponses(sink, resp, "resp_test", responsesEcho{})
				if err != nil || !st.usageComplete || st.prompt != 40 || st.completion != 128 {
					b.Fatalf("stats=%+v err=%v", st, err)
				}
			}
		})
	}
}
