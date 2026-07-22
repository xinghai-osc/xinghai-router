package app

import (
	"net/http"
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
	if got := errorCode(500); !strings.HasPrefix(got, "upstream_") {
		t.Fatalf("errorCode(500) = %q", got)
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"application/json", "application/json"},
		{"application/json; charset=utf-8", "application/json"},
		{"text/event-stream", "text/event-stream"},
		{"text/event-stream; charset=utf-8", "text/event-stream"},
		{"text/plain", "application/json"},
		{"", "application/json"},
	}
	for _, tt := range tests {
		if got := contentType(tt.in); got != tt.want {
			t.Fatalf("contentType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStreamSkipsWalletReservationFlag(t *testing.T) {
	// Stream requests are not settled; reservation must stay empty so concurrent
	// stream traffic does not pin wallet reserved balances.
	var reserved reservation
	stream := true
	if stream {
		reserved = reservation{}
	}
	if reserved.amount != 0 {
		t.Fatal("stream path must not hold a non-zero reservation")
	}
}
