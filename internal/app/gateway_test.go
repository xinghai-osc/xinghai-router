package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorCode(t *testing.T) {
	if got := errorCode(200); got != "" {
		t.Fatalf("errorCode(200) = %q", got)
	}
	if got := errorCode(404); got != "upstream_"+http.StatusText(404) {
		t.Fatalf("errorCode(404) = %q", got)
	}
}

func TestContentType(t *testing.T) {
	if got := contentType("text/event-stream; charset=utf-8"); got != "text/event-stream" {
		t.Fatalf("contentType = %q", got)
	}
	if got := contentType("text/plain"); got != "application/json" {
		t.Fatalf("contentType = %q", got)
	}
}

func TestChatCompletionsRejectsInvalidBodyBeforeUpstream(t *testing.T) {
	for _, body := range []string{`{}`, `{"model":""}`, `not-json`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		(&Service{}).chatCompletions(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d", body, rec.Code)
		}
	}
}

func TestPricingUnavailableErrorDistinct(t *testing.T) {
	if errors.Is(errInvalid, errPricingUnavailable) {
		t.Fatal("pricing and invalid errors must differ")
	}
	rec := httptest.NewRecorder()
	writeError(rec, 402, "pricing_unavailable", "no enabled pricing rule for this model")
	if rec.Code != 402 || !strings.Contains(rec.Body.String(), "pricing_unavailable") {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}
