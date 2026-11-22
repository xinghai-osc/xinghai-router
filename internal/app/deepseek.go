package app

import (
	"bytes"
	"encoding/json"
	"strconv"
)

const deepSeekChatPath = "/chat/completions"

func deepSeekChatUpstream(provider, format string) bool {
	return provider == "deepseek" && (format == "openai" || format == "openai_chat")
}

func adaptDeepSeekChatRequest(body []byte, toDeepSeek bool) []byte {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil || payload == nil {
		return body
	}
	effort, _ := payload["reasoning_effort"].(string)
	thinking, _ := payload["thinking"].(map[string]any)
	mode, _ := thinking["type"].(string)
	if toDeepSeek {
		switch effort {
		case "minimal":
			effort = "low"
		case "medium", "xhigh":
			effort = "high"
		}
		if thinking == nil && effort != "" {
			mode = "enabled"
			if effort == "none" {
				mode = "disabled"
			}
			payload["thinking"] = map[string]string{"type": mode}
		}
		if limit, ok := payload["max_completion_tokens"]; ok {
			payload["max_tokens"] = limit
			delete(payload, "max_completion_tokens")
		}
	} else {
		switch mode {
		case "disabled":
			effort = "none"
		case "enabled":
			if effort == "" || effort == "none" {
				effort = "high"
			}
		}
		if effort == "max" {
			effort = "xhigh"
		}
		delete(payload, "thinking")
		if limit, ok := payload["max_tokens"]; ok {
			if _, exists := payload["max_completion_tokens"]; !exists {
				payload["max_completion_tokens"] = limit
			}
			delete(payload, "max_tokens")
		}
	}
	if effort != "" {
		payload["reasoning_effort"] = effort
	}
	messages, _ := payload["messages"].([]any)
	for _, item := range messages {
		message, _ := item.(map[string]any)
		role, _ := message["role"].(string)
		if !toDeepSeek {
			if role == "assistant" {
				delete(message, "reasoning_content")
			}
			continue
		}
		if role == "developer" {
			message["role"] = "system"
			role = "system"
		}
		if role == "system" || role == "assistant" {
			if parts, ok := message["content"].([]any); ok {
				var content bytes.Buffer
				for _, part := range parts {
					block, _ := part.(map[string]any)
					text, ok := block["text"].(string)
					if block["type"] != "text" || !ok {
						break
					}
					content.WriteString(text)
				}
				if len(parts) == 0 || content.Len() > 0 {
					message["content"] = content.String()
				}
			}
		}
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

func deepSeekChatUsage(body []byte, nativeClient bool) []byte {
	if !mayHaveSSEUsage(body) {
		return body
	}
	var root usageObject
	if json.Unmarshal(body, &root) != nil {
		return body
	}
	u := root.object("usage")
	details := u.object("prompt_tokens_details")
	cached, hasCached := details.count("cached_tokens")
	hit, hasHit := u.count("prompt_cache_hit_tokens")
	if !hasCached && !hasHit {
		return body
	}
	changed := false
	if !hasCached {
		cached = hit
		if details == nil {
			details = usageObject{}
		}
		details["cached_tokens"] = json.RawMessage(strconv.FormatInt(cached, 10))
		u["prompt_tokens_details"], _ = json.Marshal(details)
		changed = true
	}
	if nativeClient {
		if !hasHit {
			u["prompt_cache_hit_tokens"] = json.RawMessage(strconv.FormatInt(cached, 10))
			changed = true
		}
		if prompt, ok := u.count("prompt_tokens"); ok && u["prompt_cache_miss_tokens"] == nil {
			u["prompt_cache_miss_tokens"] = json.RawMessage(strconv.FormatInt(max(0, prompt-cached), 10))
			changed = true
		}
	}
	if !changed {
		return body
	}
	root["usage"], _ = json.Marshal(u)
	out, _ := json.Marshal(root)
	return out
}
