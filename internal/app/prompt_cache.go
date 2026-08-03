package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// promptPrefixCache stores recent prompts per model so that when the upstream
// reports zero cached tokens for a request whose user-facing prompt overlaps a
// previously-served prompt, the gateway can still surface a cache-friendly amount
// and bill the shared prefix at the cached-input rate. It is deliberately small
// and in-memory only: it is an accounting assist, not a correctness feature.
//
// Keys are a normalized, whitespace-collapsed rendering of the request's
// messages. A new prompt is scored against entries for the same model by common
// prefix length; the best match's cached token estimate is capped by the
// upstream-reported prompt token count.
type promptPrefixCache struct {
	mu      sync.Mutex
	enabled bool
	size    int
	// index bounds matches to recent prompts only.
	grace time.Duration
	// entries per model, most-recently-stored last.
	entries map[string][]promptCacheEntry
}

type promptCacheEntry struct {
	key    string
	tokens int64
	stored time.Time
}

func newPromptPrefixCache(enabled bool, size int) *promptPrefixCache {
	max := size
	if max < 1 || max > 1<<16 {
		max = 4096
	}
	return &promptPrefixCache{enabled: enabled, size: max, grace: 30 * time.Minute, entries: map[string][]promptCacheEntry{}}
}

// cached scans the model's entries and returns a cached-token estimate for the
// given normalized prompt, or 0 when no useful prefix is cached. estimate is
// capped so it never exceeds the upstream-reported prompt token count (when that
// count is known and positive).
func (c *promptPrefixCache) cached(model, normalized string, promptTokens int64) int64 {
	if c == nil || !c.enabled || normalized == "" {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	best := int64(0)
	prune := c.entries[model][:0]
	for _, e := range c.entries[model] {
		if now.Sub(e.stored) > c.grace {
			continue
		}
		prune = append(prune, e)
		if e.key == "" || e.tokens <= 0 {
			continue
		}
		shared := promptPrefixTokens(e.key, normalized, e.tokens)
		if shared <= 0 {
			continue
		}
		if shared > best {
			best = shared
		}
	}
	c.entries[model] = prune
	if promptTokens > 0 && best > promptTokens {
		best = promptTokens
	}
	return best
}

// store remembers normalized so later, longer prompts that extend it can be
// billed partly at the cached-input rate. It records the base prompt's full
// token count, not the estimate.
func (c *promptPrefixCache) store(model, normalized string, tokens int64) {
	if c == nil || !c.enabled || normalized == "" || len(normalized) > 1<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	list := append(append([]promptCacheEntry{}, c.entries[model]...), promptCacheEntry{key: normalized, tokens: tokens, stored: time.Now()})
	if len(list) > c.size {
		list = list[len(list)-c.size:]
	}
	c.entries[model] = list
}

// normalizedPrompt renders the chat messages of a request body into a stable key
// for the prefix cache: JSON-encoded message content with whitespace collapsed.
// Requests without messages (or non-JSON bodies) yield "" and are not cached.
func normalizedPrompt(body []byte) string {
	var payload struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &payload) != nil || len(payload.Messages) == 0 {
		return ""
	}
	var parts []string
	for _, m := range payload.Messages {
		if !json.Valid(m) {
			return ""
		}
		parts = append(parts, collapseWhitespace(string(m)))
	}
	return strings.Join(parts, ",")
}

func collapseWhitespace(s string) string {
	var b bytes.Buffer
	b.Grow(len(s))
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			b.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			b.WriteByte(ch)
			continue
		}
		if !inString && (ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r') {
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// promptPrefixTokens returns the cached-token estimate for an incoming prompt
// relative to a stored one. It only scores a pair when one normalized prompt is
// a prefix of the other (the JSON scaffolding shared by every pair must not be
// billed as cached content). A longer incoming prompt extends a cached full
// prompt (everything cached is "already seen"); a shorter one is billed in
// proportion to how much of the cached prompt remains.
func promptPrefixTokens(cachedKey, incoming string, cachedTokens int64) int64 {
	var shorter, longer string
	if len(cachedKey) <= len(incoming) {
		shorter, longer = cachedKey, incoming
	} else {
		shorter, longer = incoming, cachedKey
	}
	if len(shorter) == 0 || !strings.HasPrefix(longer, shorter) {
		return 0
	}
	if len(incoming) >= len(cachedKey) {
		return cachedTokens
	}
	return int64(float64(cachedTokens) * float64(len(incoming)) / float64(len(cachedKey)))
}