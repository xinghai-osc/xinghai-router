package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesCompletionsRejectsInvalidBodyBeforeUpstream(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"model":""}`,
		`not-json`,
		`{"model":"m"}`,
		`{"model":"m","input":[]}`,
		`{"model":"m","input":""}`,
		`{"model":"m","input":"hi","max_output_tokens":200001}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
		(&Service{}).responsesCompletions(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestResponsesRequestToChatCompletions(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"instructions": "Be brief.",
		"input": [
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"Tokyo\"}"},
			{"type": "function_call", "call_id": "call_2", "name": "get_time", "arguments": "{\"city\":\"Tokyo\"}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "20c"},
			{"type": "message", "role": "assistant", "content": "Check the weather."}
		],
		"max_output_tokens": 100,
		"stream": true,
		"tools": [
			{"type": "function", "name": "get_weather", "description": "Weather", "parameters": {"type": "object"}, "strict": true},
			{"type": "web_search"}
		],
		"tool_choice": {"type": "function", "name": "get_weather"},
		"text": {"format": {"type": "json_object"}}
	}`)
	out, echo, err := responsesRequestToChatCompletions(body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    bool   `json:"stream"`
		Messages  []struct {
			Role      string `json:"role"`
			Content   any    `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
		Tools []map[string]any `json:"tools"`
	}
	if json.Unmarshal(out, &payload) != nil {
		t.Fatalf("converted body is not valid JSON: %s", out)
	}
	if payload.Model != "gpt-4o" || payload.MaxTokens != 100 || !payload.Stream {
		t.Fatalf("header fields lost: %s", out)
	}
	if len(payload.Messages) != 5 {
		t.Fatalf("messages = %d, want 5: %s", len(payload.Messages), out)
	}
	if payload.Messages[0].Role != "system" || payload.Messages[0].Content != "Be brief." {
		t.Fatalf("instructions not mapped to system message: %s", out)
	}
	if payload.Messages[1].Role != "user" {
		t.Fatalf("first user message role = %q", payload.Messages[1].Role)
	}
	text, ok := payload.Messages[1].Content.([]any)
	if !ok || len(text) != 1 || text[0].(map[string]any)["text"] != "Hello" {
		t.Fatalf("input_text not converted: %s", out)
	}
	// Two consecutive function_call items must merge into one assistant message.
	assistant := payload.Messages[2]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 2 {
		t.Fatalf("function_call items not merged: %s", out)
	}
	if assistant.ToolCalls[0].ID != "call_1" || assistant.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("tool call fields wrong: %s", out)
	}
	tool := payload.Messages[3]
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content != "20c" {
		t.Fatalf("function_call_output not mapped: %s", out)
	}
	if payload.Messages[4].Role != "assistant" || payload.Messages[4].Content != "Check the weather." {
		t.Fatalf("trailing assistant message wrong: %s", out)
	}
	if len(payload.Tools) != 1 {
		t.Fatalf("web_search tool should be dropped, got %d tools", len(payload.Tools))
	}
	function, _ := payload.Tools[0]["function"].(map[string]any)
	if function["strict"] != true {
		t.Fatalf("tool strict not passed through: %s", out)
	}
	var raw map[string]any
	_ = json.Unmarshal(out, &raw)
	if choiceMap, ok := raw["tool_choice"].(map[string]any); !ok || choiceMap["type"] != "function" {
		t.Fatalf("tool_choice not converted: %s", out)
	}
	if _, ok := raw["response_format"].(map[string]any); !ok {
		t.Fatalf("response_format not converted: %s", out)
	}
	if echo.instructions != "Be brief." || echo.maxOutputTokens != 100 {
		t.Fatalf("echo fields not captured: %+v", echo)
	}
	if len(echo.tools) != 2 || echo.tools[0].(map[string]any)["type"] != "function" {
		t.Fatalf("echo tools not captured: %+v", echo.tools)
	}
}

func TestResponsesRequestToStringInput(t *testing.T) {
	out, _, err := responsesRequestToChatCompletions([]byte(`{"model":"m","input":"hi there"}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(out, &payload) != nil || len(payload.Messages) != 1 {
		t.Fatalf("unexpected conversion: %s", out)
	}
	if payload.Messages[0].Role != "user" || payload.Messages[0].Content != "hi there" {
		t.Fatalf("string input not mapped: %s", out)
	}
}

func TestChatCompletionsToResponses(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"model": "gpt-4o",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Hello world",
				"tool_calls": [{
					"id": "call_x",
					"type": "function",
					"function": {"name": "get_weather", "arguments": "{\"city\":\"Tokyo\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15,
			"prompt_tokens_details": {"cached_tokens": 3}}
	}`)
	out, err := chatCompletionsToResponses(body, "resp_test", responsesEcho{})
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		ID     string `json:"id"`
		Object string `json:"object"`
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"output"`
		Usage struct {
			Input        int `json:"input_tokens"`
			Output       int `json:"output_tokens"`
			Total        int `json:"total_tokens"`
			InputDetails struct {
				Cached int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(out, &response) != nil {
		t.Fatalf("response body is not valid JSON: %s", out)
	}
	if response.ID != "resp_test" || response.Object != "response" || response.Status != "completed" || response.Model != "gpt-4o" {
		t.Fatalf("top-level fields wrong: %s", out)
	}
	if len(response.Output) != 2 {
		t.Fatalf("output items = %d, want 2: %s", len(response.Output), out)
	}
	if response.Output[0].Type != "message" || response.Output[0].Content[0].Text != "Hello world" {
		t.Fatalf("text item wrong: %s", out)
	}
	call := response.Output[1]
	if call.Type != "function_call" || call.CallID != "call_x" || call.Name != "get_weather" || call.Arguments != `{"city":"Tokyo"}` {
		t.Fatalf("function_call item wrong: %s", out)
	}
	if response.Usage.Input != 10 || response.Usage.Output != 5 || response.Usage.Total != 15 || response.Usage.InputDetails.Cached != 3 {
		t.Fatalf("usage wrong: %+v", response.Usage)
	}
}

func TestResponseObjectEcho(t *testing.T) {
	echo := responsesEcho{model: "gpt-4o", instructions: "Be brief", maxOutputTokens: 64, temperature: 0.5, topP: 0.9,
		store: false, parallelToolCalls: false, previousResponseID: "resp_prev", user: "u1",
		reasoning: map[string]any{"effort": "high", "summary": nil},
		tools:     []any{map[string]any{"type": "function", "name": "get_weather", "strict": true}},
	}
	out, err := json.Marshal(responseObject("resp_x", "gpt-4o", "completed", nil, nil, echo))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if json.Unmarshal(out, &response) != nil {
		t.Fatalf("response body is not valid JSON: %s", out)
	}
	for field, want := range map[string]any{
		"instructions":         "Be brief",
		"max_output_tokens":    float64(64),
		"temperature":          0.5,
		"top_p":                0.9,
		"store":                false,
		"parallel_tool_calls":  false,
		"previous_response_id": "resp_prev",
		"user":                 "u1",
		"truncation":           "disabled",
	} {
		if got := response[field]; got != want {
			t.Fatalf("response[%q] = %v, want %v", field, got, want)
		}
	}
	if effort := response["reasoning"].(map[string]any)["effort"]; effort != "high" {
		t.Fatalf("reasoning.effort = %v, want high", effort)
	}
	if tools := response["tools"].([]any); len(tools) != 1 || tools[0].(map[string]any)["strict"] != true {
		t.Fatalf("tools not echoed with strict default: %s", out)
	}
}

func TestChatCompletionsToResponsesIncomplete(t *testing.T) {
	body := []byte(`{"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"length"}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	out, err := chatCompletionsToResponses(body, "resp_test", responsesEcho{})
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Status           string `json:"status"`
		IncompleteDetail struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Output []any `json:"output"`
	}
	if json.Unmarshal(out, &response) != nil || response.Status != "incomplete" || len(response.Output) != 0 {
		t.Fatalf("unexpected conversion: %s", out)
	}
	if response.IncompleteDetail.Reason != "max_output_tokens" {
		t.Fatalf("incomplete_details = %+v, want max_output_tokens", response.IncompleteDetail)
	}
}

func TestStreamChatCompletionsToResponses(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":5,\"total_tokens\":13}}\n\n" +
		"data: [DONE]\n\n"
	rec := httptest.NewRecorder()
	st, err := streamChatCompletionsToResponses(rec, streamBody(t, sse), "resp_test", responsesEcho{
		instructions: "Be brief", temperature: 1.0, topP: 1.0, truncation: "disabled",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.created") ||
		!strings.Contains(out, "event: response.in_progress") ||
		!strings.Contains(out, `"usage":null`) ||
		!strings.Contains(out, `"instructions":"Be brief"`) ||
		!strings.Contains(out, `"truncation":"disabled"`) ||
		!strings.Contains(out, "event: response.output_item.added") ||
		!strings.Contains(out, "event: response.content_part.added") ||
		strings.Count(out, "event: response.output_text.delta") != 2 ||
		!strings.Contains(out, `"delta":"Hel"`) ||
		!strings.Contains(out, "event: response.output_text.done") ||
		!strings.Contains(out, "event: response.output_item.done") ||
		!strings.Contains(out, "event: response.completed") ||
		!strings.Contains(out, `"text":"Hello"`) {
		t.Fatalf("missing events in output:\n%s", out)
	}
	if st.prompt != 8 || st.completion != 5 {
		t.Fatalf("streamStats = {prompt:%d completion:%d}, want {8 5}", st.prompt, st.completion)
	}
	if strings.Contains(out, "[DONE]") {
		t.Fatalf("responses streams must not relay [DONE]:\n%s", out)
	}
}

func TestStreamChatCompletionsToResponsesToolCalls(t *testing.T) {
	sse := "data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"city\\\":\"}}]},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"Tokyo\\\"}\"}}]},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"model\":\"gpt-4o\",\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	rec := httptest.NewRecorder()
	if _, err := streamChatCompletionsToResponses(rec, streamBody(t, sse), "resp_test", responsesEcho{}); err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if strings.Count(out, "event: response.function_call_arguments.delta") != 2 ||
		!strings.Contains(out, "event: response.function_call_arguments.done") ||
		!strings.Contains(out, `"call_id":"call_1"`) ||
		!strings.Contains(out, `"name":"get_weather"`) ||
		!strings.Contains(out, `"arguments":"{\"city\":\"Tokyo\"}"`) {
		t.Fatalf("tool call stream missing pieces:\n%s", out)
	}
	if !strings.Contains(out, `"type":"function_call"`) {
		t.Fatalf("function_call item missing:\n%s", out)
	}
}

func TestStreamChatCompletionsToResponsesAnthropicUpstream(t *testing.T) {
	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"role\":\"assistant\",\"model\":\"claude-sonnet\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"get_time\",\"input\":{}}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"tz\\\":\\\"UTC\\\"}\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":1}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":12}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"
	rec := httptest.NewRecorder()
	st, err := streamChatCompletionsToResponses(rec, streamBody(t, sse), "resp_test", responsesEcho{})
	if err != nil {
		t.Fatal(err)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.created") ||
		!strings.Contains(out, `"model":"claude-sonnet"`) ||
		!strings.Contains(out, `"text":"Hi"`) ||
		!strings.Contains(out, `"call_id":"toolu_1"`) ||
		!strings.Contains(out, `"name":"get_time"`) ||
		!strings.Contains(out, `"arguments":"{\"tz\":\"UTC\"}"`) ||
		!strings.Contains(out, "event: response.completed") {
		t.Fatalf("anthropic conversion incomplete:\n%s", out)
	}
	if st.prompt != 10 || st.completion != 12 {
		t.Fatalf("streamStats = {prompt:%d completion:%d}, want {10 12}", st.prompt, st.completion)
	}
}

func TestResponsesInputItemImages(t *testing.T) {
	out, _, err := responsesRequestToChatCompletions([]byte(`{
		"model": "gpt-4o",
		"input": [{
			"type": "message",
			"role": "user",
			"content": [
				{"type": "input_text", "text": "What is this?"},
				{"type": "input_image", "image_url": "https://example.com/a.png", "detail": "high"},
				{"type": "input_audio", "data": "AAA", "format": "wav"}
			]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Messages []struct {
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(out, &payload) != nil || len(payload.Messages) != 1 {
		t.Fatalf("unexpected conversion: %s", out)
	}
	parts := payload.Messages[0].Content
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3: %s", len(parts), out)
	}
	image := parts[1]["image_url"].(map[string]any)
	if image["url"] != "https://example.com/a.png" || image["detail"] != "high" {
		t.Fatalf("image part wrong: %s", out)
	}
	if parts[2]["type"] != "input_audio" {
		t.Fatalf("audio part wrong: %s", out)
	}
}
