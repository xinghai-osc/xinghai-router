package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicToOpenAITools(t *testing.T) {
	in := anthropicRequest{
		Model:     "kimi-k2.6",
		System:    json.RawMessage(`"Be concise"`),
		MaxTokens: 256,
		Messages: []anthropicMessage{
			{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"call_1","name":"weather","input":{"city":"Beijing"}}]`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"call_1","content":"sunny"}]`)},
		},
		Tools: []anthropicTool{{Name: "weather", InputSchema: map[string]any{"type": "object"}}},
	}
	body, err := anthropicToOpenAI(in)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	messages := payload["messages"].([]any)
	if len(messages) != 3 || messages[0].(map[string]any)["role"] != "system" || messages[2].(map[string]any)["role"] != "tool" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	toolCalls := messages[1].(map[string]any)["tool_calls"].([]any)
	if toolCalls[0].(map[string]any)["id"] != "call_1" {
		t.Fatalf("unexpected tool call: %#v", toolCalls)
	}
}

func TestOpenAIToAnthropic(t *testing.T) {
	body, err := openAIToAnthropic([]byte(`{"id":"chat_1","model":"kimi-k2.6","choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","function":{"name":"weather","arguments":"{\"city\":\"Beijing\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":4}}`))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response["stop_reason"] != "tool_use" || response["type"] != "message" {
		t.Fatalf("unexpected response: %#v", response)
	}
	block := response["content"].([]any)[0].(map[string]any)
	if block["type"] != "tool_use" || block["name"] != "weather" {
		t.Fatalf("unexpected content block: %#v", block)
	}
}

func TestOpenAIToAnthropicArrayContent(t *testing.T) {
	body, err := openAIToAnthropic([]byte(`{"id":"chat_1","model":"kimi-k2.6","choices":[{"message":{"content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	content := response["content"].([]any)
	if len(content) != 1 || content[0].(map[string]any)["text"] != "hello world" {
		t.Fatalf("unexpected content: %#v", content)
	}
}

func TestOpenAIToAnthropicDefaultsStopReason(t *testing.T) {
	body, err := openAIToAnthropic([]byte(`{"id":"chat_1","model":"kimi-k2.6","choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response["stop_reason"] != "end_turn" {
		t.Fatalf("unexpected stop_reason: %#v", response["stop_reason"])
	}
}

func TestOpenAIRequestToAnthropic(t *testing.T) {
	body, prefill, err := openAIRequestToAnthropic([]byte(`{"model":"claude-sonnet","messages":[{"role":"system","content":"Be concise"},{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"Beijing\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"sunny"}],"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}],"max_tokens":256}`))
	if err != nil {
		t.Fatal(err)
	}
	if prefill != "" {
		t.Fatalf("unexpected prefill: %q", prefill)
	}
	var request map[string]any
	if err = json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if request["system"] != "Be concise" || request["max_tokens"].(float64) != 256 {
		t.Fatalf("unexpected request: %#v", request)
	}
	messages := request["messages"].([]any)
	if len(messages) != 2 || messages[0].(map[string]any)["role"] != "assistant" || messages[1].(map[string]any)["role"] != "user" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestOpenAIRequestToAnthropicResponseFormat(t *testing.T) {
	body, prefill, err := openAIRequestToAnthropic([]byte(`{"model":"claude-sonnet","messages":[{"role":"system","content":"Be concise"},{"role":"user","content":"list colors"}],"max_tokens":256,"response_format":{"type":"json_schema","json_schema":{"name":"colors","schema":{"type":"object","properties":{"items":{"type":"array"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if prefill != "{" {
		t.Fatalf("prefill = %q, want %q", prefill, "{")
	}
	var request map[string]any
	if err = json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	system, _ := request["system"].(string)
	if !strings.Contains(system, "Be concise") || !strings.Contains(system, `"type":"object"`) {
		t.Fatalf("unexpected system: %q", system)
	}
	messages := request["messages"].([]any)
	last := messages[len(messages)-1].(map[string]any)
	if last["role"] != "assistant" {
		t.Fatalf("expected assistant prefill message: %#v", messages)
	}
	blocks := last["content"].([]any)
	if blocks[0].(map[string]any)["text"] != "{" {
		t.Fatalf("unexpected prefill block: %#v", blocks)
	}
}

func TestOpenAIRequestToAnthropicResponseFormatMergesAssistantPrefill(t *testing.T) {
	body, prefill, err := openAIRequestToAnthropic([]byte(`{"model":"claude-sonnet","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"Working on it."}],"response_format":{"type":"json_object"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if prefill != "{" {
		t.Fatalf("prefill = %q, want %q", prefill, "{")
	}
	var request map[string]any
	if err = json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	messages := request["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected prefill merged into last assistant message: %#v", messages)
	}
	blocks := messages[1].(map[string]any)["content"].([]any)
	if len(blocks) != 2 || blocks[1].(map[string]any)["text"] != "{" {
		t.Fatalf("unexpected assistant blocks: %#v", blocks)
	}
}

func TestAnthropicResponseToOpenAIPrefill(t *testing.T) {
	body, err := anthropicResponseToOpenAI([]byte(`{"id":"msg_1","model":"claude-sonnet","content":[{"type":"text","text":"\"a\": 1}"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":4}}`), "{")
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	choice := response["choices"].([]any)[0].(map[string]any)
	content := choice["message"].(map[string]any)["content"]
	if content != `{"a": 1}` {
		t.Fatalf("unexpected content: %#v", content)
	}
}

func TestAnthropicResponseToOpenAI(t *testing.T) {
	body, err := anthropicResponseToOpenAI([]byte(`{"id":"msg_1","model":"claude-sonnet","content":[{"type":"tool_use","id":"tool_1","name":"weather","input":{"city":"Beijing"}}],"stop_reason":"tool_use","usage":{"input_tokens":9,"output_tokens":3}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	choice := response["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" || response["usage"].(map[string]any)["total_tokens"].(float64) != 12 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestAnthropicResponseToOpenAIDefaultsFinishReason(t *testing.T) {
	body, err := anthropicResponseToOpenAI([]byte(`{"id":"msg_1","model":"claude-sonnet","content":[{"type":"text","text":"hi"}],"stop_reason":"refusal","usage":{"input_tokens":5,"output_tokens":2}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	choice := response["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "stop" {
		t.Fatalf("unexpected finish_reason: %#v", choice["finish_reason"])
	}
}

func TestAnthropicAPIKeyAndLoopback(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req.Header.Set("X-API-Key", "sk-xh-test")
	if bearer(req) != "sk-xh-test" {
		t.Fatal("x-api-key was not accepted")
	}
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected %s to be loopback", host)
		}
	}
	if isLoopbackHost("ollama.example.com") {
		t.Fatal("unexpected loopback result")
	}
}

func TestAnthropicMessagesRequiresVersionHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: "1", keyID: "k"}))
	(&Service{}).anthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "anthropic-version") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAnthropicMessagesRejectsLongModel(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"` + strings.Repeat("m", 201) + `","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: "1", keyID: "k"}))
	(&Service{}).anthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAnthropicMessagesRejectsOversizeMaxTokens(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"claude","max_tokens":200001,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: "1", keyID: "k"}))
	(&Service{}).anthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "max_tokens") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAnthropicMessagesRejectsInvalidMaxTokensOrEmptyMessages(t *testing.T) {
	bodies := []string{
		`{"model":"claude","max_tokens":0,"messages":[{"role":"user","content":"hi"}]}`,
		`{"model":"claude","max_tokens":-1,"messages":[{"role":"user","content":"hi"}]}`,
		`{"model":"claude","max_tokens":16,"messages":[]}`,
		`{"model":"claude","max_tokens":16}`,
	}
	for _, body := range bodies {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Anthropic-Version", "2023-06-01")
		req = req.WithContext(context.WithValue(req.Context(), contextKey{}, keyContext{userID: "1", keyID: "k"}))
		(&Service{}).anthropicMessages(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rec.Body.String(), "invalid_request") {
			t.Fatalf("body %s response = %s", body, rec.Body.String())
		}
	}
}