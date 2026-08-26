package app

import "testing"

func TestNormalizedPrompt(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hello  world"}]}`)
	got := normalizedPrompt(body)
	if got == "" {
		t.Fatal("expected a normalized key")
	}
	again := normalizedPrompt([]byte(`{"model":"m","max_tokens":10,"messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hello  world"}]}`))
	if got != again {
		t.Fatalf("normalization depends on unrelated fields: %q vs %q", got, again)
	}
	if normalizedPrompt([]byte(`{"model":"m"}`)) != "" {
		t.Fatal("request without messages must not be cached")
	}
	if normalizedPrompt([]byte(`not json`)) != "" {
		t.Fatal("invalid json must not be cached")
	}
}

func TestPromptPrefixCacheExtendsPrefix(t *testing.T) {
	cache := newPromptPrefixCache(true, 16)
	// First call stores a 1000-token base prompt.
	cache.store("gpt-4", `{"role":"user","content":"build a website"}`, 1000)

	// A request with the same prefix but more content should claim most of the
	// base prompt as "cached".
	key := `{"role":"user","content":"build a website"},{"role":"assistant","content":"sure"},{"role":"user","content":"now add auth"}`
	if got := int(cache.cached("gpt-4", key, 3000)); got < 950 || got > 3000 {
		t.Fatalf("extension should estimate ~1000 cached tokens, got %d", got)
	}
	// Unrelated prompt gets nothing.
	if got := int(cache.cached("gpt-4", `{"role":"user","content":"totaly different thing"}`, 100)); got != 0 {
		t.Fatalf("unrelated prompt must not match, got %d", got)
	}
	// Other model gets nothing.
	if got := int(cache.cached("claude", key, 3000)); got != 0 {
		t.Fatalf("other model must not match, got %d", got)
	}
	// Cached tokens capped by upstream prompt count.
	if got := int(cache.cached("gpt-4", key, 5)); got != 5 {
		t.Fatalf("cap by prompt tokens, got %d", got)
	}
}

func TestPromptPrefixCacheDisabled(t *testing.T) {
	c := newPromptPrefixCache(false, 16)
	c.store("m", `{"role":"user","content":"hi"}`, 100)
	if got := c.cached("m", `{"role":"user","content":"hi"}`, 100); got != 0 {
		t.Fatalf("disabled cache must not match, got %d", got)
	}
}
