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

func TestResponsesLiteRejectsUnsupportedToolAndInvalidParallel(t *testing.T) {
	for _, body := range []string{
		`{"model":"m","input":"x","tools":[{"type":"web_search"}]}`,
		`{"model":"m","input":"x","parallel_tool_calls":"false"}`,
	} {
		if _, _, err := normalizeResponsesLite([]byte(body)); err == nil {
			t.Fatalf("expected rejection for %s", body)
		}
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
