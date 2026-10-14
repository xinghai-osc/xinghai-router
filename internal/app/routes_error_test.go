package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorIncludesRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Request-ID", "req-test-123")
	writeError(rec, 429, "quota_exceeded", "request quota exceeded")

	var body struct {
		Error struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 429 || body.Error.Message != "request quota exceeded" || body.Error.Type != "quota_exceeded" || body.Error.Code != "quota_exceeded" || body.Error.RequestID != "req-test-123" {
		t.Fatalf("unexpected error response: status=%d body=%+v", rec.Code, body)
	}
}
