package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONEncodesUserIDsAsNumbers(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, map[string]any{
		"user_id":        "42",
		"inviter_id":     "7",
		"origin_user_id": "9",
		"released_by":    nil,
		"user_ids":       []string{"42", "7"},
		"activity":       map[string]any{"user_id": "system"},
		"id":             "resource-uuid",
	})

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user_id"] != float64(42) || body["inviter_id"] != float64(7) || body["origin_user_id"] != float64(9) {
		t.Fatalf("user IDs were not numeric: %#v", body)
	}
	if body["id"] != "resource-uuid" {
		t.Fatalf("non-user id was changed: %#v", body["id"])
	}
	if activity := body["activity"].(map[string]any); activity["user_id"] != nil {
		t.Fatalf("non-numeric user_id was not cleared: %#v", activity)
	}
	ids := body["user_ids"].([]any)
	if ids[0] != float64(42) || ids[1] != float64(7) {
		t.Fatalf("user_ids were not numeric: %#v", ids)
	}
}

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
