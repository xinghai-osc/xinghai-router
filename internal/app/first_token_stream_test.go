package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFirstTokenWriterEventType(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"text event", "event: response.output_text.delta\r\ndata: {\"delta\":\"hello\"}\r\n\r\n", true},
		{"tool event", "event: response.function_call_arguments.delta\ndata: {\"delta\":\"{}\"}\n\n", true},
		{"thinking event", "event: content_block_delta\ndata: {\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"think\"}}\n\n", true},
		{"event cleared at boundary", "event: response.output_text.delta\ndata: {\"delta\":\"\"}\n\ndata: {\"delta\":\"not an output event\"}\n\n", false},
		{"payload type authoritative", "event: response.output_text.delta\ndata: {\"type\":\"response.completed\",\"delta\":\"not output\"}\n\n", false},
		{"heartbeat and usage", ": ping\n\nevent: ping\ndata: {\"type\":\"ping\"}\n\ndata: {\"usage\":{\"output_tokens\":5}}\n\ndata: [DONE]\n\n", false},
		{"missing terminal newline", "event: response.function_call_arguments.delta\ndata: {\"delta\":\"{}\"}", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tracker := &firstTokenTracker{started: time.Now().Add(-time.Second)}
			w := newFirstTokenWriter(rec, tracker)
			for i := range tc.body {
				if _, err := w.Write([]byte(tc.body[i : i+1])); err != nil {
					t.Fatal(err)
				}
			}
			w.finish()
			if got := tracker.milliseconds() != nil; got != tc.want {
				t.Fatalf("recorded = %v, want %v", got, tc.want)
			}
			if rec.Body.String() != tc.body {
				t.Fatalf("response bytes changed: %q", rec.Body.String())
			}
		})
	}
}

func TestFirstTokenStreamAdapters(t *testing.T) {
	chatText := "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"
	chatTool := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{}\"}}]}}]}\n\n"
	chatReasoning := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n"
	anthropicTool := "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call_1\",\"name\":\"get_weather\",\"input\":{}}}\n\n"
	anthropicReasoning := "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"think\"}}\n\n"
	commandTool := "data: {\"type\":\"tool-call\",\"toolCallId\":\"call_1\",\"toolName\":\"get_weather\",\"input\":{}}\n\n"
	commandReasoning := "data: {\"type\":\"reasoning-delta\",\"text\":\"think\"}\n\n"
	responsesTool := "event: response.function_call_arguments.delta\ndata: {\"delta\":\"{}\"}\n\n"
	responsesReasoning := "data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"think\"}\n\n"
	toResponses := func(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
		return streamChatCompletionsToResponses(w, resp, "resp_test", responsesEcho{})
	}
	toChat := func(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
		return streamAnthropicToOpenAI(w, resp, "")
	}
	ws := &wsResponsesAdapter{model: "test-model"}
	for _, tc := range []struct {
		name   string
		body   string
		stream streamTransform
	}{
		{"chat text", chatText, (&Service{}).streamResponse},
		{"chat tool only", chatTool, (&Service{}).streamResponse},
		{"chat reasoning only", chatReasoning, (&Service{}).streamResponse},
		{"anthropic tool passthrough", anthropicTool, streamOpenAIToAnthropic},
		{"anthropic reasoning passthrough", anthropicReasoning, streamOpenAIToAnthropic},
		{"anthropic tool to chat", anthropicTool, toChat},
		{"chat tool to anthropic", chatTool, streamOpenAIToAnthropic},
		{"chat tool to responses", chatTool, toResponses},
		{"anthropic tool to responses", anthropicTool, toResponses},
		{"responses tool direct", responsesTool, streamResponseDirect},
		{"responses reasoning direct", responsesReasoning, streamResponseDirect},
		{"command tool to chat", commandTool, streamCommandCodeToOpenAI},
		{"command reasoning to chat", commandReasoning, streamCommandCodeToOpenAI},
		{"command tool to responses", commandTool, toResponses},
		{"command reasoning to responses", commandReasoning, toResponses},
		{"websocket bridge tool", chatTool, ws.stream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tracker := &firstTokenTracker{started: time.Now().Add(-time.Second)}
			writer := newFirstTokenWriter(rec, tracker)
			capture := newStreamCaptureWriter(writer, true)
			var first time.Time
			tail := firstTokenReadFunc(func([]byte) (int, error) {
				if tracker.at.IsZero() {
					t.Fatal("output must be timed before the upstream stream finishes")
				}
				first = tracker.at
				return 0, io.EOF
			})
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       io.NopCloser(io.MultiReader(strings.NewReader(tc.body), tail)),
			}
			if _, err := tc.stream(capture, resp); err != nil {
				t.Fatal(err)
			}
			writer.finish()
			if ms := tracker.milliseconds(); ms == nil || *ms < 1000 {
				t.Fatalf("missing or invalid first token time: %v", ms)
			}
			if !tracker.at.Equal(first) {
				t.Fatal("terminal events must not overwrite the first output time")
			}
			if string(capture.bytes()) != rec.Body.String() {
				t.Fatal("timing wrapper changed cached response bytes")
			}
		})
	}
}

func TestFirstTokenPartialStream(t *testing.T) {
	for _, output := range []string{
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\"}}]}}]}\n\n",
	} {
		tracker := &firstTokenTracker{started: time.Now()}
		writer := newFirstTokenWriter(httptest.NewRecorder(), tracker)
		resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(io.MultiReader(
			strings.NewReader(output),
			firstTokenReadFunc(func([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }),
		))}
		_, err := (&Service{}).streamResponse(writer, resp)
		writer.finish()
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("stream error = %v", err)
		}
		if tracker.milliseconds() == nil {
			t.Fatal("partial generated output must retain its first token time")
		}
	}
}

type firstTokenReadFunc func([]byte) (int, error)

func (f firstTokenReadFunc) Read(p []byte) (int, error) { return f(p) }
