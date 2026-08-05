package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// responsesCompletions accepts OpenAI Responses API requests (POST /v1/responses)
// and proxies them through the chat-completions pipeline: the request is
// converted to the chat format every channel speaks, and the upstream response
// (chat-completions or Anthropic) is converted back to the Responses shape.
func (s *Service) responsesCompletions(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		s.logReject(r.Context(), "", 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "could not read request")
		return
	}
	var request struct {
		Model           string `json:"model"`
		Stream          bool   `json:"stream"`
		MaxOutputTokens int    `json:"max_output_tokens"`
	}
	if json.Unmarshal(body, &request) != nil {
		s.logReject(r.Context(), "", 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if !validModelName(request.Model) {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "model must be 1-200 characters")
		return
	}
	if request.MaxOutputTokens > maxGatewayMaxTokens {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "max_output_tokens must be at most 200000")
		return
	}
	converted, echo, err := responsesRequestToChatCompletions(body)
	if err != nil {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	responseID := "resp_" + randomIDString()
	s.proxyChatCompletions(w, r, converted, request.Model, request.Stream, request.MaxOutputTokens,
		func(responseBody []byte) ([]byte, error) { return chatCompletionsToResponses(responseBody, responseID, echo) },
		func(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
			return streamChatCompletionsToResponses(w, resp, responseID, echo)
		})
}

// responsesEcho carries the request fields a Responses response object echoes
// back, matching the shape documented for the Responses API (defaults included).
type responsesEcho struct {
	model              string
	instructions       any
	maxOutputTokens    any
	previousResponseID any
	store              bool
	temperature        any
	topP               any
	text               any
	reasoning          map[string]any
	metadata           map[string]any
	user               any
	parallelToolCalls  bool
	toolChoice         any
	tools              []any
	truncation         string
}

// responsesRequestToChatCompletions converts an OpenAI Responses request into a
// chat-completions request, plus the request fields to echo back in the
// response object. instructions becomes the system message; input strings,
// messages, function_call and function_call_output items map to their chat
// counterparts. Fields with no chat equivalent (store, previous_response_id,
// truncation, ...) are dropped from the upstream payload but still echoed.
func responsesRequestToChatCompletions(body []byte) ([]byte, responsesEcho, error) {
	var in map[string]any
	if json.Unmarshal(body, &in) != nil {
		return nil, responsesEcho{}, fmt.Errorf("invalid OpenAI Responses request")
	}
	model, _ := in["model"].(string)
	echo := responsesEcho{model: model, store: true, parallelToolCalls: true, truncation: "disabled"}
	out := map[string]any{"model": model}
	if stream, ok := in["stream"].(bool); ok {
		out["stream"] = stream
	}
	if maxOutput, ok := in["max_output_tokens"].(float64); ok && maxOutput > 0 {
		out["max_tokens"] = int(maxOutput)
		echo.maxOutputTokens = int(maxOutput)
	}
	if temperature, ok := in["temperature"].(float64); ok {
		out["temperature"] = temperature
		echo.temperature = temperature
	}
	if topP, ok := in["top_p"].(float64); ok {
		out["top_p"] = topP
		echo.topP = topP
	}
	if stop, ok := in["stop"]; ok {
		out["stop"] = stop
	}
	if parallel, ok := in["parallel_tool_calls"].(bool); ok {
		out["parallel_tool_calls"] = parallel
		echo.parallelToolCalls = parallel
	}
	if reasoning, ok := in["reasoning"].(map[string]any); ok {
		effort, _ := reasoning["effort"].(string)
		summary, _ := reasoning["summary"].(string)
		if effort != "" {
			out["reasoning_effort"] = effort
		}
		echo.reasoning = map[string]any{"effort": nil, "summary": nil}
		if effort != "" {
			echo.reasoning["effort"] = effort
		}
		if summary != "" {
			echo.reasoning["summary"] = summary
		}
	}
	if tools, ok := in["tools"].([]any); ok {
		converted := []any{}
		echoTools := []any{}
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			if tool == nil {
				continue
			}
			if tool["type"] == "function" {
				function := map[string]any{
					"name":        tool["name"],
					"description": tool["description"],
					"parameters":  tool["parameters"],
				}
				if strict, ok := tool["strict"].(bool); ok {
					function["strict"] = strict
				}
				converted = append(converted, map[string]any{"type": "function", "function": function})
				echoTool := map[string]any{}
				for key, value := range tool {
					echoTool[key] = value
				}
				if _, ok := echoTool["strict"]; !ok {
					echoTool["strict"] = true
				}
				echoTools = append(echoTools, echoTool)
			} else {
				echoTools = append(echoTools, tool)
			}
		}
		if len(converted) > 0 {
			out["tools"] = converted
		}
		echo.tools = echoTools
	}
	if raw, ok := in["tool_choice"]; ok {
		echo.toolChoice = raw
		switch choice := raw.(type) {
		case string:
			out["tool_choice"] = choice
		case map[string]any:
			if choice["type"] == "function" {
				out["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": choice["name"]}}
			}
		}
	}
	if text, ok := in["text"].(map[string]any); ok {
		echo.text = text
		if format, ok := text["format"].(map[string]any); ok {
			switch format["type"] {
			case "json_object":
				out["response_format"] = map[string]any{"type": "json_object"}
			case "json_schema":
				if schema, ok := format["schema"].(map[string]any); ok {
					out["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "response", "schema": schema}}
				}
			}
		}
	}
	messages := []any{}
	if instructions, ok := in["instructions"].(string); ok {
		echo.instructions = instructions
		if strings.TrimSpace(instructions) != "" {
			messages = append(messages, map[string]any{"role": "system", "content": instructions})
		}
	}
	if id, ok := in["previous_response_id"].(string); ok && id != "" {
		echo.previousResponseID = id
	}
	if store, ok := in["store"].(bool); ok {
		echo.store = store
	}
	if metadata, ok := in["metadata"].(map[string]any); ok && len(metadata) > 0 {
		echo.metadata = metadata
	}
	if user, ok := in["user"].(string); ok && user != "" {
		echo.user = user
	}
	if truncation, ok := in["truncation"].(string); ok {
		echo.truncation = truncation
	}
	input, ok := in["input"]
	if !ok || input == nil {
		return nil, responsesEcho{}, fmt.Errorf("input is required")
	}
	switch value := input.(type) {
	case string:
		if value == "" {
			return nil, responsesEcho{}, fmt.Errorf("input must not be empty")
		}
		messages = append(messages, map[string]any{"role": "user", "content": value})
	case []any:
		if len(value) == 0 {
			return nil, responsesEcho{}, fmt.Errorf("input must not be empty")
		}
		for _, item := range value {
			if message := responsesInputItem(item); message != nil {
				messages = append(messages, message)
			}
		}
		if len(messages) == 0 {
			return nil, responsesEcho{}, fmt.Errorf("input must not be empty")
		}
		messages = mergeToolCallMessages(messages)
	default:
		return nil, responsesEcho{}, fmt.Errorf("input must be a string or an array of items")
	}
	out["messages"] = messages
	converted, err := json.Marshal(out)
	return converted, echo, err
}

// responsesInputItem maps one element of a Responses input array to a chat
// message, or nil when the item has no chat equivalent.
func responsesInputItem(item any) map[string]any {
	switch value := item.(type) {
	case string:
		if value == "" {
			return nil
		}
		return map[string]any{"role": "user", "content": value}
	case map[string]any:
		if role, ok := value["role"].(string); ok {
			return map[string]any{"role": validChatRole(role), "content": responsesContent(value["content"])}
		}
		switch value["type"] {
		case "message":
			role, _ := value["role"].(string)
			return map[string]any{"role": validChatRole(role), "content": responsesContent(value["content"])}
		case "function_call":
			return map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{map[string]any{
					"id":       value["call_id"],
					"type":     "function",
					"function": map[string]any{"name": value["name"], "arguments": value["arguments"]},
				}},
			}
		case "function_call_output":
			return map[string]any{"role": "tool", "tool_call_id": value["call_id"], "content": responsesOutputText(value["output"])}
		}
	}
	return nil
}

func validChatRole(role string) string {
	switch role {
	case "system", "developer", "user", "assistant":
		return role
	}
	return "user"
}

// responsesContent converts Responses content (string or input_text/input_image/
// input_audio parts) into chat-completions content.
func responsesContent(content any) any {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []any:
		parts := []any{}
		for _, item := range value {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch part["type"] {
			case "text", "input_text", "output_text":
				if text, ok := part["text"].(string); ok && text != "" {
					parts = append(parts, map[string]any{"type": "text", "text": text})
				}
			case "input_image":
				urlValue, ok := part["image_url"]
				if !ok {
					continue
				}
				var url any
				if raw, isString := urlValue.(string); isString {
					url = map[string]any{"url": raw}
				} else {
					url = urlValue
				}
				image := map[string]any{"type": "image_url", "image_url": url}
				if detail, ok := part["detail"].(string); ok {
					if imageURL, ok := image["image_url"].(map[string]any); ok {
						imageURL["detail"] = detail
					}
				}
				parts = append(parts, image)
			case "input_audio":
				parts = append(parts, map[string]any{"type": "input_audio", "data": part["data"], "format": part["format"]})
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return parts
	}
	return ""
}

// responsesOutputText flattens a function_call_output payload (string or an
// array of output items) into the text a tool message carries.
func responsesOutputText(output any) any {
	switch value := output.(type) {
	case nil:
		return ""
	case string:
		return value
	case []any:
		var b strings.Builder
		for _, item := range value {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	}
	return ""
}

// mergeToolCallMessages folds consecutive assistant tool-call messages into one,
// which strict upstreams require for repeated function_call items.
func mergeToolCallMessages(messages []any) []any {
	merged := make([]any, 0, len(messages))
	for _, message := range messages {
		current, _ := message.(map[string]any)
		if len(merged) > 0 {
			last, _ := merged[len(merged)-1].(map[string]any)
			if last["role"] == "assistant" && current["role"] == "assistant" {
				lastCalls, ok1 := last["tool_calls"].([]any)
				calls, ok2 := current["tool_calls"].([]any)
				if ok1 && ok2 && len(lastCalls) > 0 {
					last["tool_calls"] = append(lastCalls, calls...)
					continue
				}
			}
		}
		merged = append(merged, message)
	}
	return merged
}

// chatCompletionsToResponses converts a chat-completions response body into the
// OpenAI Responses object shape.
func chatCompletionsToResponses(body []byte, responseID string, echo responsesEcho) ([]byte, error) {
	var in struct {
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
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Prompt              int `json:"prompt_tokens"`
			Completion          int `json:"completion_tokens"`
			Total               int `json:"total_tokens"`
			PromptTokensDetails struct {
				Cached int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokensDetails struct {
				Reasoning int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &in) != nil || len(in.Choices) == 0 {
		return nil, fmt.Errorf("invalid upstream response")
	}
	choice := in.Choices[0]
	output := []any{}
	if text := openAIMessageText(choice.Message.Content); text != "" {
		output = append(output, map[string]any{
			"id":      "msg_" + randomIDString(),
			"type":    "message",
			"status":  "completed",
			"role":    "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
		})
	}
	for _, call := range choice.Message.ToolCalls {
		output = append(output, map[string]any{
			"id":        "fc_" + randomIDString(),
			"type":      "function_call",
			"status":    "completed",
			"call_id":   call.ID,
			"name":      call.Function.Name,
			"arguments": call.Function.Arguments,
		})
	}
	status := "completed"
	switch choice.FinishReason {
	case "length":
		status = "incomplete"
	case "content_filter":
		status = "failed"
	}
	usage := map[string]any{
		"input_tokens":          in.Usage.Prompt,
		"output_tokens":         in.Usage.Completion,
		"total_tokens":          in.Usage.Prompt + in.Usage.Completion,
		"input_tokens_details":  map[string]any{"cached_tokens": in.Usage.PromptTokensDetails.Cached},
		"output_tokens_details": map[string]any{"reasoning_tokens": in.Usage.CompletionTokensDetails.Reasoning},
	}
	if in.Usage.Total > 0 {
		usage["total_tokens"] = in.Usage.Total
	}
	return json.Marshal(responseObject(responseID, in.Model, status, output, usage, echo))
}

// responseObject builds the top-level Responses object shared by every
// conversion path. Fields the client set in the request are echoed back;
// otherwise the documented Responses defaults apply.
func responseObject(id, model, status string, output []any, usage map[string]any, echo responsesEcho) map[string]any {
	reasoning := echo.reasoning
	if reasoning == nil {
		reasoning = map[string]any{"effort": nil, "summary": nil}
	}
	tools := echo.tools
	if tools == nil {
		tools = []any{}
	}
	toolChoice := echo.toolChoice
	if toolChoice == nil {
		toolChoice = "auto"
	}
	text := echo.text
	if text == nil {
		text = map[string]any{"format": map[string]any{"type": "text"}}
	}
	temperature := echo.temperature
	if temperature == nil {
		temperature = 1.0
	}
	topP := echo.topP
	if topP == nil {
		topP = 1.0
	}
	metadata := echo.metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	truncation := echo.truncation
	if truncation == "" {
		truncation = "disabled"
	}
	obj := map[string]any{
		"id":                   id,
		"object":               "response",
		"created_at":           time.Now().Unix(),
		"status":               status,
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         echo.instructions,
		"max_output_tokens":    echo.maxOutputTokens,
		"model":                model,
		"output":               output,
		"parallel_tool_calls":  echo.parallelToolCalls,
		"previous_response_id": echo.previousResponseID,
		"reasoning":            reasoning,
		"store":                echo.store,
		"temperature":          temperature,
		"text":                 text,
		"tool_choice":          toolChoice,
		"tools":                tools,
		"top_p":                topP,
		"truncation":           truncation,
		"usage":                usage,
		"user":                 echo.user,
		"metadata":             metadata,
	}
	if status == "incomplete" {
		obj["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	return obj
}

// responsesStream converts an upstream SSE stream (OpenAI chat-completions
// chunks or Anthropic events) into OpenAI Responses streaming events.
type responsesStream struct {
	responseID   string
	echo         responsesEcho
	model        string
	output       []any
	outputIndex  int
	msgItemID    string
	textOutput   int
	textBlock    int
	textOpen     bool
	msgText      strings.Builder
	tools        map[int]*responsesTool
	toolOrder    []int
	status       string
	inputTokens  int
	cached       int
	outputTokens int
	started      bool
	finished     bool
}

type responsesTool struct {
	itemID      string
	callID      string
	name        string
	arguments   strings.Builder
	outputIndex int
	closed      bool
}

func (s *responsesStream) nextOutputIndex() int {
	index := s.outputIndex
	s.outputIndex++
	return index
}

// object builds the response snapshot carried by an event. usage stays null
// until the response reaches its terminal state, matching the documented
// streaming shape.
func (s *responsesStream) object(status string, includeUsage bool) map[string]any {
	var usage any
	if includeUsage {
		usage = map[string]any{
			"input_tokens":          s.inputTokens,
			"output_tokens":         s.outputTokens,
			"total_tokens":          s.inputTokens + s.outputTokens,
			"input_tokens_details":  map[string]any{"cached_tokens": s.cached},
			"output_tokens_details": map[string]any{"reasoning_tokens": 0},
		}
	}
	return responseObject(s.responseID, s.model, status, s.output, usage, s.echo)
}

func (s *responsesStream) start(st *streamStats, emit func(string, any)) {
	s.inputTokens = st.prompt
	s.cached = st.cached
	s.started = true
	object := s.object("in_progress", false)
	emit("response.created", map[string]any{"type": "response.created", "response": object})
	emit("response.in_progress", map[string]any{"type": "response.in_progress", "response": object})
}

func (s *responsesStream) finish(st *streamStats, emit func(string, any)) {
	if s.finished {
		return
	}
	s.inputTokens = st.prompt
	s.outputTokens = st.completion
	s.cached = st.cached
	event := "response.completed"
	switch s.status {
	case "incomplete":
		event = "response.incomplete"
	case "failed":
		event = "response.failed"
	}
	emit(event, map[string]any{"type": event, "response": s.object(s.status, true)})
	s.finished = true
}

// startTextItem opens an assistant message item plus its first content part.
func (s *responsesStream) startTextItem(emit func(string, any)) {
	if s.textOpen {
		return
	}
	s.msgItemID = "msg_" + randomIDString()
	s.textOutput = s.nextOutputIndex()
	s.textOpen = true
	emit("response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"output_index": s.textOutput,
		"item":         map[string]any{"id": s.msgItemID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}},
	})
	emit("response.content_part.added", map[string]any{
		"type":          "response.content_part.added",
		"item_id":       s.msgItemID,
		"output_index":  s.textOutput,
		"content_index": 0,
		"part":          map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
	})
}

func (s *responsesStream) closeTextItem(emit func(string, any)) {
	if !s.textOpen {
		return
	}
	text := s.msgText.String()
	part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
	emit("response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       s.msgItemID,
		"output_index":  s.textOutput,
		"content_index": 0,
		"text":          text,
	})
	emit("response.content_part.done", map[string]any{
		"type":          "response.content_part.done",
		"item_id":       s.msgItemID,
		"output_index":  s.textOutput,
		"content_index": 0,
		"part":          part,
	})
	emit("response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"output_index": s.textOutput,
		"item":         map[string]any{"id": s.msgItemID, "type": "message", "status": "completed", "role": "assistant", "content": []any{part}},
	})
	s.output = append(s.output, map[string]any{"id": s.msgItemID, "type": "message", "status": "completed", "role": "assistant", "content": []any{part}})
	s.textOpen = false
}

func (s *responsesStream) startToolItem(tool *responsesTool, emit func(string, any)) {
	tool.outputIndex = s.nextOutputIndex()
	emit("response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"output_index": tool.outputIndex,
		"item":         map[string]any{"id": tool.itemID, "type": "function_call", "status": "in_progress", "call_id": tool.callID, "name": tool.name, "arguments": ""},
	})
}

func (s *responsesStream) closeToolItem(tool *responsesTool, emit func(string, any)) {
	if tool.closed {
		return
	}
	arguments := tool.arguments.String()
	emit("response.function_call_arguments.done", map[string]any{
		"type":         "response.function_call_arguments.done",
		"item_id":      tool.itemID,
		"output_index": tool.outputIndex,
		"arguments":    arguments,
	})
	emit("response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"output_index": tool.outputIndex,
		"item":         map[string]any{"id": tool.itemID, "type": "function_call", "status": "completed", "call_id": tool.callID, "name": tool.name, "arguments": arguments},
	})
	s.output = append(s.output, map[string]any{"id": tool.itemID, "type": "function_call", "status": "completed", "call_id": tool.callID, "name": tool.name, "arguments": arguments})
	tool.closed = true
}

func (s *responsesStream) closeAllItems(emit func(string, any)) {
	s.closeTextItem(emit)
	for _, index := range s.toolOrder {
		s.closeToolItem(s.tools[index], emit)
	}
}

// chatEvent consumes one OpenAI chat-completions SSE chunk.
func (s *responsesStream) chatEvent(data string, st *streamStats, emit func(string, any)) {
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
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal([]byte(data), &chunk) != nil {
		return
	}
	if !s.started {
		s.model = chunk.Model
		s.start(st, emit)
	}
	if len(chunk.Choices) == 0 {
		// Trailing usage-only chunk; nothing left to relay.
		return
	}
	choice := chunk.Choices[0]
	if choice.Delta.Content != "" {
		s.startTextItem(emit)
		s.msgText.WriteString(choice.Delta.Content)
		emit("response.output_text.delta", map[string]any{
			"type":          "response.output_text.delta",
			"item_id":       s.msgItemID,
			"output_index":  s.textOutput,
			"content_index": 0,
			"delta":         choice.Delta.Content,
		})
	}
	for _, call := range choice.Delta.ToolCalls {
		tool := s.tools[call.Index]
		if tool == nil {
			tool = &responsesTool{itemID: "fc_" + randomIDString(), callID: call.ID, name: call.Function.Name}
			s.tools[call.Index] = tool
			s.toolOrder = append(s.toolOrder, call.Index)
			s.startToolItem(tool, emit)
		}
		if call.Function.Arguments != "" {
			tool.arguments.WriteString(call.Function.Arguments)
			emit("response.function_call_arguments.delta", map[string]any{
				"type":         "response.function_call_arguments.delta",
				"item_id":      tool.itemID,
				"output_index": tool.outputIndex,
				"delta":        call.Function.Arguments,
			})
		}
	}
	if choice.FinishReason != "" {
		switch choice.FinishReason {
		case "length":
			s.status = "incomplete"
		case "content_filter":
			s.status = "failed"
		}
		s.closeAllItems(emit)
	}
}

// anthropicEvent consumes one Anthropic SSE event.
func (s *responsesStream) anthropicEvent(data string, st *streamStats, emit func(string, any)) {
	var event map[string]any
	if json.Unmarshal([]byte(data), &event) != nil {
		return
	}
	index := func() int {
		value, _ := event["index"].(float64)
		return int(value)
	}
	switch event["type"] {
	case "message_start":
		message, _ := event["message"].(map[string]any)
		s.model, _ = message["model"].(string)
		s.start(st, emit)
	case "content_block_start":
		contentBlock, _ := event["content_block"].(map[string]any)
		switch contentBlock["type"] {
		case "text":
			s.textBlock = index()
			s.startTextItem(emit)
		case "tool_use":
			id, _ := contentBlock["id"].(string)
			name, _ := contentBlock["name"].(string)
			tool := &responsesTool{itemID: "fc_" + randomIDString(), callID: id, name: name}
			s.tools[index()] = tool
			s.toolOrder = append(s.toolOrder, index())
			s.startToolItem(tool, emit)
		}
	case "content_block_delta":
		delta, _ := event["delta"].(map[string]any)
		switch delta["type"] {
		case "text_delta":
			text, _ := delta["text"].(string)
			s.msgText.WriteString(text)
			emit("response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       s.msgItemID,
				"output_index":  s.textOutput,
				"content_index": 0,
				"delta":         text,
			})
		case "input_json_delta":
			if tool := s.tools[index()]; tool != nil {
				partial, _ := delta["partial_json"].(string)
				tool.arguments.WriteString(partial)
				emit("response.function_call_arguments.delta", map[string]any{
					"type":         "response.function_call_arguments.delta",
					"item_id":      tool.itemID,
					"output_index": tool.outputIndex,
					"delta":        partial,
				})
			}
		}
	case "content_block_stop":
		if blockIndex := index(); blockIndex == s.textBlock {
			s.closeTextItem(emit)
		} else if tool := s.tools[blockIndex]; tool != nil {
			s.closeToolItem(tool, emit)
		}
	case "message_delta":
		delta, _ := event["delta"].(map[string]any)
		if reason, _ := delta["stop_reason"].(string); reason != "" {
			switch reason {
			case "max_tokens":
				s.status = "incomplete"
			case "refusal", "error":
				s.status = "failed"
			}
		}
	}
}

// streamChatCompletionsToResponses relays an upstream SSE stream to the client
// as Responses events, converting both chat-completions chunks and Anthropic
// events (for channels routed through the Anthropic adapter).
func streamChatCompletionsToResponses(w http.ResponseWriter, resp *http.Response, responseID string, echo responsesEcho) (streamStats, error) {
	var st streamStats
	flusher, ok := w.(http.Flusher)
	if !ok {
		return st, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	emit := func(name string, value any) {
		data, _ := json.Marshal(value)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		flusher.Flush()
	}
	stream := &responsesStream{responseID: responseID, echo: echo, status: "completed", tools: map[int]*responsesTool{}}
	decided, chatMode := false, true
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		parseSSEUsage([]byte(data), &st)
		if !decided {
			var probe map[string]any
			if json.Unmarshal([]byte(data), &probe) != nil {
				continue
			}
			decided = true
			if _, isChat := probe["choices"]; isChat {
				chatMode = true
			} else if kind, _ := probe["type"].(string); kind != "" {
				chatMode = false
			} else {
				chatMode = true
			}
		}
		if chatMode {
			stream.chatEvent(data, &st, emit)
		} else {
			stream.anthropicEvent(data, &st, emit)
		}
		if stream.finished {
			break
		}
	}
	if !stream.finished && stream.started {
		stream.closeAllItems(emit)
		stream.finish(&st, emit)
	}
	return st, scanner.Err()
}
