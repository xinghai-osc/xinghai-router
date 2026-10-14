package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// sessionAffinityTTL is how long an idle conversation keeps its assignment.
	// Every request that is served renews the lease, so an active session never
	// loses the upstream prompt cache it built, while an abandoned one frees its
	// slot. The default covers conversations resumed the next day, which is when
	// long-lived upstream prefix caches are still warm.
	sessionAffinityTTL = 7 * 24 * time.Hour
	// maxSessionAffinityEntries bounds the cache at roughly a few dozen megabytes
	// of live conversations; past the bound the oldest assignments are dropped.
	maxSessionAffinityEntries = 1 << 17
	// sessionReferenceTextLimit bounds the per-message text that is hashed into a
	// derived session reference.
	sessionReferenceTextLimit = 2048
	// maxSessionIDLength matches the length a client-provided session header must
	// respect to be forwarded upstream verbatim.
	maxSessionIDLength = 256
)

// sessionAffinityKey scopes an assignment to the routing context that produced
// it, so two users, groups or models that happen to share a session id never
// inherit each other's channel.
type sessionAffinityKey struct {
	session string
	userID  string
	groupID string
	model   string
}

// sessionAffinityTarget is one session's assignment: the upstream channel that
// holds the conversation's prompt cache and the credential that filled it.
type sessionAffinityTarget struct {
	channelID int64
	keyID     string
}

// sessionIdentity is a gateway request's conversation identity. stable reports
// whether the next turn of the same conversation is recognized again - a
// client-declared session header and a head-derived conversation reference are,
// the per-request id is not - because only a recognizable conversation can be
// served by the channel that cached its prompt prefix.
type sessionIdentity struct {
	session string
	stable  bool
}

type sessionAffinityEntry struct {
	target  sessionAffinityTarget
	expires time.Time
}

// sessionAffinityRecord is the persisted form of one assignment. It carries the
// full cache key so a restored snapshot cannot leak an assignment into another
// user, group or model.
type sessionAffinityRecord struct {
	Session   string    `json:"session"`
	UserID    string    `json:"user_id,omitempty"`
	GroupID   string    `json:"group_id,omitempty"`
	Model     string    `json:"model"`
	ChannelID int64     `json:"channel_id"`
	KeyID     string    `json:"key_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// sessionAffinityCache remembers which channel served a conversation so every
// later request of that conversation keeps landing there. Providers cache prompt
// prefixes per upstream account, so reusing one channel across the turns of a
// conversation keeps that prefix cached - which lowers both latency and the
// cached-input price - while a fresh channel per request discards it every turn.
//
// Entries expire after the ttl, are renewed on every request that is served by
// the assigned channel, and are evicted oldest-first once limit is reached, so a
// busy deployment cannot grow the map without bound. Changes are collected into
// a snapshot that is written to disk, which is what carries the assignments
// across a restart or a redeploy.
type sessionAffinityCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	limit   int
	now     func() time.Time
	dirty   bool
	entries map[sessionAffinityKey]sessionAffinityEntry
	order   []sessionAffinityKey
}

func newSessionAffinityCache(ttl time.Duration, limit int) *sessionAffinityCache {
	if ttl <= 0 {
		ttl = sessionAffinityTTL
	}
	if limit <= 0 {
		limit = maxSessionAffinityEntries
	}
	return &sessionAffinityCache{ttl: ttl, limit: limit, now: time.Now, entries: map[sessionAffinityKey]sessionAffinityEntry{}}
}

func (c *sessionAffinityCache) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

func (c *sessionAffinityCache) get(key sessionAffinityKey) (sessionAffinityTarget, bool) {
	if c == nil || key.session == "" {
		return sessionAffinityTarget{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !c.clock().Before(entry.expires) {
		return sessionAffinityTarget{}, false
	}
	return entry.target, true
}

// bind assigns the channel that served a request to its session, or renews the
// lease of the existing assignment when the same channel served it again. Either
// way the entry is kept alive for another ttl, which is the renewal that keeps a
// conversation's prompt cache reachable.
func (c *sessionAffinityCache) bind(key sessionAffinityKey, target sessionAffinityTarget) {
	if c == nil || key.session == "" || target.channelID == 0 {
		return
	}
	expires := c.clock().Add(c.ttl)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = true
	if entry, ok := c.entries[key]; ok {
		entry.target = target
		entry.expires = expires
		c.entries[key] = entry
		return
	}
	if len(c.entries) >= c.limit && len(c.order) > 0 {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[key] = sessionAffinityEntry{target: target, expires: expires}
	c.order = append(c.order, key)
}

// snapshot drops expired assignments and returns the records to persist, in
// renewal order. It reports false when the file on disk is already current.
func (c *sessionAffinityCache) snapshot() ([]sessionAffinityRecord, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	pruned := 0
	live := c.order[:0]
	for _, key := range c.order {
		entry, ok := c.entries[key]
		if !ok || !now.Before(entry.expires) {
			delete(c.entries, key)
			pruned++
			continue
		}
		live = append(live, key)
	}
	c.order = live
	stale := c.dirty || pruned > 0
	c.dirty = false
	if !stale {
		return nil, false
	}
	records := make([]sessionAffinityRecord, 0, len(live))
	for _, key := range live {
		entry := c.entries[key]
		records = append(records, sessionAffinityRecord{
			Session:   key.session,
			UserID:    key.userID,
			GroupID:   key.groupID,
			Model:     key.model,
			ChannelID: entry.target.channelID,
			KeyID:     entry.target.keyID,
			ExpiresAt: entry.expires,
		})
	}
	return records, true
}

// restore loads a persisted snapshot, ignoring assignments that already expired
// or that do not fit the cache bound. A restored entry counts as unchanged, so a
// service that only reads a snapshot does not rewrite it.
func (c *sessionAffinityCache) restore(records []sessionAffinityRecord) int {
	if c == nil || len(records) == 0 {
		return 0
	}
	now := c.clock()
	c.mu.Lock()
	defer c.mu.Unlock()
	restored := 0
	for _, record := range records {
		if restored >= c.limit {
			break
		}
		if record.Session == "" || record.ChannelID == 0 || !now.Before(record.ExpiresAt) {
			continue
		}
		key := sessionAffinityKey{session: record.Session, userID: record.UserID, groupID: record.GroupID, model: record.Model}
		if _, exists := c.entries[key]; exists {
			continue
		}
		c.entries[key] = sessionAffinityEntry{target: sessionAffinityTarget{channelID: record.ChannelID, keyID: record.KeyID}, expires: record.ExpiresAt}
		c.order = append(c.order, key)
		restored++
	}
	return restored
}

// markDirty schedules another write, used when persisting a snapshot failed.
func (c *sessionAffinityCache) markDirty() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.dirty = true
	c.mu.Unlock()
}

// requestSessionID resolves the session identity of a gateway request: the
// client's x-opencode-session header when it carries a usable value, otherwise a
// reference derived from the conversation head so clients that do not send the
// header still keep one channel per conversation, otherwise the per-request id.
// Only the first two identify the conversation again on a later turn, which is
// what stable reports.
func requestSessionID(ctx context.Context, r *http.Request, body []byte, userID string) sessionIdentity {
	if provided := strings.TrimSpace(r.Header.Get("x-opencode-session")); provided != "" && len(provided) <= maxSessionIDLength {
		return sessionIdentity{session: provided, stable: true}
	}
	if reference := conversationReference(body, userID); reference != "" {
		return sessionIdentity{session: reference, stable: true}
	}
	return sessionIdentity{session: opencodeSessionID(ctx, "")}
}

// conversationReference derives a stable identity for a chat conversation from
// its request body: the system prompt plus the first user message, hashed. Every
// later turn repeats that head, so a conversation keeps one reference from its
// first request to its last. The user id is mixed in so two users with identical
// prompts never share an upstream session. Bodies without a user message are not
// a conversation yet, and yield "".
func conversationReference(body []byte, userID string) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	system, first := "", ""
	for _, message := range payload.Messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "system", "developer":
			if system == "" {
				system = sessionReferenceText(message.Content)
			}
		case "user":
			if first == "" {
				first = sessionReferenceText(message.Content)
			}
		}
		if first != "" {
			break
		}
	}
	if first == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(userID + "\x00" + system + "\x00" + first))
	return hex.EncodeToString(sum[:16])
}

// sessionReferenceText renders a message's content as bounded text: either the
// string form or the concatenated text parts of a multimodal message.
func sessionReferenceText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return boundSessionText(text)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		if part.Text == "" {
			continue
		}
		b.WriteString(part.Text)
		if b.Len() >= sessionReferenceTextLimit {
			break
		}
	}
	return boundSessionText(b.String())
}

// boundSessionText collapses whitespace and truncates to the hashed length. The
// prefix is truncated before collapsing so a huge first message stays cheap, and
// truncation is stable: the same head always hashes to the same reference.
func boundSessionText(text string) string {
	const slack = sessionReferenceTextLimit * 4
	if len(text) > slack {
		text = text[:slack]
	}
	text = collapseWhitespace(text)
	if len(text) > sessionReferenceTextLimit {
		text = text[:sessionReferenceTextLimit]
	}
	return text
}

// applySessionAffinity orders the candidate channels by the session's affinity
// and restores the credential the session was pinned with, so a conversation is
// retried against the upstream that cached its prompt prefix before any channel
// without affinity for it. The bound channel is promoted to the front even when
// a higher-priority channel exists: the affinity is worth more than the priority
// order, because the upstream prefix cache it protects is what lowers latency
// and the cached-input price, and the remaining channels stay behind it in their
// natural order as the fallback.
//
// A session that has no assignment yet picks one, weighted by channel weight and
// seeded by the session id, so every request of that session makes the same
// choice. A session whose bound channel is no longer a candidate (disabled,
// deleted, out of the key's group) is assigned a fresh one.
func (s *Service) applySessionAffinity(key sessionAffinityKey, channels []channel) []channel {
	if s.sessionAffinity == nil || key.session == "" || len(channels) == 0 {
		return channels
	}
	pinned, hasPin := s.sessionAffinity.get(key)
	target := -1
	if hasPin {
		for i, ch := range channels {
			if ch.id == pinned.channelID {
				target = i
				break
			}
		}
	}
	if target < 0 {
		target = pickSessionChannel(channels, key.session)
	}
	if target < 0 {
		return channels
	}
	if ch := &channels[target]; len(ch.keys) > 1 {
		index := initialKeyIndex(ch.keys, sessionSeed(key.session))
		if hasPin && pinned.keyID != "" {
			for i := range ch.keys {
				// The pinned credential may have been disabled since; the channel's
				// remaining keys are the fallback.
				if ch.keys[i].id == pinned.keyID {
					index = i
					break
				}
			}
		}
		ch.keyIndex = index
		ch.apiKey = ch.keys[index].key
		ch.keyID = ch.keys[index].id
	}
	if target != 0 {
		channels[0], channels[target] = channels[target], channels[0]
	}
	return channels
}

// pickSessionChannel assigns a session to one of the candidates, weighted by
// channel weight, and returns its index. Channels without a positive weight are
// only chosen when no candidate carries one, and the caller guarantees a
// non-empty candidate list. The choice is seeded by the session id so it is
// stable for every request of that session.
func pickSessionChannel(channels []channel, session string) int {
	seed := sessionSeed(session)
	first, total := -1, 0
	for i, ch := range channels {
		if ch.weight <= 0 {
			continue
		}
		if first < 0 {
			first = i
		}
		total += ch.weight
	}
	if first < 0 {
		return 0
	}
	pick := int(seed[0])<<8 | int(seed[1])
	pick %= total
	for i, ch := range channels {
		if ch.weight <= 0 {
			continue
		}
		pick -= ch.weight
		if pick < 0 {
			return i
		}
	}
	return first
}

// bindSessionChannel records the channel that actually served a conversation,
// whatever protocol it speaks. The next request of the session is tried there
// first, so a failover that moved the conversation to another channel moves the
// affinity with it.
func (s *Service) bindSessionChannel(key sessionAffinityKey, ch channel) {
	if s.sessionAffinity == nil || ch.id == 0 {
		return
	}
	s.sessionAffinity.bind(key, sessionAffinityTarget{channelID: ch.id, keyID: ch.keyID})
}

func sessionSeed(session string) []byte {
	sum := sha256.Sum256([]byte(session))
	return sum[:]
}
