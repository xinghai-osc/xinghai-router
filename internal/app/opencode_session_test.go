package app

import (
	"context"
	"strings"
	"testing"
)

func TestOpencodeSessionID(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey{}, "request-session")
	if got := opencodeSessionID(ctx, " client-session "); got != "client-session" {
		t.Fatalf("provided session = %q", got)
	}
	if got := opencodeSessionID(ctx, ""); got != "request-session" {
		t.Fatalf("request session = %q", got)
	}
	if got := opencodeSessionID(context.Background(), strings.Repeat("x", 257)); got == "" || len(got) > 256 {
		t.Fatalf("fallback session = %q", got)
	}
}
