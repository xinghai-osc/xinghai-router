package app

import (
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

func TestOpenAIRequestToAnthropic(t *testing.T) {
	body, err := openAIRequestToAnthropic([]byte(`{"model":"claude-sonnet","messages":[{"role":"system","content":"Be concise"},{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"weather","arguments":"{\"city\":\"Beijing\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"sunny"}],"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}],"max_tokens":256}`))
	if err != nil {
		t.Fatal(err)
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

func TestAnthropicResponseToOpenAI(t *testing.T) {
	body, err := anthropicResponseToOpenAI([]byte(`{"id":"msg_1","model":"claude-sonnet","content":[{"type":"tool_use","id":"tool_1","name":"weather","input":{"city":"Beijing"}}],"stop_reason":"tool_use","usage":{"input_tokens":9,"output_tokens":3}}`))
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

func TestAnthropicMessagesRejectsOversizeMaxTokens(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"model":"claude","max_tokens":200001,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	(&Service{}).anthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "max_tokens") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}
