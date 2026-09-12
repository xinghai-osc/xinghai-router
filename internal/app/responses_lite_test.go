package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponsesLiteNormalization(t *testing.T) {
	body, changed, err := normalizeResponsesLite([]byte(`{"model":"gpt-5.1","input":"hello","tools":[{"type":"namespace","name":"shell","tools":[{"type":"function","name":"exec"}]}]}`))
	if err != nil || !changed {
		t.Fatalf("normalize = %s changed=%v err=%v", body, changed, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["parallel_tool_calls"] != false {
		t.Fatalf("parallel_tool_calls = %#v", payload["parallel_tool_calls"])
	}
	reasoning := payload["reasoning"].(map[string]any)
	if reasoning["context"] != "all_turns" {
		t.Fatalf("reasoning = %#v", reasoning)
	}
	if _, ok := payload["tools"]; ok {
		t.Fatalf("namespace tools remained top-level: %s", body)
	}
	input := payload["input"].([]any)
	additional := input[1].(map[string]any)
	if additional["type"] != "additional_tools" || additional["role"] != "developer" {
		t.Fatalf("additional_tools = %#v", additional)
	}
}

func TestResponsesLiteToolsAreReturnedAfterNormalization(t *testing.T) {
	request := []byte(`{
		"model":"gpt-5.1",
		"input":"hello",
		"tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"namespace","name":"shell","tools":[{"type":"function","name":"exec"}]}
		]
	}`)
	normalized, changed, err := normalizeResponsesLite(request)
	if err != nil || !changed {
		t.Fatalf("normalize = %s changed=%v err=%v", normalized, changed, err)
	}
	_, echo, err := responsesRequestToChatCompletions(normalized)
	if err != nil {
		t.Fatal(err)
	}
	response, err := chatCompletionsToResponses(
		[]byte(`{"model":"gpt-5.1","choices":[{"message":{"role":"assistant","content":"ok"}}]}`),
		"resp_lite",
		echo,
	)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(response, &payload); err != nil {
		t.Fatal(err)
	}
	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) != 2 {
		t.Fatalf("response tools = %#v, want function and namespace", payload["tools"])
	}
	function, _ := tools[0].(map[string]any)
	namespace, _ := tools[1].(map[string]any)
	if function["type"] != "function" || function["name"] != "lookup" || function["strict"] != true {
		t.Fatalf("function tool = %#v", function)
	}
	if namespace["type"] != "namespace" || namespace["name"] != "shell" {
		t.Fatalf("namespace tool = %#v", namespace)
	}
	children, _ := namespace["tools"].([]any)
	if len(children) != 1 || children[0].(map[string]any)["name"] != "exec" {
		t.Fatalf("namespace children = %#v", namespace["tools"])
	}
}

func TestResponsesLiteNamespaceMigrationRetainsClientToolOrderInEcho(t *testing.T) {
	request := []byte(`{"model":"m","input":"hello","tools":[{"type":"namespace","name":"shell"},{"type":"function","name":"lookup"}]}`)
	normalized, _, err := normalizeResponsesLite(request)
	if err != nil {
		t.Fatal(err)
	}
	_, echo, err := responsesRequestToChatCompletions(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if len(echo.tools) != 2 {
		t.Fatalf("echo tools = %#v", echo.tools)
	}
	if echo.tools[0].(map[string]any)["type"] != "function" || echo.tools[1].(map[string]any)["type"] != "namespace" {
		t.Fatalf("normalized tool order = %#v; namespace migration intentionally places additional tools after top-level tools", echo.tools)
	}
}

func TestResponsesAdditionalToolsAreEchoedWithoutWrapper(t *testing.T) {
	_, echo, err := responsesRequestToChatCompletions([]byte(`{
		"model":"m",
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"additional_tools","role":"developer","tools":[{"type":"namespace","name":"mcp"}]}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(echo.tools) != 1 {
		t.Fatalf("echo tools = %#v, want one namespace tool", echo.tools)
	}
	tool, _ := echo.tools[0].(map[string]any)
	if tool["type"] != "namespace" || tool["name"] != "mcp" {
		t.Fatalf("echo tool = %#v", tool)
	}
}

func TestResponsesLiteRejectsUnsupportedToolAndInvalidParallel(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","input":"x","tools":[{"type":"web_search"}]}`,
		`{"model":"m","input":"x","parallel_tool_calls":"false"}`,
		`{"model":"m","input":"x","tools":"function"}`,
	} {
		if _, _, err := normalizeResponsesLite([]byte(body)); err == nil {
			t.Fatalf("expected rejection for %s", body)
		}
	}
}

func TestResponsesLiteRejectsEmptyInputAfterNamespaceMigration(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","tools":[{"type":"namespace","name":"shell"}]}`,
		`{"model":"m","input":"","tools":[{"type":"namespace","name":"shell"}]}`,
	} {
		normalized, _, err := normalizeResponsesLite([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := responsesRequestToChatCompletions(normalized); err == nil {
			t.Fatalf("expected empty input rejection for %s; normalized=%s", body, normalized)
		}
	}
}

func TestResponsesLiteRejectsMalformedAdditionalTools(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","input":[{"type":"message","role":"user","content":"x"},{"type":"additional_tools","tools":"function"}],"tools":[{"type":"namespace","name":"shell"}]}`,
	} {
		if _, _, err := normalizeResponsesLite([]byte(body)); err == nil {
			t.Fatalf("expected malformed additional_tools.tools to be rejected for %s", body)
		}
	}
}

func TestResponsesLiteRejectsNonObjectTool(t *testing.T) {
	if _, _, err := normalizeResponsesLite([]byte(`{"model":"m","input":"x","tools":["custom shorthand"]}`)); err == nil {
		t.Fatal("expected non-object Lite tool to be rejected")
	}
}

func TestResponsesLiteToolTypeWhitespaceIsConverted(t *testing.T) {
	normalized, _, err := normalizeResponsesLite([]byte(`{"model":"m","input":"x","tools":[{"type":" function ","name":"lookup"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	converted, _, err := responsesRequestToChatCompletions(normalized)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(converted, &payload); err != nil {
		t.Fatal(err)
	}
	tools, _ := payload["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["type"] != "function" {
		t.Fatalf("converted tools = %#v", payload["tools"])
	}
}

func TestResponsesLiteMarkers(t *testing.T) {
	if !isResponsesLiteHeader(" TRUE ") {
		t.Fatal("header marker not recognized")
	}
	if !isResponsesLiteBody([]byte(`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"}}`)) {
		t.Fatal("websocket marker not recognized")
	}
	if isResponsesLiteBody([]byte(`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"false"}}`)) {
		t.Fatal("false marker recognized")
	}
}

func TestNormalizeResponsesWSRequest(t *testing.T) {
	first, model, err := normalizeResponsesWSRequest([]byte(`{"model":"gpt-5.1","input":"hello"}`), "", true)
	if err != nil || model != "gpt-5.1" || !strings.Contains(string(first), `"stream":true`) || strings.Contains(string(first), `"type"`) {
		t.Fatalf("first = %s model=%s err=%v", first, model, err)
	}
	headerLite, _, err := normalizeResponsesWSRequest([]byte(`{"model":"gpt-5.1","input":"hello","tools":[{"type":"namespace","name":"shell"}]}`), "", true, true)
	if err != nil || strings.Contains(string(headerLite), `"tools":[{"type":"namespace"`) || !strings.Contains(string(headerLite), `"additional_tools"`) {
		t.Fatalf("header-only Lite normalization = %s err=%v", headerLite, err)
	}
	marker := isResponsesLiteBody([]byte(`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"}}`))
	followUp, _, err := normalizeResponsesWSRequest([]byte(`{"type":"response.create","model":"gpt-5.1","input":"next","tools":[{"type":"namespace","name":"shell"}]}`), model, false, marker)
	if err != nil || !strings.Contains(string(followUp), `"additional_tools"`) {
		t.Fatalf("follow-up Lite normalization = %s err=%v", followUp, err)
	}
	second, model, err := normalizeResponsesWSRequest([]byte(`{"type":"response.create","input":"next","previous_response_id":"resp_1"}`), model, false)
	if err != nil || !strings.Contains(string(second), `"model":"gpt-5.1"`) || model != "gpt-5.1" {
		t.Fatalf("second = %s model=%s err=%v", second, model, err)
	}
	if _, _, err := normalizeResponsesWSRequest([]byte(`{"type":"response.append"}`), model, false); err == nil {
		t.Fatal("response.append must be rejected")
	}
	if _, _, err := normalizeResponsesWSRequest([]byte(`{"type":"response.create","previous_response_id":"msg_1"}`), model, false); err == nil {
		t.Fatal("message previous_response_id must be rejected")
	}
}
