package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func streamBody(t *testing.T, body string) *http.Response {
	t.Helper()
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestAnthropicResponseToOpenAIPassesThroughOpenAIShape(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"claude-sonnet","choices":[{"index":0,"message":{"role":"assistant","content":"Hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	out, err := anthropicResponseToOpenAI(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected passthrough, got: %s", out)
	}
	prompt, completion, total, _ := usage(out)
	if prompt != 10 || completion != 5 || total != 15 {
		t.Fatalf("unexpected usage: %d %d %d", prompt, completion, total)
	}
}

func TestOpenAIToAnthropicPassesThroughAnthropicShape(t *testing.T) {
	body := []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet","content":[{"type":"text","text":"Hello world"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
	out, err := openAIToAnthropic(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected passthrough, got: %s", out)
	}
	prompt, completion, total, _ := usage(out)
	if prompt != 10 || completion != 5 || total != 15 {
		t.Fatalf("unexpected usage: %d %d %d", prompt, completion, total)
	}
}

func TestStreamAnthropicToOpenAIPassesThroughOpenAIChunks(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	rec := httptest.NewRecorder()
	if _, err := streamAnthropicToOpenAI(rec, streamBody(t, sse), ""); err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "Hello") || strings.Count(out, "data: [DONE]") != 1 {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestStreamAnthropicToOpenAIConvertsAnthropicEvents(t *testing.T) {
	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"role\":\"assistant\",\"model\":\"claude-sonnet\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":12}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	rec := httptest.NewRecorder()
	st, err := streamAnthropicToOpenAI(rec, streamBody(t, sse), "")
	if err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, `"content":"Hello"`) || !strings.Contains(out, `"finish_reason":"stop"`) || !strings.Contains(out, "data: [DONE]") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if st.prompt != 10 || st.completion != 12 {
		t.Fatalf("streamStats = {prompt:%d completion:%d}, want {10 12}", st.prompt, st.completion)
	}
}

func TestStreamOpenAIToAnthropicPassesThroughAnthropicEvents(t *testing.T) {
	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"role\":\"assistant\",\"model\":\"claude-sonnet\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	rec := httptest.NewRecorder()
	if _, err := streamOpenAIToAnthropic(rec, streamBody(t, sse)); err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "message_stop") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestStreamOpenAIToAnthropicConvertsOpenAIChunks(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	rec := httptest.NewRecorder()
	st, err := streamOpenAIToAnthropic(rec, streamBody(t, sse))
	if err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, `"text":"Hello"`) || !strings.Contains(out, "message_stop") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if st.prompt != 8 || st.completion != 5 {
		t.Fatalf("streamStats = {prompt:%d completion:%d}, want {8 5}", st.prompt, st.completion)
	}
}
