package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type firstTokenTracker struct {
	started time.Time
	at      time.Time
}

func (t *firstTokenTracker) mark() {
	if t != nil && t.at.IsZero() {
		t.at = time.Now()
	}
}

func (t *firstTokenTracker) milliseconds() *int {
	if t == nil || t.at.IsZero() {
		return nil
	}
	ms := int(t.at.Sub(t.started).Milliseconds())
	if ms < 0 {
		ms = 0
	}
	return &ms
}

func hasVisibleContent(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text) != ""
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return false
	}
	for _, part := range parts {
		if (part.Type == "text" || part.Type == "output_text") && strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

type firstTokenWriter struct {
	http.ResponseWriter
	tracker   *firstTokenTracker
	pending   bytes.Buffer
	eventType string
}

func newFirstTokenWriter(w http.ResponseWriter, tracker *firstTokenTracker) *firstTokenWriter {
	return &firstTokenWriter{ResponseWriter: w, tracker: tracker}
}

func (w *firstTokenWriter) processLine(line string) {
	if w.tracker == nil || !w.tracker.at.IsZero() {
		return
	}
	if line == "" {
		w.eventType = ""
		return
	}
	if strings.HasPrefix(line, "event:") {
		w.eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		return
	}
	if !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data != "" && data != "[DONE]" && hasStreamOutput([]byte(data), w.eventType) {
		w.tracker.mark()
	}
}

func (w *firstTokenWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.tracker != nil && w.tracker.at.IsZero() {
		w.pending.Write(p)
		for w.tracker.at.IsZero() {
			raw := w.pending.Bytes()
			i := bytes.IndexByte(raw, '\n')
			if i < 0 {
				break
			}
			line := strings.TrimSpace(string(raw[:i]))
			w.pending.Next(i + 1)
			w.processLine(line)
		}
	}
	return w.ResponseWriter.Write(p)
}

func (w *firstTokenWriter) finish() {
	if w.tracker == nil || !w.tracker.at.IsZero() || w.pending.Len() == 0 {
		return
	}
	raw := w.pending.String()
	for w.tracker.at.IsZero() {
		i := strings.IndexByte(raw, '\n')
		if i < 0 {
			w.processLine(strings.TrimSpace(raw))
			break
		}
		w.processLine(strings.TrimSpace(raw[:i]))
		raw = raw[i+1:]
	}
	w.pending.Reset()
}

func (w *firstTokenWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *firstTokenWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type firstTokenFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (f firstTokenFunction) hasOutput() bool {
	return strings.TrimSpace(f.Name) != "" || strings.TrimSpace(f.Arguments) != ""
}

func hasStreamOutput(data []byte, eventType string) bool {
	var event struct {
		Type    string          `json:"type"`
		Delta   json.RawMessage `json:"delta"`
		Choices []struct {
			Delta struct {
				Content          json.RawMessage    `json:"content"`
				ReasoningContent json.RawMessage    `json:"reasoning_content"`
				Reasoning        json.RawMessage    `json:"reasoning"`
				Refusal          json.RawMessage    `json:"refusal"`
				FunctionCall     firstTokenFunction `json:"function_call"`
				ToolCalls        []struct {
					Function firstTokenFunction `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
		ContentBlock struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
			Name     string `json:"name"`
		} `json:"content_block"`
		Item struct {
			Type string `json:"type"`
			firstTokenFunction
		} `json:"item"`
	}
	if json.Unmarshal(data, &event) != nil {
		return false
	}
	if event.Type == "" {
		event.Type = eventType
	}
	for _, choice := range event.Choices {
		delta := choice.Delta
		if hasVisibleContent(delta.Content) || hasVisibleContent(delta.ReasoningContent) ||
			hasVisibleContent(delta.Reasoning) || hasVisibleContent(delta.Refusal) || delta.FunctionCall.hasOutput() {
			return true
		}
		for _, call := range delta.ToolCalls {
			if call.Function.hasOutput() {
				return true
			}
		}
	}
	switch event.Type {
	case "content_block_start":
		block := event.ContentBlock
		switch block.Type {
		case "text":
			return strings.TrimSpace(block.Text) != ""
		case "thinking":
			return strings.TrimSpace(block.Thinking) != ""
		case "tool_use", "server_tool_use":
			return strings.TrimSpace(block.Name) != ""
		}
	case "content_block_delta":
		var delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			Thinking    string `json:"thinking"`
		}
		if json.Unmarshal(event.Delta, &delta) != nil {
			return false
		}
		switch delta.Type {
		case "text_delta":
			return strings.TrimSpace(delta.Text) != ""
		case "input_json_delta":
			return strings.TrimSpace(delta.PartialJSON) != ""
		case "thinking_delta":
			return strings.TrimSpace(delta.Thinking) != ""
		}
	case "response.output_text.delta", "response.refusal.delta", "response.reasoning_text.delta", "response.reasoning_summary_text.delta", "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		var delta string
		return json.Unmarshal(event.Delta, &delta) == nil && strings.TrimSpace(delta) != ""
	case "response.output_item.added":
		return event.Item.Type == "function_call" && event.Item.hasOutput()
	}
	return false
}
