package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type quotaFailWriter struct {
	header http.Header
	writes int
	failAt int
}

func (w *quotaFailWriter) Header() http.Header { return w.header }
func (w *quotaFailWriter) WriteHeader(int)     {}
func (w *quotaFailWriter) Flush()              {}
func (w *quotaFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestDirectStreamRetainsUsageBeforeFailedWrite(t *testing.T) {
	for _, ending := range []string{"\n\n", ""} {
		body := "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":12,\"output_tokens\":7}}}" + ending
		w := &quotaFailWriter{header: make(http.Header), failAt: 1}
		st, err := streamResponseDirect(w, &http.Response{Body: io.NopCloser(strings.NewReader(body))})
		if !errors.Is(err, io.ErrClosedPipe) || st.prompt != 12 || st.completion != 7 || !st.usageComplete {
			t.Fatalf("stats=%+v error=%v", st, err)
		}
	}
}

func TestDirectStreamFlushesEachLineBeforeNextRead(t *testing.T) {
	w := httptest.NewRecorder()
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n"
	tail := firstTokenReadFunc(func([]byte) (int, error) {
		if w.Body.String() != body || !w.Flushed {
			t.Fatal("line was not flushed before next upstream read")
		}
		return 0, io.ErrUnexpectedEOF
	})
	st, err := streamResponseDirect(w, &http.Response{Body: io.NopCloser(io.MultiReader(strings.NewReader(body), tail))})
	if !errors.Is(err, io.ErrUnexpectedEOF) || st.facts.HasUsage() {
		t.Fatalf("partial output invented usage: %+v err=%v", st, err)
	}
}

func TestDirectStreamBoundsUnterminatedLine(t *testing.T) {
	var read int
	body := firstTokenReadFunc(func(p []byte) (int, error) {
		for i := range p {
			p[i] = 'x'
		}
		read += len(p)
		return len(p), nil
	})
	_, err := streamResponseDirect(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(body)})
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("error=%v", err)
	}
	if read > (2<<20)+4096 {
		t.Fatalf("read %d bytes past bound", read)
	}
}

func TestDirectStreamPreservesBytesAndUsageOnReadError(t *testing.T) {
	body := ": heartbeat\r\ndata: {\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":4}}"
	reader := firstTokenReadFunc(func(p []byte) (int, error) {
		if body == "" {
			return 0, io.ErrUnexpectedEOF
		}
		n := copy(p, body)
		body = body[n:]
		return n, io.ErrUnexpectedEOF
	})
	w := httptest.NewRecorder()
	st, err := streamResponseDirect(w, &http.Response{Body: io.NopCloser(reader)})
	if !errors.Is(err, io.ErrUnexpectedEOF) || st.prompt != 9 || st.completion != 4 {
		t.Fatalf("stats=%+v error=%v", st, err)
	}
	if w.Body.String() != ": heartbeat\r\ndata: {\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":4}}" {
		t.Fatalf("bytes changed: %q", w.Body.String())
	}
}

type eventFlushRecorder struct {
	*httptest.ResponseRecorder
	flushBytes []int
}

func (w *eventFlushRecorder) Flush() {
	w.flushBytes = append(w.flushBytes, w.Body.Len())
	w.ResponseRecorder.Flush()
}

func TestStreamFlushesCompleteEventsAndPartialFrames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		relay streamTransform
		eol   string
	}{
		{"chat", (&Service{}).streamResponse, "\n"},
		{"responses", streamResponseDirect, "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &eventFlushRecorder{ResponseRecorder: httptest.NewRecorder()}
			received := ": heartbeat\r\n\r\nevent: output\r\ndata: par"
			first := ": heartbeat" + tc.eol + tc.eol
			partial := first + "event: output" + tc.eol
			check := firstTokenReadFunc(func([]byte) (int, error) {
				if len(w.flushBytes) != 2 || w.flushBytes[0] != len(first) || w.flushBytes[1] != len(partial) {
					t.Fatalf("flush byte offsets = %v, want [%d %d]", w.flushBytes, len(first), len(partial))
				}
				return 0, io.EOF
			})
			resp := &http.Response{Body: io.NopCloser(io.MultiReader(strings.NewReader(received), check, strings.NewReader("tial\r\n\r\n")))}
			if _, err := tc.relay(w, resp); err != nil {
				t.Fatal(err)
			}
			if want := partial + "data: partial" + tc.eol + tc.eol; w.Body.String() != want {
				t.Fatalf("body = %q, want %q", w.Body.String(), want)
			}
		})
	}
}

func TestSSEUsageAliasesAfterNullDelta(t *testing.T) {
	for _, event := range []string{
		`{"usage":null,"usage":{"input_tokens":12,"output_tokens":7}}`,
		`{"usage":null,"totalUsage":{"inputTokens":12,"outputTokens":7}}`,
		`{"usage":null,"total_usage":{"input_tokens":12,"output_tokens":7}}`,
		`{"usage":null,"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":7}}`,
		`{"usage":null,"usage_metadata":{"prompt_token_count":12,"candidates_token_count":7}}`,
		`{"usage":null,"response":{"\u0075sage" : {"input_tokens":12,"output_tokens":7}}}`,
	} {
		var st streamStats
		parseSSEUsage([]byte(event), &st)
		if !st.usageComplete || st.prompt != 12 || st.completion != 7 {
			t.Fatalf("usage for %s = %+v", event, st)
		}
	}
}
