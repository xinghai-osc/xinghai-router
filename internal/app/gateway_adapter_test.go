package app

import "testing"

func TestOpenAIAdapters(t *testing.T) {
	if !openAIResponsePreferred("openai", "") {
		t.Fatal("openai should prefer responses")
	}
	if openAIResponsePreferred("openai", "openai_chat") {
		t.Fatal("openai_chat must not prefer responses")
	}
	if got := resolveUpstreamFormat("openai_chat", "openai"); got != "openai_chat" {
		t.Fatalf("format = %q", got)
	}
	if got := resolveUpstreamFormat("openai", "openai_chat"); got != "openai" {
		t.Fatalf("format = %q", got)
	}
}
