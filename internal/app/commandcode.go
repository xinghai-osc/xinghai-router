package app

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Command Code adapter: routes OpenAI-compatible chat-completions requests to
// the Command Code /alpha/generate streaming endpoint. The request body is
// rebuilt into the Command Code config/params shape, and the Command Code SSE
// event stream is converted back into OpenAI chat-completions SSE chunks.

const (
	commandCodeDefaultMaxTokens    = 64_000
	commandCodeDefaultTemperature  = 0.3
	commandCodeCLIVersion          = "1.15.1"
	commandCodeGeneratePath        = "/alpha/generate"
	commandCodeModelsPath          = "/provider/v1/models"
	commandCodeProjectSlugFallback = "project"
)

// commandCodeWorkingDir is the working directory reported in the /alpha/generate
// config block; it also seeds the x-project-slug header. Resolved once at
// startup so every request reports the same project context.
var commandCodeWorkingDir = func() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return dir
}()

// commandCodeReasoningEfforts mirrors the per-model reasoning efforts supported
// by the Command Code generate endpoint. A client reasoning_effort is forwarded
// only when the selected model accepts that level; omitted models let Command
// Code choose the reasoning depth.
var commandCodeReasoningEfforts = map[string]map[string]bool{
	"Qwen/Qwen3.8-Max":             {"low": true, "medium": true, "xhigh": true},
	"claude-fable-5":               {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude-opus-4-7":              {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude-opus-4-8":              {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude-opus-5":                {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude-sonnet-4-6":            {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"claude-sonnet-5":              {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"deepseek/deepseek-v4-flash":   {"high": true, "max": true},
	"deepseek/deepseek-v4-pro":     {"high": true, "max": true},
	"gpt-5.3-codex":                {"low": true, "medium": true, "high": true, "xhigh": true},
	"gpt-5.4":                      {"low": true, "medium": true, "high": true, "xhigh": true},
	"gpt-5.4-mini":                 {"low": true, "medium": true, "high": true},
	"gpt-5.5":                      {"low": true, "medium": true, "high": true, "xhigh": true},
	"gpt-5.6-luna":                 {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"gpt-5.6-sol":                  {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"gpt-5.6-terra":                {"low": true, "medium": true, "high": true, "xhigh": true, "max": true},
	"google/gemini-3.1-flash-lite": {"low": true, "medium": true, "high": true},
	"google/gemini-3.5-flash":      {"low": true, "medium": true, "high": true},
	"google/gemini-3.5-flash-lite": {"low": true, "medium": true, "high": true},
	"google/gemini-3.6-flash":      {"low": true, "medium": true, "high": true},
	"sakana/fugu-ultra":            {"high": true, "xhigh": true},
	"xai/grok-4.5":                 {"low": true, "medium": true, "high": true},
	"zai-org/GLM-5.2":              {"high": true, "max": true},
}

// commandCodeReasoningEffort returns the effort level to send for a model when
// the requested level is supported, and false otherwise. Levels the model does
// not advertise are dropped so Command Code picks its own depth.
func commandCodeReasoningEffort(model string, raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var effort string
	if json.Unmarshal(raw, &effort) != nil {
		return "", false
	}
	effort = strings.TrimSpace(effort)
	if effort == "" || effort == "off" {
		return "", false
	}
	if commandCodeReasoningEfforts[model][effort] {
		return effort, true
	}
	return "", false
}

// commandCodeProjectSlug derives a URL-safe slug from a working directory so
// the x-project-slug header matches what the Command Code CLI would send.
func commandCodeProjectSlug(path string) string {
	slug := strings.ToLower(strings.TrimSpace(path))
	parts := strings.FieldsFunc(slug, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	slug = strings.Trim(strings.Join(parts, "-"), "-")
	if slug == "" {
		return commandCodeProjectSlugFallback
	}
	return slug
}

// commandCodeNewID returns a random lowercase hex identifier of 24 characters
// (OpenAI-style) for synthesized chat-completion ids.
func commandCodeNewID() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// commandCodeNewThreadID returns a random UUID-v4-formatted thread identifier
// for one /alpha/generate request.
func commandCodeNewThreadID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func commandCodeEnvironment() string {
	return "linux-x64, Go gateway"
}

// commandCodeMessage represents one Command Code wire message.
type commandCodeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// pairedCommandCodeToolCallIDs returns the tool-call ids that have a matching
// tool result in the conversation; only those calls are sent upstream, matching
// the Command Code converter's pairing rule.
func pairedCommandCodeToolCallIDs(messages []any) map[string]bool {
	callIDs := map[string]bool{}
	resultIDs := map[string]bool{}
	for _, item := range messages {
		message, _ := item.(map[string]any)
		role, _ := message["role"].(string)
		if role == "assistant" {
			if calls, ok := message["tool_calls"].([]any); ok {
				for _, callValue := range calls {
					call, _ := callValue.(map[string]any)
					if id, _ := call["id"].(string); id != "" {
						callIDs[id] = true
					}
				}
			}
		} else if role == "tool" {
			if id, _ := message["tool_call_id"].(string); id != "" {
				resultIDs[id] = true
			}
		}
	}
	paired := map[string]bool{}
	for id := range callIDs {
		if resultIDs[id] {
			paired[id] = true
		}
	}
	return paired
}

// commandCodeTextContent flattens OpenAI content parts into plain text.
func commandCodeTextContent(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		var builder strings.Builder
		for _, partValue := range value {
			part, _ := partValue.(map[string]any)
			if part["type"] == "text" {
				text, _ := part["text"].(string)
				builder.WriteString(text)
			}
		}
		return builder.String()
	}
	return ""
}

// commandCodeContentToCC converts OpenAI message content to the Command Code
// content vocabulary. Images become base64 data URLs when present.
func commandCodeContentToCC(content any) any {
	switch value := content.(type) {
	case string:
		if value == "" {
			return []any{}
		}
		return value
	case []any:
		parts := []any{}
		for _, partValue := range value {
			part, _ := partValue.(map[string]any)
			switch part["type"] {
			case "text":
				text, _ := part["text"].(string)
				if text != "" {
					parts = append(parts, map[string]any{"type": "text", "text": text})
				}
			case "image_url":
				image, _ := part["image_url"].(map[string]any)
				urlValue, _ := image["url"].(string)
				mediaType, data, found := strings.Cut(strings.TrimPrefix(urlValue, "data:"), ";base64,")
				if found {
					parts = append(parts, map[string]any{"type": "image", "image": "data:" + mediaType + ";base64," + data, "mimeType": mediaType})
				}
			}
		}
		if len(parts) == 0 {
			return []any{}
		}
		return parts
	}
	return []any{}
}

// commandCodeMessagesToCC converts an OpenAI messages array to the Command Code
// wire vocabulary. System messages are returned separately so the caller can
// place them in params.system.
func commandCodeMessagesToCC(messages []any) (system string, converted []any) {
	paired := pairedCommandCodeToolCallIDs(messages)
	for _, item := range messages {
		message, _ := item.(map[string]any)
		role, _ := message["role"].(string)
		switch role {
		case "system":
			system += commandCodeTextContent(message["content"])
		case "user":
			content := commandCodeContentToCC(message["content"])
			if len(asAnySlice(content)) == 0 && asString(content) == "" {
				continue
			}
			converted = append(converted, commandCodeMessage{Role: "user", Content: content})
		case "assistant":
			parts := []any{}
			if text := commandCodeTextContent(message["content"]); text != "" {
				parts = append(parts, map[string]any{"type": "text", "text": text})
			}
			if calls, ok := message["tool_calls"].([]any); ok {
				for _, callValue := range calls {
					call, _ := callValue.(map[string]any)
					id, _ := call["id"].(string)
					if !paired[id] {
						continue
					}
					function, _ := call["function"].(map[string]any)
					name, _ := function["name"].(string)
					var input any = map[string]any{}
					if arguments, _ := function["arguments"].(string); arguments != "" {
						_ = json.Unmarshal([]byte(arguments), &input)
					}
					parts = append(parts, map[string]any{"type": "tool-call", "toolCallId": id, "toolName": name, "input": input})
				}
			}
			if len(parts) > 0 {
				converted = append(converted, commandCodeMessage{Role: "assistant", Content: parts})
			}
		case "tool":
			id, _ := message["tool_call_id"].(string)
			if !paired[id] {
				continue
			}
			converted = append(converted, commandCodeMessage{Role: "tool", Content: []any{map[string]any{
				"type":       "tool-result",
				"toolCallId": id,
				"toolName":   "unknown",
				"output":     map[string]any{"type": "text", "value": commandCodeTextContent(message["content"])},
			}}})
		}
	}
	return system, converted
}

// commandCodeToolsToCC converts OpenAI tools to the Command Code tools shape.
func commandCodeToolsToCC(tools []any) []any {
	converted := make([]any, 0, len(tools))
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		function, _ := tool["function"].(map[string]any)
		parameters := function["parameters"]
		if parameters == nil {
			parameters = map[string]any{}
		}
		converted = append(converted, map[string]any{
			"type":         "function",
			"name":         function["name"],
			"description":  function["description"],
			"input_schema": parameters,
		})
	}
	return converted
}

// commandCodeRequest carries the OpenAI request fields the adapter reads.
type commandCodeRequest struct {
	Model           string          `json:"model"`
	MaxTokens       int             `json:"max_tokens"`
	Temperature     json.RawMessage `json:"temperature"`
	ReasoningEffort json.RawMessage `json:"reasoning_effort"`
	Messages        []any           `json:"messages"`
	Tools           []any           `json:"tools"`
}

// commandCodeBodyFromOpenAI converts an OpenAI chat-completions body into the
// Command Code /alpha/generate body. The endpoint only streams, so the built
// body always requests a stream regardless of the client's flag.
func commandCodeBodyFromOpenAI(body []byte, workingDir string) ([]byte, error) {
	var in commandCodeRequest
	if json.Unmarshal(body, &in) != nil {
		return nil, fmt.Errorf("invalid OpenAI request")
	}
	if len(in.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}
	system, messages := commandCodeMessagesToCC(in.Messages)
	params := map[string]any{
		"model":       in.Model,
		"messages":    messages,
		"tools":       commandCodeToolsToCC(in.Tools),
		"system":      system,
		"max_tokens":  commandCodeMaxTokens(in.MaxTokens),
		"temperature": commandCodeTemperature(in.Temperature),
		"stream":      true,
	}
	if effort, ok := commandCodeReasoningEffort(in.Model, in.ReasoningEffort); ok {
		params["reasoning_effort"] = effort
	}
	payload := map[string]any{
		"config": map[string]any{
			"workingDir":    workingDir,
			"date":          time.Now().Format("2006-01-02"),
			"environment":   commandCodeEnvironment(),
			"structure":     []any{},
			"isGitRepo":     false,
			"currentBranch": "",
			"mainBranch":    "",
			"gitStatus":     "",
			"recentCommits": []any{},
		},
		"memory":   nil,
		"taste":    nil,
		"skills":   nil,
		"params":   params,
		"threadId": commandCodeNewThreadID(),
	}
	return json.Marshal(payload)
}

func commandCodeMaxTokens(maxTokens int) int {
	if maxTokens <= 0 {
		return commandCodeDefaultMaxTokens
	}
	if maxTokens > commandCodeDefaultMaxTokens {
		return commandCodeDefaultMaxTokens
	}
	return maxTokens
}

func commandCodeTemperature(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return commandCodeDefaultTemperature
	}
	var value float64
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return commandCodeDefaultTemperature
}

// commandCodeEvent is one Command Code SSE event.
type commandCodeEvent struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	ToolCallID   string          `json:"toolCallId"`
	ToolName     string          `json:"toolName"`
	Input        json.RawMessage `json:"input"`
	Args         json.RawMessage `json:"args"`
	Arguments    json.RawMessage `json:"arguments"`
	FinishReason string          `json:"finishReason"`
	TotalUsage   json.RawMessage `json:"totalUsage"`
	Error        json.RawMessage `json:"error"`
	Message      json.RawMessage `json:"message"`
}

type commandCodeUsage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	InputTokenDetails struct {
		NoCacheTokens    int `json:"noCacheTokens"`
		CacheReadTokens  int `json:"cacheReadTokens"`
		CacheWriteTokens int `json:"cacheWriteTokens"`
	} `json:"inputTokenDetails"`
}

// commandCodeFinishReason maps a Command Code finish reason to the OpenAI
// finish reason vocabulary.
func commandCodeFinishReason(reason string) string {
	switch reason {
	case "length", "max_tokens", "max-tokens", "max_output_tokens":
		return "length"
	case "tool-calls":
		return "tool_calls"
	default:
		return "stop"
	}
}

// commandCodeUsageFromEvent fills the gateway stream stats from a finish
// event's totalUsage object and returns the OpenAI usage object to emit.
func commandCodeUsageFromEvent(raw json.RawMessage, st *streamStats) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var u commandCodeUsage
	if json.Unmarshal(raw, &u) != nil {
		return nil
	}
	input := u.InputTokenDetails.NoCacheTokens
	if input <= 0 {
		input = u.InputTokens - u.InputTokenDetails.CacheReadTokens - u.InputTokenDetails.CacheWriteTokens
	}
	if input < 0 {
		input = 0
	}
	st.prompt = input
	st.completion = u.OutputTokens
	st.cached = u.InputTokenDetails.CacheReadTokens
	st.usageReported = true
	st.promptReported = true
	st.completionReported = true
	st.usageComplete = true
	return map[string]any{
		"prompt_tokens":     input,
		"completion_tokens": u.OutputTokens,
		"total_tokens":      input + u.OutputTokens,
		"prompt_tokens_details": map[string]any{
			"cached_tokens": u.InputTokenDetails.CacheReadTokens,
		},
	}
}

// commandCodeOpenAIChunk writes one OpenAI chat-completion SSE chunk.
func commandCodeOpenAIChunk(w http.ResponseWriter, flusher http.Flusher, id, model string, choices any, usage map[string]any) {
	if choices == nil {
		choices = []any{}
	}
	payload := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": choices,
	}
	if len(usage) > 0 {
		payload["usage"] = usage
	}
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// commandCodeToolEvent represents one parsed tool-call event.
type commandCodeToolEvent struct {
	id      string
	name    string
	argJSON string
}

// commandCodeToolEventFromEvent normalizes a tool-call event's input/args/
// arguments fields into a JSON string of arguments.
func commandCodeToolEventFromEvent(event commandCodeEvent) (commandCodeToolEvent, error) {
	raw := event.Input
	if len(raw) == 0 {
		raw = event.Args
	}
	if len(raw) == 0 {
		raw = event.Arguments
	}
	if len(raw) == 0 {
		return commandCodeToolEvent{id: event.ToolCallID, name: event.ToolName, argJSON: "{}"}, nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return commandCodeToolEvent{}, fmt.Errorf("invalid tool call input")
	}
	if text, isText := value.(string); isText {
		return commandCodeToolEvent{id: event.ToolCallID, name: event.ToolName, argJSON: text}, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return commandCodeToolEvent{}, err
	}
	return commandCodeToolEvent{id: event.ToolCallID, name: event.ToolName, argJSON: string(encoded)}, nil
}

// commandCodeErrorMessage extracts a human message from an error event.
func commandCodeErrorMessage(event commandCodeEvent) string {
	for _, raw := range []json.RawMessage{event.Error, event.Message} {
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) == nil {
			switch text := value.(type) {
			case string:
				if text != "" {
					return text
				}
			case map[string]any:
				for _, key := range []string{"message", "errorMessage", "error", "detail", "code", "type", "reason"} {
					if part, ok := text[key].(string); ok && part != "" {
						return part
					}
				}
			}
		}
	}
	return "unknown Command Code error"
}

// streamCommandCodeToOpenAI reads a Command Code SSE stream and writes OpenAI
// chat-completions SSE chunks to w. Usage is extracted from the finish event so
// the gateway can bill the request after the stream closes.
func streamCommandCodeToOpenAI(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
	var st streamStats
	flusher, ok := w.(http.Flusher)
	if !ok {
		return st, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	id := "chatcmpl-" + commandCodeNewID()
	model := ""
	toolIndexes := map[string]int{}
	nextToolIndex := 0
	sawText := false
	finished := false
	writeFinish := func() {
		if finished {
			return
		}
		commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}, nil)
		finished = true
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event commandCodeEvent
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		switch event.Type {
		case "text-delta":
			if !sawText {
				sawText = true
				commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}}, nil)
			}
			commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{"content": event.Text}, "finish_reason": nil}}, nil)
		case "reasoning-delta":
			commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": event.Text}, "finish_reason": nil}}, nil)
		case "tool-call":
			tool, toolErr := commandCodeToolEventFromEvent(event)
			if toolErr != nil {
				continue
			}
			if _, exists := toolIndexes[tool.id]; !exists {
				toolIndexes[tool.id] = nextToolIndex
				nextToolIndex++
				commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": toolIndexes[tool.id], "id": tool.id, "type": "function", "function": map[string]any{"name": tool.name, "arguments": ""}}}}, "finish_reason": nil}}, nil)
			}
			if tool.argJSON != "" {
				commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": toolIndexes[tool.id], "function": map[string]any{"arguments": tool.argJSON}}}}, "finish_reason": nil}}, nil)
			}
		case "finish":
			usage := commandCodeUsageFromEvent(event.TotalUsage, &st)
			commandCodeOpenAIChunk(w, flusher, id, model, []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": commandCodeFinishReason(event.FinishReason)}}, usage)
			finished = true
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return st, nil
		case "error":
			writeFinish()
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return st, fmt.Errorf("Command Code stream error: %s", commandCodeErrorMessage(event))
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return st, err
	}
	writeFinish()
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
	return st, nil
}

// commandCodeStreamToOpenAI reads a raw Command Code SSE response and returns
// the equivalent OpenAI chat-completion JSON. Used when the client did not
// request a stream: the upstream still streams, so the gateway buffers the
// events into one message.
func commandCodeStreamToOpenAI(body []byte) ([]byte, error) {
	id := "chatcmpl-" + commandCodeNewID()
	var text, reasoning strings.Builder
	type toolCall struct {
		id   string
		name string
		args strings.Builder
	}
	toolCalls := []toolCall{}
	toolIndexes := map[string]int{}
	var finishReason string
	var usageJSON map[string]any
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event commandCodeEvent
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		switch event.Type {
		case "text-delta":
			text.WriteString(event.Text)
		case "reasoning-delta":
			reasoning.WriteString(event.Text)
		case "tool-call":
			tool, toolErr := commandCodeToolEventFromEvent(event)
			if toolErr != nil {
				continue
			}
			if index, exists := toolIndexes[tool.id]; exists {
				toolCalls[index].args.WriteString(tool.argJSON)
			} else {
				toolIndexes[tool.id] = len(toolCalls)
				toolCalls = append(toolCalls, toolCall{id: tool.id, name: tool.name})
				toolCalls[len(toolCalls)-1].args.WriteString(tool.argJSON)
			}
		case "finish":
			finishReason = commandCodeFinishReason(event.FinishReason)
			if len(event.TotalUsage) > 0 && string(event.TotalUsage) != "null" {
				var u commandCodeUsage
				if json.Unmarshal(event.TotalUsage, &u) == nil {
					input := u.InputTokenDetails.NoCacheTokens
					if input <= 0 {
						input = u.InputTokens - u.InputTokenDetails.CacheReadTokens - u.InputTokenDetails.CacheWriteTokens
					}
					if input < 0 {
						input = 0
					}
					usageJSON = map[string]any{
						"prompt_tokens":     input,
						"completion_tokens": u.OutputTokens,
						"total_tokens":      input + u.OutputTokens,
						"prompt_tokens_details": map[string]any{
							"cached_tokens": u.InputTokenDetails.CacheReadTokens,
						},
					}
				}
			}
		case "error":
			return nil, fmt.Errorf("Command Code stream error: %s", commandCodeErrorMessage(event))
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	message := map[string]any{"role": "assistant", "content": text.String()}
	if reasoning.Len() > 0 {
		message["reasoning_content"] = reasoning.String()
	}
	if len(toolCalls) > 0 {
		converted := make([]any, 0, len(toolCalls))
		for _, tool := range toolCalls {
			converted = append(converted, map[string]any{"id": tool.id, "type": "function", "function": map[string]any{"name": tool.name, "arguments": tool.args.String()}})
		}
		message["tool_calls"] = converted
	}
	if finishReason == "" {
		finishReason = "stop"
	}
	response := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "",
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finishReason}},
	}
	if len(usageJSON) > 0 {
		response["usage"] = usageJSON
	}
	return json.Marshal(response)
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func asAnySlice(value any) []any {
	parts, _ := value.([]any)
	return parts
}
