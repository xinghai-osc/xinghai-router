package app

import "testing"

func TestOpenAIAdapters(t *testing.T) {
	if got := resolveUpstreamFormat("openai_chat", "openai"); got != "openai" {
		t.Fatalf("format = %q", got)
	}
	if got := resolveUpstreamFormat("openai", "openai_chat"); got != "openai_chat" {
		t.Fatalf("format = %q", got)
	}
}
