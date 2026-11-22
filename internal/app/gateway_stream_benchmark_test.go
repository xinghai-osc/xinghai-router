package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func BenchmarkGatewayStream(b *testing.B) {
	for _, tc := range []struct {
		name  string
		relay streamTransform
		delta string
		end   string
	}{
		{"Chat", (&Service{}).streamResponse, `data: {"id":"chunk","model":"test","choices":[{"delta":{"content":"hello"},"finish_reason":null}],"usage":null}`, `data: {"id":"chunk","model":"test","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":40,"completion_tokens":128}}`},
		{"Responses", streamResponseDirect, `data: {"type":"response.output_text.delta","delta":"hello"}`, `data: {"type":"response.completed","response":{"usage":{"input_tokens":40,"output_tokens":128}}}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			body := strings.Repeat(tc.delta+"\n\n", 128) + tc.end + "\n\n"
			w := &quotaFailWriter{header: make(http.Header)}
			b.ReportAllocs()
			for b.Loop() {
				resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
				stats, err := tc.relay(w, resp)
				if err != nil || !stats.usageComplete || stats.completion != 128 {
					b.Fatalf("stats=%+v err=%v", stats, err)
				}
			}
		})
	}
}
