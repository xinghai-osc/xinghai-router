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

func hasVisibleJSONString(raw json.RawMessage) bool {
	if len(raw) < 2 || raw[0] != '"' {
		return false
	}
	text := raw[1 : len(raw)-1]
	if bytes.IndexByte(text, '\\') < 0 {
		return len(bytes.TrimSpace(text)) > 0
	}
	var decoded string
	return json.Unmarshal(raw, &decoded) == nil && strings.TrimSpace(decoded) != ""
}

func hasVisibleContent(raw json.RawMessage) bool {
	if len(raw) == 0 || raw[0] != '[' {
		return hasVisibleJSONString(raw)
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

func (w *firstTokenWriter) processLine(line []byte) {
	if w.tracker == nil || !w.tracker.at.IsZero() {
		return
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		w.eventType = ""
		return
	}
	if bytes.HasPrefix(line, []byte("event:")) {
		w.eventType = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
		return
	}
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
	if len(data) > 0 && !bytes.Equal(data, []byte("[DONE]")) && hasStreamOutput(data, w.eventType) {
		w.tracker.mark()
	}
}

func (w *firstTokenWriter) Write(p []byte) (int, error) {
	if w.tracker != nil && w.tracker.at.IsZero() {
		for rest := p; len(rest) > 0; {
			i := bytes.IndexByte(rest, '\n')
			if i < 0 {
				_, _ = w.pending.Write(rest)
				break
			}
			line := rest[:i]
			if w.pending.Len() > 0 {
				_, _ = w.pending.Write(line)
				line = w.pending.Bytes()
				w.pending.Reset()
			}
			w.processLine(line)
			if !w.tracker.at.IsZero() {
				w.pending = bytes.Buffer{}
				break
			}
			rest = rest[i+1:]
		}
	}
	return w.ResponseWriter.Write(p)
}

func (w *firstTokenWriter) finish() {
	if w.tracker == nil || !w.tracker.at.IsZero() || w.pending.Len() == 0 {
		return
	}
	w.processLine(w.pending.Bytes())
	w.pending = bytes.Buffer{}
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
		return hasVisibleJSONString(event.Delta)
	case "response.output_item.added":
		return event.Item.Type == "function_call" && event.Item.hasOutput()
	}
	return false
}
