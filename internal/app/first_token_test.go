package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHasVisibleStreamText(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		// OpenAI chat-completions: visible text content
		{`openai content delta`, `{"choices":[{"index":0,"delta":{"content":"Hello"}}]}`, true},
		{`openai content delta non-ASCII`, `{"choices":[{"index":0,"delta":{"content":"你好"}}]}`, true},
		{`openai content whitespace only`, `{"choices":[{"index":0,"delta":{"content":" "}}}`, false},
		{`openai content newline only`, `{"choices":[{"index":0,"delta":{"content":"\n"}}}`, false},
		{`openai content empty string`, `{"choices":[{"index":0,"delta":{"content":""}}}`, false},
		// OpenAI role-only chunk
		{`openai role only`, `{"choices":[{"index":0,"delta":{"role":"assistant"}}]}`, false},
		// OpenAI tool_calls-only chunk
		{`openai tool calls`, `{"choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":"","name":"get_weather"}}]}}]}`, false},
		// OpenAI finish_reason-only
		{`openai finish reason`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`, false},
		// OpenAI empty choices
		{`openai empty choices`, `{"choices":[]}`, false},
		// Anthropic content_block_delta text_delta
		{`anthropic text delta`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`, true},
		{`anthropic text delta whitespace`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":" "}}`, false},
		{`anthropic text delta empty`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":""}}`, false},
		// Anthropic input_json_delta (tool arguments)
		{`anthropic input json delta`, `{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"loc\":\"NYC\"}"}}`, false},
		// Anthropic content_block_start
		{`anthropic content block start`, `{"type":"content_block_start","content_block":{"type":"text","text":""}}`, false},
		// Anthropic message_start (no delta)
		{`anthropic message start`, `{"type":"message_start","message":{"role":"assistant"}}`, false},
		// Anthropic message_delta (usage only)
		{`anthropic message delta`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`, false},
		// Anthropic message_stop
		{`anthropic message stop`, `{"type":"message_stop"}`, false},
		// Responses output_text.delta
		{`responses output text delta`, `{"type":"response.output_text.delta","delta":"Hello"}`, true},
		{`responses output text delta whitespace`, `{"type":"response.output_text.delta","delta":" "}`, false},
		{`responses output text delta empty`, `{"type":"response.output_text.delta","delta":""}`, false},
		// Responses function_call_arguments.delta
		{`responses function call args`, `{"type":"response.function_call_arguments.delta","delta":"{\"loc\":\"NYC\"}"}`, false},
		// Responses created/completed
		{`responses created`, `{"type":"response.created","response":{"id":"r_123"}}`, false},
		{`responses completed`, `{"type":"response.completed","response":{"id":"r_123"}}`, false},
		// Malformed
		{`malformed json`, `{bad json`, false},
		{`empty`, ``, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasVisibleStreamText([]byte(tt.data))
			if got != tt.want {
				t.Errorf("hasVisibleStreamText(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestFirstTokenTracker(t *testing.T) {
	// Never marked -> nil
	t1 := &firstTokenTracker{started: time.Now()}
	if ms := t1.milliseconds(); ms != nil {
		t.Fatalf("unmarked tracker must return nil, got %v", *ms)
	}
	// Marked -> non-negative ms
	started := time.Now().Add(-50 * time.Millisecond)
	t2 := &firstTokenTracker{started: started}
	t2.mark()
	ms := t2.milliseconds()
	if ms == nil {
		t.Fatal("marked tracker must not return nil")
	}
	if *ms < 0 || *ms > 10000 {
		t.Fatalf("marked tracker ms = %d, want 0..10000", *ms)
	}
	// Mark is idempotent
	first := *ms
	time.Sleep(2 * time.Millisecond)
	t2.mark()
	ms2 := t2.milliseconds()
	if ms2 == nil || *ms2 != first {
		t.Fatalf("second mark changed ms from %d to %d", first, *ms2)
	}
	// Negative duration clamps to 0
	started = time.Now().Add(10 * time.Second)
	t3 := &firstTokenTracker{started: started}
	t3.mark()
	ms3 := t3.milliseconds()
	if ms3 == nil || *ms3 != 0 {
		t.Fatalf("negative duration must clamp to 0, got %v", ms3)
	}
	// nil tracker safety
	if ms := (*firstTokenTracker)(nil).milliseconds(); ms != nil {
		t.Fatal("nil tracker must return nil")
	}
}

// TestFirstTokenWriter verifies that the writer detects the first visible
// SSE data line, does not mark on non-visible lines, passes bytes through
// identically, and handles split writes and CRLF line endings.
func TestFirstTokenWriter(t *testing.T) {
	t.Run("marks on first visible data line", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		body := "event: ping\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"}}]}\n\ndata: [DONE]\n\n"
		w.Write([]byte(body))
		ms := tracker.milliseconds()
		if ms == nil {
			t.Fatal("firstTokenWriter must mark on visible content")
		}
		if rec.Body.String() != body {
			t.Fatalf("body mismatch:\ngot  %q\nwant %q", rec.Body.String(), body)
		}
	})
	t.Run("does not mark on non-visible data", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\ndata: [DONE]\n\n"
		w.Write([]byte(body))
		if tracker.milliseconds() != nil {
			t.Fatal("must not mark on role-only chunk")
		}
	})
	t.Run("does not mark on [DONE]", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		w.Write([]byte("data: [DONE]\n\n"))
		if tracker.milliseconds() != nil {
			t.Fatal("must not mark on [DONE]")
		}
	})
	t.Run("visible data split across writes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"con"))
		if tracker.milliseconds() != nil {
			t.Fatal("must not mark on incomplete line")
		}
		w.Write([]byte("tent\":\"Hi\"}}]}\n\n"))
		ms := tracker.milliseconds()
		if ms == nil {
			t.Fatal("must mark after complete line arrives")
		}
		want := "data: {\"choices\":[{\"index\":0,\"delta\":{\"con" + "tent\":\"Hi\"}}]}\n\n"
		if rec.Body.String() != want {
			t.Fatalf("body mismatch:\ngot  %q\nwant %q", rec.Body.String(), want)
		}
	})
	t.Run("CRLF line endings", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"}}]}\r\n\r\n"
		w.Write([]byte(body))
		ms := tracker.milliseconds()
		if ms == nil {
			t.Fatal("must mark on CRLF line ending")
		}
		if rec.Body.String() != body {
			t.Fatalf("body mismatch:\ngot  %q\nwant %q", rec.Body.String(), body)
		}
	})
	t.Run("underlying write errors are propagated", func(t *testing.T) {
		// A writer that returns an error after the first byte simulates an
		// underlying transport failure.
		errWriter := &errWriter{err: io.ErrUnexpectedEOF}
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(errWriter, tracker)
		_, err := w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
		if err == nil {
			t.Fatal("expected write error to propagate")
		}
	})
	t.Run("Flush forwards to underlying flusher", func(t *testing.T) {
		rec := httptest.NewRecorder()
		tracker := &firstTokenTracker{started: time.Now()}
		w := newFirstTokenWriter(rec, tracker)
		// ResponseRecorder implements http.Flusher; flush should not panic.
		w.Flush()
	})
	t.Run("nil tracker does not panic", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := newFirstTokenWriter(rec, nil)
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
		if rec.Body.Len() == 0 {
			t.Fatal("nil tracker writer must still forward bytes")
		}
	})
}

// errWriter mimics an http.ResponseWriter that returns a fixed error on Write.
type errWriter struct {
	http.ResponseWriter
	err error
}

func (e *errWriter) Header() http.Header         { return http.Header{} }
func (e *errWriter) WriteHeader(statusCode int)  {}
func (e *errWriter) Write(p []byte) (int, error) { return 0, e.err }
