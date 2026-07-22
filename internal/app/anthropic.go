package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      json.RawMessage    `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	Stop        json.RawMessage    `json:"stop_sequences"`
	Tools       []anthropicTool    `json:"tools"`
	ToolChoice  map[string]any     `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func (s *Service) anthropicMessages(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.Header.Get("Anthropic-Version")) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "anthropic-version header is required")
		return
	}
	var in anthropicRequest
	if decode(r, &in) != nil || in.Model == "" || in.MaxTokens <= 0 || len(in.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "model, messages, and max_tokens are required")
		return
	}
	body, err := anthropicToOpenAI(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	s.proxyChatCompletions(w, r, body, in.Model, in.Stream, openAIToAnthropic, streamOpenAIToAnthropic)
}

func anthropicToOpenAI(in anthropicRequest) ([]byte, error) {
	payload := map[string]any{"model": in.Model, "max_tokens": in.MaxTokens, "stream": in.Stream}
	messages := make([]any, 0, len(in.Messages)+1)
	if len(in.System) > 0 && string(in.System) != "null" {
		content, err := anthropicContentToOpenAI(in.System)
		if err != nil {
			return nil, fmt.Errorf("invalid system content")
		}
		messages = append(messages, map[string]any{"role": "system", "content": content})
	}
	for _, message := range in.Messages {
		converted, err := convertAnthropicMessage(message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, converted...)
	}
	payload["messages"] = messages
	if in.Temperature != nil {
		payload["temperature"] = *in.Temperature
	}
	if in.TopP != nil {
		payload["top_p"] = *in.TopP
	}
	if len(in.Stop) > 0 && string(in.Stop) != "null" {
		var stop any
		if json.Unmarshal(in.Stop, &stop) == nil {
			payload["stop"] = stop
		}
	}
	if len(in.Tools) > 0 {
		tools := make([]any, 0, len(in.Tools))
		for _, tool := range in.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.InputSchema}})
		}
		payload["tools"] = tools
	}
	if kind, _ := in.ToolChoice["type"].(string); kind != "" {
		switch kind {
		case "auto":
			payload["tool_choice"] = "auto"
		case "any":
			payload["tool_choice"] = "required"
		case "tool":
			payload["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": in.ToolChoice["name"]}}
		}
	}
	return json.Marshal(payload)
}

func convertAnthropicMessage(message anthropicMessage) ([]any, error) {
	var text string
	if json.Unmarshal(message.Content, &text) == nil {
		return []any{map[string]any{"role": message.Role, "content": text}}, nil
	}
	var blocks []map[string]any
	if json.Unmarshal(message.Content, &blocks) != nil {
		return nil, fmt.Errorf("invalid message content")
	}
	content := []any{}
	toolCalls := []any{}
	toolResults := []any{}
	for _, block := range blocks {
		switch block["type"] {
		case "text":
			content = append(content, map[string]any{"type": "text", "text": block["text"]})
		case "image":
			source, _ := block["source"].(map[string]any)
			media, _ := source["media_type"].(string)
			data, _ := source["data"].(string)
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + media + ";base64," + data}})
		case "tool_use":
			args, _ := json.Marshal(block["input"])
			toolCalls = append(toolCalls, map[string]any{"id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": string(args)}})
		case "tool_result":
			value := block["content"]
			if value == nil {
				value = ""
			}
			toolResults = append(toolResults, map[string]any{"role": "tool", "tool_call_id": block["tool_use_id"], "content": value})
		}
	}
	result := toolResults
	if len(content) > 0 || len(toolCalls) > 0 {
		entry := map[string]any{"role": message.Role, "content": content}
		if len(toolCalls) > 0 {
			entry["tool_calls"] = toolCalls
		}
		result = append(result, entry)
	}
	return result, nil
}

func anthropicContentToOpenAI(raw json.RawMessage) (any, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var blocks []map[string]any
	if json.Unmarshal(raw, &blocks) != nil {
		return nil, fmt.Errorf("invalid content")
	}
	parts := []any{}
	for _, block := range blocks {
		if block["type"] == "text" {
			parts = append(parts, map[string]any{"type": "text", "text": block["text"]})
		}
	}
	return parts, nil
}

func openAIToAnthropic(body []byte) ([]byte, error) {
	var response struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content   any `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Input  int `json:"prompt_tokens"`
			Output int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &response) != nil || len(response.Choices) == 0 {
		return nil, fmt.Errorf("invalid upstream response")
	}
	content := []any{}
	if text, ok := response.Choices[0].Message.Content.(string); ok && text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	for _, call := range response.Choices[0].Message.ToolCalls {
		var input any = map[string]any{}
		_ = json.Unmarshal([]byte(call.Function.Arguments), &input)
		content = append(content, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": input})
	}
	stop := map[string]string{"stop": "end_turn", "length": "max_tokens", "tool_calls": "tool_use"}[response.Choices[0].Finish]
	return json.Marshal(map[string]any{"id": response.ID, "type": "message", "role": "assistant", "model": response.Model, "content": content, "stop_reason": stop, "stop_sequence": nil, "usage": map[string]int{"input_tokens": response.Usage.Input, "output_tokens": response.Usage.Output}})
}

func streamOpenAIToAnthropic(w http.ResponseWriter, resp *http.Response) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)
	writeEvent := func(name string, value any) {
		data, _ := json.Marshal(value)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		flusher.Flush()
	}
	started := false
	nextBlock := 0
	textBlock := -1
	textStopped := false
	toolBlocks := map[int]int{}
	toolOrder := []int{}
	stopReason := "end_turn"
	var outputTokens int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			ID, Model string
			Choices   []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int                              `json:"index"`
						ID       string                           `json:"id"`
						Function struct{ Name, Arguments string } `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				Finish string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				Input  int `json:"prompt_tokens"`
				Output int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if !started {
			writeEvent("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": chunk.ID, "type": "message", "role": "assistant", "model": chunk.Model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]int{"input_tokens": chunk.Usage.Input, "output_tokens": 0}}})
			started = true
		}
		if chunk.Usage.Output > 0 {
			outputTokens = chunk.Usage.Output
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			if textBlock < 0 {
				textBlock = nextBlock
				nextBlock++
				writeEvent("content_block_start", map[string]any{"type": "content_block_start", "index": textBlock, "content_block": map[string]any{"type": "text", "text": ""}})
			}
			writeEvent("content_block_delta", map[string]any{"type": "content_block_delta", "index": textBlock, "delta": map[string]any{"type": "text_delta", "text": choice.Delta.Content}})
		}
		for _, call := range choice.Delta.ToolCalls {
			block, exists := toolBlocks[call.Index]
			if !exists {
				if textBlock >= 0 && !textStopped {
					writeEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": textBlock})
					textStopped = true
				}
				block = nextBlock
				nextBlock++
				toolBlocks[call.Index] = block
				toolOrder = append(toolOrder, call.Index)
				writeEvent("content_block_start", map[string]any{"type": "content_block_start", "index": block, "content_block": map[string]any{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": map[string]any{}}})
			}
			if call.Function.Arguments != "" {
				writeEvent("content_block_delta", map[string]any{"type": "content_block_delta", "index": block, "delta": map[string]any{"type": "input_json_delta", "partial_json": call.Function.Arguments}})
			}
		}
		if choice.Finish != "" {
			if mapped := map[string]string{"stop": "end_turn", "length": "max_tokens", "tool_calls": "tool_use"}[choice.Finish]; mapped != "" {
				stopReason = mapped
			}
		}
	}
	if textBlock >= 0 && !textStopped {
		writeEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": textBlock})
	}
	for _, index := range toolOrder {
		writeEvent("content_block_stop", map[string]any{"type": "content_block_stop", "index": toolBlocks[index]})
	}
	writeEvent("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil}, "usage": map[string]int{"output_tokens": outputTokens}})
	writeEvent("message_stop", map[string]string{"type": "message_stop"})
	return scanner.Err()
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func openAIRequestToAnthropic(body []byte) ([]byte, error) {
	var in map[string]any
	if json.Unmarshal(body, &in) != nil {
		return nil, fmt.Errorf("invalid OpenAI request")
	}
	out := map[string]any{"model": in["model"], "max_tokens": 4096}
	for _, key := range []string{"max_tokens", "stream", "temperature", "top_p"} {
		if value, ok := in[key]; ok {
			out[key] = value
		}
	}
	if stop, ok := in["stop"]; ok {
		if value, stringValue := stop.(string); stringValue {
			out["stop_sequences"] = []string{value}
		} else {
			out["stop_sequences"] = stop
		}
	}
	if tools, ok := in["tools"].([]any); ok {
		converted := make([]any, 0, len(tools))
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			function, _ := tool["function"].(map[string]any)
			converted = append(converted, map[string]any{"name": function["name"], "description": function["description"], "input_schema": function["parameters"]})
		}
		out["tools"] = converted
	}
	if choice, ok := in["tool_choice"]; ok {
		switch value := choice.(type) {
		case string:
			out["tool_choice"] = map[string]any{"type": map[string]string{"required": "any", "auto": "auto"}[value]}
		case map[string]any:
			function, _ := value["function"].(map[string]any)
			out["tool_choice"] = map[string]any{"type": "tool", "name": function["name"]}
		}
	}
	messages, ok := in["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}
	converted := []any{}
	for _, item := range messages {
		message, _ := item.(map[string]any)
		role, _ := message["role"].(string)
		if role == "system" {
			out["system"] = message["content"]
			continue
		}
		var blocks []any
		if content, isText := message["content"].(string); isText && content != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": content})
		} else if parts, isParts := message["content"].([]any); isParts {
			for _, partValue := range parts {
				part, _ := partValue.(map[string]any)
				switch part["type"] {
				case "text":
					blocks = append(blocks, map[string]any{"type": "text", "text": part["text"]})
				case "image_url":
					image, _ := part["image_url"].(map[string]any)
					urlValue, _ := image["url"].(string)
					mediaType, data, found := strings.Cut(strings.TrimPrefix(urlValue, "data:"), ";base64,")
					if found {
						blocks = append(blocks, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mediaType, "data": data}})
					}
				}
			}
		}
		if calls, ok := message["tool_calls"].([]any); ok {
			for _, callValue := range calls {
				call, _ := callValue.(map[string]any)
				function, _ := call["function"].(map[string]any)
				var input any = map[string]any{}
				if arguments, _ := function["arguments"].(string); arguments != "" {
					_ = json.Unmarshal([]byte(arguments), &input)
				}
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": input})
			}
		}
		if role == "tool" {
			blocks = []any{map[string]any{"type": "tool_result", "tool_use_id": message["tool_call_id"], "content": message["content"]}}
			role = "user"
		}
		if len(blocks) == 0 {
			blocks = append(blocks, map[string]any{"type": "text", "text": ""})
		}
		entry := map[string]any{"role": role, "content": blocks}
		if len(converted) > 0 {
			previous := converted[len(converted)-1].(map[string]any)
			if previous["role"] == role {
				previous["content"] = append(previous["content"].([]any), blocks...)
				continue
			}
		}
		converted = append(converted, entry)
	}
	out["messages"] = converted
	return json.Marshal(out)
}

func anthropicResponseToOpenAI(body []byte) ([]byte, error) {
	var in struct {
		ID         string           `json:"id"`
		Model      string           `json:"model"`
		StopReason string           `json:"stop_reason"`
		Content    []map[string]any `json:"content"`
		Usage      struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &in) != nil || in.ID == "" {
		return nil, fmt.Errorf("invalid Anthropic response")
	}
	text := ""
	toolCalls := []any{}
	for _, block := range in.Content {
		switch block["type"] {
		case "text":
			value, _ := block["text"].(string)
			text += value
		case "tool_use":
			arguments, _ := json.Marshal(block["input"])
			toolCalls = append(toolCalls, map[string]any{"id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": string(arguments)}})
		}
	}
	message := map[string]any{"role": "assistant", "content": text}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	finish := map[string]string{"end_turn": "stop", "stop_sequence": "stop", "max_tokens": "length", "tool_use": "tool_calls"}[in.StopReason]
	return json.Marshal(map[string]any{"id": in.ID, "object": "chat.completion", "created": 0, "model": in.Model, "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": in.Usage.Input, "completion_tokens": in.Usage.Output, "total_tokens": in.Usage.Input + in.Usage.Output}})
}

func streamAnthropicToOpenAI(w http.ResponseWriter, resp *http.Response) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)
	writeChunk := func(value any) {
		data, _ := json.Marshal(value)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	id, model := "", ""
	toolIndexes := map[int]int{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event map[string]any
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		switch event["type"] {
		case "message_start":
			message, _ := event["message"].(map[string]any)
			id, _ = message["id"].(string)
			model, _ = message["model"].(string)
			writeChunk(map[string]any{"id": id, "object": "chat.completion.chunk", "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}}})
		case "content_block_start":
			block, _ := event["content_block"].(map[string]any)
			if block["type"] != "tool_use" {
				continue
			}
			blockIndex := int(event["index"].(float64))
			toolIndex := len(toolIndexes)
			toolIndexes[blockIndex] = toolIndex
			writeChunk(map[string]any{"id": id, "object": "chat.completion.chunk", "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": toolIndex, "id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": ""}}}}, "finish_reason": nil}}})
		case "content_block_delta":
			delta, _ := event["delta"].(map[string]any)
			value := map[string]any{}
			if delta["type"] == "text_delta" {
				value["content"] = delta["text"]
			} else if delta["type"] == "input_json_delta" {
				blockIndex := int(event["index"].(float64))
				value["tool_calls"] = []any{map[string]any{"index": toolIndexes[blockIndex], "function": map[string]any{"arguments": delta["partial_json"]}}}
			} else {
				continue
			}
			writeChunk(map[string]any{"id": id, "object": "chat.completion.chunk", "model": model, "choices": []any{map[string]any{"index": 0, "delta": value, "finish_reason": nil}}})
		case "message_delta":
			delta, _ := event["delta"].(map[string]any)
			stop, _ := delta["stop_reason"].(string)
			finish := map[string]string{"end_turn": "stop", "stop_sequence": "stop", "max_tokens": "length", "tool_use": "tool_calls"}[stop]
			writeChunk(map[string]any{"id": id, "object": "chat.completion.chunk", "model": model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}})
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
	return scanner.Err()
}
