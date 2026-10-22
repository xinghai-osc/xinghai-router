package app

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type consoleRequestBody struct {
	source io.ReadCloser
	load   func() ([]byte, error)
	reader *bytes.Reader
	err    error
}

func (b *consoleRequestBody) Read(p []byte) (int, error) {
	if b.reader == nil && b.err == nil {
		var data []byte
		data, b.err = b.load()
		b.reader = bytes.NewReader(data)
	}
	if b.err != nil {
		return 0, b.err
	}
	return b.reader.Read(p)
}

func (b *consoleRequestBody) Close() error { return b.source.Close() }

type consoleBodyResponseWriter struct {
	protoMajor int
	http.ResponseWriter
	body     *consoleRequestBody
	written  bool
	rejected bool
}

func (w *consoleBodyResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *consoleBodyResponseWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	if w.body.reader == nil && w.body.err == nil && w.protoMajor == 1 {
		w.Header().Set("Connection", "close")
		_ = http.NewResponseController(w.ResponseWriter).SetReadDeadline(time.Now())
	}
	if w.body.err != nil {
		w.rejected = true
		status, code, message := requestBodyError(w.body.err)
		writeError(w.ResponseWriter, status, code, message)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *consoleBodyResponseWriter) Write(p []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if w.rejected {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}
