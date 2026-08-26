package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

const responsesLiteHeader = "X-OpenAI-Internal-Codex-Responses-Lite"

func isResponsesLiteHeader(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func isResponsesLiteBody(body []byte) bool {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	metadata, _ := payload["client_metadata"].(map[string]any)
	return isResponsesLiteHeader(firstString(metadata["ws_request_header_x_openai_internal_codex_responses_lite"]))
}

func firstString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// normalizeResponsesLite applies the compatibility rules used by Sub2Api for
// Responses Lite. It deliberately rejects unknown hosted tool types instead of
// silently dropping them, because dropping a tool changes the request meaning.
func normalizeResponsesLite(body []byte) ([]byte, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
		return body, false, fmt.Errorf("responses Lite request must be a JSON object")
	}
	changed := false
	if reasoning, ok := payload["reasoning"]; ok && reasoning != nil {
		obj, ok := reasoning.(map[string]any)
		if !ok {
			return body, false, fmt.Errorf("responses Lite requires reasoning to be an object")
		}
		if _, ok := obj["context"]; !ok || firstString(obj["context"]) != "all_turns" {
			obj["context"] = "all_turns"
			changed = true
		}
	} else {
		payload["reasoning"] = map[string]any{"context": "all_turns"}
		changed = true
	}
	if parallel, ok := payload["parallel_tool_calls"]; ok {
		if _, valid := parallel.(bool); !valid {
			return body, false, fmt.Errorf("responses Lite requires parallel_tool_calls to be a boolean")
		}
		if parallel.(bool) {
			payload["parallel_tool_calls"] = false
			changed = true
		}
	} else {
		payload["parallel_tool_calls"] = false
		changed = true
	}
	tools, ok := payload["tools"].([]any)
	if ok {
		top := make([]any, 0, len(tools))
		namespaces := make([]any, 0)
		for i, raw := range tools {
			tool, isObject := raw.(map[string]any)
			if !isObject {
				if strings.TrimSpace(firstString(raw)) == "" {
					return body, false, fmt.Errorf("responses Lite tool at index %d is invalid", i)
				}
				top = append(top, raw)
				continue
			}
			switch strings.TrimSpace(firstString(tool["type"])) {
			case "function", "custom", "tool_search":
				top = append(top, raw)
			case "namespace":
				namespaces = append(namespaces, raw)
			case "":
				return body, false, fmt.Errorf("responses Lite tool at index %d is missing type", i)
			default:
				return body, false, fmt.Errorf("responses Lite does not support tool type %q", firstString(tool["type"]))
			}
		}
		if len(namespaces) > 0 {
			input, err := liteInputItems(payload["input"])
			if err != nil {
				return body, false, err
			}
			input = appendLiteAdditionalTools(input, namespaces)
			payload["input"] = input
			changed = true
		}
		if len(top) == 0 {
			delete(payload, "tools")
		} else {
			payload["tools"] = top
		}
	}
	if !changed {
		return body, false, nil
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body, false, fmt.Errorf("encode responses Lite request: %w", err)
	}
	return out, true, nil
}

func liteInputItems(input any) ([]any, error) {
	switch value := input.(type) {
	case nil:
		return []any{}, nil
	case string:
		return []any{map[string]any{"type": "message", "role": "user", "content": value}}, nil
	case []any:
		return value, nil
	default:
		return nil, fmt.Errorf("responses Lite input must be a string or array")
	}
}

func appendLiteAdditionalTools(items, namespaces []any) []any {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok || firstString(obj["type"]) != "additional_tools" {
			continue
		}
		current, _ := obj["tools"].([]any)
		obj["tools"] = append(current, namespaces...)
		return items
	}
	return append(items, map[string]any{"type": "additional_tools", "role": "developer", "tools": namespaces})
}
