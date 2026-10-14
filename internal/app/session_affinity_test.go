package app

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func affinityChannel(id int64, provider string, priority, weight int, keyIDs ...string) channel {
	ch := channel{id: id, provider: provider, priority: priority, weight: weight}
	for _, keyID := range keyIDs {
		ch.keys = append(ch.keys, channelKeyCredential{id: keyID, key: "fake-key-" + keyID, priority: 1})
	}
	if len(ch.keys) > 0 {
		ch.keyIndex = 0
		ch.apiKey = ch.keys[0].key
		ch.keyID = ch.keys[0].id
	}
	return ch
}

func affinityIDs(channels []channel) []int64 {
	ids := make([]int64, len(channels))
	for i, ch := range channels {
		ids[i] = ch.id
	}
	return ids
}

func TestConversationReferenceFollowsTheConversationHead(t *testing.T) {
	head := `{"messages":[{"role":"system","content":"you are a coding agent"},{"role":"user","content":"fix the build"}]}`
	reference := conversationReference([]byte(head), "user-1")
	if reference == "" {
		t.Fatal("no reference for a conversation")
	}
	grown := `{"messages":[{"role":"system","content":"you are a coding agent"},{"role":"user","content":"fix the build"},{"role":"assistant","content":"sure"},{"role":"user","content":"now run the tests"}]}`
	if got := conversationReference([]byte(grown), "user-1"); got != reference {
		t.Fatalf("later turn lost the reference: %q != %q", got, reference)
	}
	other := `{"messages":[{"role":"system","content":"you are a coding agent"},{"role":"user","content":"write a poem"}]}`
	if got := conversationReference([]byte(other), "user-1"); got == reference {
		t.Fatal("a different conversation reused the reference")
	}
	if got := conversationReference([]byte(head), "user-2"); got == reference {
		t.Fatal("two users shared a reference")
	}
	whitespace := `{"messages":[{"role":"system","content":"you are  a\ncoding agent"},{"role":"user","content":"fix   the build"}]}`
	if got := conversationReference([]byte(whitespace), "user-1"); got != reference {
		t.Fatalf("whitespace changed the reference: %q != %q", got, reference)
	}
	parts := `{"messages":[{"role":"system","content":[{"type":"text","text":"you are a coding agent"}]},{"role":"user","content":[{"type":"text","text":"fix the build"}]}]}`
	if got := conversationReference([]byte(parts), "user-1"); got != reference {
		t.Fatalf("content parts changed the reference: %q != %q", got, reference)
	}
}

func TestConversationReferenceRequiresAUserTurn(t *testing.T) {
	for _, body := range []string{
		"",
		"not json",
		`{"messages":[]}`,
		`{"messages":[{"role":"system","content":"only a system prompt"}]}`,
		`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`,
	} {
		if got := conversationReference([]byte(body), "user-1"); got != "" {
			t.Fatalf("conversationReference(%q) = %q, want empty", body, got)
		}
	}
}

func TestConversationReferenceBoundsHugeMessages(t *testing.T) {
	head := strings.Repeat("x", 2*sessionReferenceTextLimit)
	body := `{"messages":[{"role":"user","content":"` + head + `"}]}`
	reference := conversationReference([]byte(body), "user-1")
	if reference == "" || len(reference) > maxSessionIDLength {
		t.Fatalf("reference = %q", reference)
	}
	extended := `{"messages":[{"role":"user","content":"` + head + strings.Repeat("y", 1000) + `"}]}`
	if got := conversationReference([]byte(extended), "user-1"); got != reference {
		t.Fatal("a growing single-message conversation lost its reference")
	}
}

func TestRequestSessionIDPrefersHeaderThenReference(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	request.Header.Set("x-opencode-session", " client-session ")
	if got := requestSessionID(context.Background(), request, body, "user-1"); got.session != "client-session" || !got.stable {
		t.Fatalf("header identity = %#v", got)
	}
	request.Header.Set("x-opencode-session", strings.Repeat("x", maxSessionIDLength+1))
	if got := requestSessionID(context.Background(), request, body, "user-1"); got.session != conversationReference(body, "user-1") || !got.stable {
		t.Fatalf("oversized header was not replaced by the conversation reference: %#v", got)
	}
	request.Header.Del("x-opencode-session")
	if got := requestSessionID(context.Background(), request, body, "user-1"); got.session != conversationReference(body, "user-1") || !got.stable {
		t.Fatalf("derived identity = %#v", got)
	}
	ctx := context.WithValue(context.Background(), requestIDKey{}, "request-session")
	if got := requestSessionID(ctx, request, nil, "user-1"); got.session != "request-session" || got.stable {
		t.Fatalf("fallback identity = %#v, want an unstable per-request id", got)
	}
}

func TestSessionAffinityKeepsOneChannelPerSession(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	key := sessionAffinityKey{session: "session-1", userID: "user-1", model: "model"}
	candidates := []channel{
		affinityChannel(1, "opencode_go", 10, 1, "key-a", "key-b"),
		affinityChannel(2, "opencode_go", 10, 1, "key-a", "key-b"),
	}
	first := service.applySessionAffinity(key, candidates)
	pinned := first[0]
	service.bindSessionChannel(key, pinned)

	// The next request sees the candidates in the opposite order and the
	// credentials in the opposite order: the session must still be served by the
	// same channel and the same credential.
	rotated := []channel{
		affinityChannel(2, "opencode_go", 10, 1, "key-b", "key-a"),
		affinityChannel(1, "opencode_go", 10, 1, "key-b", "key-a"),
	}
	next := service.applySessionAffinity(key, rotated)
	if next[0].id != pinned.id {
		t.Fatalf("channel = %d, want the pinned %d", next[0].id, pinned.id)
	}
	if next[0].keyID != pinned.keyID {
		t.Fatalf("credential = %q, want the pinned %q", next[0].keyID, pinned.keyID)
	}
	if next[0].apiKey != pinned.apiKey {
		t.Fatal("credential value does not match the pinned credential")
	}
}

func TestSessionAffinityMovesWithFailover(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	key := sessionAffinityKey{session: "session-2", model: "model"}
	candidates := []channel{
		affinityChannel(1, "opencode_go", 10, 1, "key-a"),
		affinityChannel(2, "opencode_go", 10, 1, "key-a"),
	}
	service.bindSessionChannel(key, affinityChannel(2, "opencode_go", 10, 1, "key-a"))
	if got := service.applySessionAffinity(key, candidates); got[0].id != 2 {
		t.Fatalf("sticky channel = %d, want 2", got[0].id)
	}
	// A channel that disappeared from the candidate list (disabled, deleted, out
	// of the key's group) must fall back to a fresh assignment.
	gone := []channel{affinityChannel(7, "opencode_go", 10, 1, "key-a")}
	if got := service.applySessionAffinity(key, gone); got[0].id != 7 {
		t.Fatalf("reassigned channel = %d, want 7", got[0].id)
	}
	// A credential disabled after the pin was recorded falls back to a live one.
	shrunk := []channel{affinityChannel(2, "opencode_go", 10, 1, "key-b", "key-c")}
	if got := service.applySessionAffinity(key, shrunk); got[0].id != 2 || (got[0].keyID != "key-b" && got[0].keyID != "key-c") {
		t.Fatalf("credential = %d/%q, want channel 2 on a live credential", got[0].id, got[0].keyID)
	}
}

func TestSessionAffinitySpreadsSessionsOverWeightedChannels(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	selected := map[int64]int{}
	for i := 0; i < 60; i++ {
		key := sessionAffinityKey{session: "session-" + string(rune('a'+i%26)) + string(rune('0'+i/26)), model: "model"}
		channels := service.applySessionAffinity(key, []channel{
			affinityChannel(1, "opencode_go", 10, 1, "key-a"),
			affinityChannel(2, "opencode_go", 10, 1, "key-a"),
		})
		selected[channels[0].id]++
	}
	if selected[1] == 0 || selected[2] == 0 {
		t.Fatalf("sessions were not spread over channels: %#v", selected)
	}
}

func TestSessionAffinityPromotesTheBoundChannelOverPriority(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	key := sessionAffinityKey{session: "session-3", model: "model"}
	// A conversation served once by the lower-priority channel keeps retrying it
	// before the higher-priority channels that have no affinity for it.
	service.bindSessionChannel(key, affinityChannel(2, "opencode_go", 5, 1, "key-a"))
	channels := []channel{
		affinityChannel(1, "openai", 10, 1, "key-a"),
		affinityChannel(2, "opencode_go", 5, 1, "key-a"),
	}
	got := service.applySessionAffinity(key, channels)
	if want := []int64{2, 1}; !slices.Equal(affinityIDs(got), want) {
		t.Fatalf("affinity order = %v, want the bound channel first: %v", affinityIDs(got), want)
	}
	// A session without an assignment is assigned to one of the candidates, so
	// every later turn of the session keeps landing there.
	fresh := sessionAffinityKey{session: "session-3-fresh", model: "model"}
	if got := service.applySessionAffinity(fresh, []channel{
		affinityChannel(1, "openai", 10, 1, "key-a"),
		affinityChannel(2, "opencode_go", 5, 1, "key-a"),
	}); got[0].id != 1 {
		t.Fatalf("unassigned session did not follow the candidate order: %v", affinityIDs(got))
	}
	if _, ok := service.sessionAffinity.get(sessionAffinityKey{session: "", model: "model"}); ok {
		t.Fatal("pinned an empty session")
	}
}

func TestSessionAffinityOrdersTheFallbackBehindTheBoundChannel(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	key := sessionAffinityKey{session: "session-6", model: "model"}
	candidates := []channel{
		affinityChannel(1, "opencode_go", 10, 1, "key-a"),
		affinityChannel(2, "opencode_go", 10, 1, "key-a"),
	}
	service.bindSessionChannel(key, affinityChannel(2, "opencode_go", 10, 1, "key-a"))
	// The bound channel is retried first and the channels without affinity for the
	// session follow in their own order, which is what the retry loop walks.
	got := service.applySessionAffinity(key, candidates)
	if want := []int64{2, 1}; !slices.Equal(affinityIDs(got), want) {
		t.Fatalf("affinity order = %v, want %v", affinityIDs(got), want)
	}
	if _, hasPin := service.sessionAffinity.get(key); !hasPin {
		t.Fatal("applying affinity dropped the assignment")
	}
	for _, id := range affinityIDs(got) {
		if id != 1 && id != 2 {
			t.Fatalf("reordering lost or duplicated a candidate: %v", affinityIDs(got))
		}
	}
}

func TestSessionAffinityBindsAnyProvider(t *testing.T) {
	service := &Service{sessionAffinity: newSessionAffinityCache(time.Minute, 64)}
	key := sessionAffinityKey{session: "session-4", model: "model"}
	service.bindSessionChannel(key, affinityChannel(1, "openai", 10, 1, "key-a"))
	if target, ok := service.sessionAffinity.get(key); !ok || target.channelID != 1 || target.keyID != "key-a" {
		t.Fatalf("target = %#v ok=%v", target, ok)
	}
	service.bindSessionChannel(key, affinityChannel(2, "opencode_go", 10, 1, "key-a"))
	if target, ok := service.sessionAffinity.get(key); !ok || target.channelID != 2 || target.keyID != "key-a" {
		t.Fatalf("target = %#v ok=%v", target, ok)
	}
	// A failover moves the conversation's affinity to the channel that answered.
	service.bindSessionChannel(key, affinityChannel(3, "anthropic", 10, 1, "key-a"))
	if got := service.applySessionAffinity(key, []channel{
		affinityChannel(3, "anthropic", 10, 1, "key-a"),
		affinityChannel(2, "opencode_go", 10, 1, "key-a"),
	}); got[0].id != 3 {
		t.Fatalf("affinity did not follow the failover: %v", affinityIDs(got))
	}
	service.bindSessionChannel(key, channel{})
	if target, ok := service.sessionAffinity.get(key); !ok || target.channelID != 3 {
		t.Fatalf("a channel without an id replaced the assignment: %#v ok=%v", target, ok)
	}
}

func TestSessionAffinityWithoutCacheLeavesChannelsAlone(t *testing.T) {
	service := &Service{}
	channels := []channel{affinityChannel(2, "opencode_go", 10, 1, "key-a"), affinityChannel(1, "opencode_go", 10, 1, "key-a")}
	if got := service.applySessionAffinity(sessionAffinityKey{session: "session-5"}, channels); got[0].id != 2 {
		t.Fatalf("candidate order changed: %v", affinityIDs(got))
	}
}

func TestSessionAffinityCacheExpiresAndEvicts(t *testing.T) {
	cache := newSessionAffinityCache(time.Millisecond, 2)
	first := sessionAffinityKey{session: "session-a"}
	cache.bind(first, sessionAffinityTarget{channelID: 1})
	if target, ok := cache.get(first); !ok || target.channelID != 1 {
		t.Fatalf("target = %#v ok=%v", target, ok)
	}
	// A repeated bind refreshes the assignment without reordering the cache.
	cache.bind(first, sessionAffinityTarget{channelID: 2})
	if target, ok := cache.get(first); !ok || target.channelID != 2 {
		t.Fatalf("rebind target = %#v ok=%v", target, ok)
	}
	cache.bind(sessionAffinityKey{session: "session-b"}, sessionAffinityTarget{channelID: 3})
	cache.bind(sessionAffinityKey{session: "session-c"}, sessionAffinityTarget{channelID: 4})
	if _, ok := cache.get(first); ok {
		t.Fatal("oldest assignment was not evicted")
	}
	if target, ok := cache.get(sessionAffinityKey{session: "session-c"}); !ok || target.channelID != 4 {
		t.Fatalf("newest assignment = %#v ok=%v", target, ok)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok := cache.get(sessionAffinityKey{session: "session-c"}); ok {
		t.Fatal("assignment outlived its ttl")
	}
	if got := newSessionAffinityCache(0, 0); got.ttl != sessionAffinityTTL || got.limit != maxSessionAffinityEntries {
		t.Fatalf("defaults = %s/%d", got.ttl, got.limit)
	}
	cache.bind(sessionAffinityKey{}, sessionAffinityTarget{channelID: 5})
	if len(cache.entries) != 2 {
		t.Fatalf("empty session was cached: %d entries", len(cache.entries))
	}
}

func TestSessionAffinityRenewalExtendsLease(t *testing.T) {
	now := time.Now()
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.now = func() time.Time { return now }
	key := sessionAffinityKey{session: "session-renew", model: "model"}
	target := sessionAffinityTarget{channelID: 5, keyID: "key-a"}
	cache.bind(key, target)

	// A request served before the lease lapses renews it without moving the
	// session and without duplicating its place in the eviction order.
	now = now.Add(50 * time.Minute)
	if got, ok := cache.get(key); !ok || got != target {
		t.Fatalf("lease did not survive the renewal window: %#v ok=%v", got, ok)
	}
	cache.bind(key, target)
	if len(cache.order) != 1 {
		t.Fatalf("renewal duplicated the entry: %d", len(cache.order))
	}
	now = now.Add(50 * time.Minute)
	if got, ok := cache.get(key); !ok || got != target {
		t.Fatalf("renewed lease expired anyway: %#v ok=%v", got, ok)
	}
	// An entry that is not renewed lapses after one ttl.
	stale := sessionAffinityKey{session: "session-stale", model: "model"}
	cache.bind(stale, target)
	now = now.Add(time.Hour + time.Second)
	if _, ok := cache.get(stale); ok {
		t.Fatal("idle assignment outlived its unrenewed lease")
	}
}

func TestSessionAffinitySnapshotSnapshotsOnlyLiveAssignments(t *testing.T) {
	now := time.Now()
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.now = func() time.Time { return now }
	if _, changed := cache.snapshot(); changed {
		t.Fatal("an untouched cache reported a change")
	}
	cache.bind(sessionAffinityKey{session: "session-live", userID: "user-1", model: "model"}, sessionAffinityTarget{channelID: 3, keyID: "key-b"})
	cache.bind(sessionAffinityKey{session: "session-old", model: "model"}, sessionAffinityTarget{channelID: 4})
	records, changed := cache.snapshot()
	if !changed || len(records) != 2 {
		t.Fatalf("snapshot = %d records changed=%v", len(records), changed)
	}
	if _, changed := cache.snapshot(); changed {
		t.Fatal("snapshot reported a change twice")
	}
	now = now.Add(time.Hour + time.Second)
	records, changed = cache.snapshot()
	if !changed || len(records) != 0 {
		t.Fatalf("expired snapshot = %d records changed=%v", len(records), changed)
	}
	if len(cache.entries) != 0 || len(cache.order) != 0 {
		t.Fatalf("expired entries were kept: %d/%d", len(cache.entries), len(cache.order))
	}
}

func TestSessionAffinityRestoreSkipsExpiredAndForeigns(t *testing.T) {
	now := time.Now()
	cache := newSessionAffinityCache(time.Hour, 2)
	cache.now = func() time.Time { return now }
	key := sessionAffinityKey{session: "session-restored", userID: "user-1", model: "model"}
	restored := cache.restore([]sessionAffinityRecord{
		{Session: "session-restored", UserID: "user-1", Model: "model", ChannelID: 9, KeyID: "key-c", ExpiresAt: now.Add(time.Minute)},
		{Session: "session-expired", Model: "model", ChannelID: 9, ExpiresAt: now.Add(-time.Minute)},
		{Session: "", Model: "model", ChannelID: 9, ExpiresAt: now.Add(time.Minute)},
		{Session: "session-nocchannel", Model: "model", ExpiresAt: now.Add(time.Minute)},
		{Session: "session-over-limit", Model: "model", ChannelID: 9, ExpiresAt: now.Add(time.Minute)},
	})
	if restored != 2 {
		t.Fatalf("restored = %d, want the two live assignments within the bound", restored)
	}
	if target, ok := cache.get(key); !ok || target.channelID != 9 || target.keyID != "key-c" {
		t.Fatalf("restored target = %#v ok=%v", target, ok)
	}
	// A restored snapshot is already on disk, so it is not written back.
	if _, changed := cache.snapshot(); changed {
		t.Fatal("restore marked the cache dirty")
	}
	// The user id is part of the key, so another user cannot inherit the pin.
	if _, ok := cache.get(sessionAffinityKey{session: "session-restored", userID: "user-2", model: "model"}); ok {
		t.Fatal("assignment leaked to another user")
	}
}

func TestSessionAffinityPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cache := newSessionAffinityCache(time.Hour, 8)
	service := &Service{cfg: Config{SessionAffinityDir: dir}, sessionAffinity: cache}
	service.loadSessionAffinity()
	service.flushSessionAffinity()
	if _, err := os.Stat(filepath.Join(dir, sessionAffinityFileName)); !os.IsNotExist(err) {
		t.Fatalf("an unchanged cache wrote a snapshot: %v", err)
	}

	key := sessionAffinityKey{session: "session-restart", userID: "user-1", groupID: "group-1", model: "model"}
	cache.bind(key, sessionAffinityTarget{channelID: 12, keyID: "key-d"})
	service.flushSessionAffinity()
	info, err := os.Stat(filepath.Join(dir, sessionAffinityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot mode = %o, want 600", info.Mode().Perm())
	}

	restarted := &Service{cfg: Config{SessionAffinityDir: dir}, sessionAffinity: newSessionAffinityCache(time.Hour, 8)}
	restarted.loadSessionAffinity()
	target, ok := restarted.sessionAffinity.get(key)
	if !ok || target.channelID != 12 || target.keyID != "key-d" {
		t.Fatalf("restored target = %#v ok=%v", target, ok)
	}
	if _, ok := restarted.sessionAffinity.get(sessionAffinityKey{session: "session-restart", userID: "user-2", groupID: "group-1", model: "model"}); ok {
		t.Fatal("restored assignment ignored the user id")
	}
	restarted.flushSessionAffinity()
	records, changed := restarted.sessionAffinity.snapshot()
	if changed || len(records) != 0 {
		t.Fatalf("a restored snapshot was rewritten: %d changed=%v", len(records), changed)
	}
}

func TestSessionAffinitySnapshotToleratesCorruptionAndWriteErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, sessionAffinityFileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := &Service{cfg: Config{SessionAffinityDir: dir}, sessionAffinity: newSessionAffinityCache(time.Hour, 8)}
	service.loadSessionAffinity()
	if len(service.sessionAffinity.entries) != 0 {
		t.Fatal("corrupt snapshot produced assignments")
	}

	// A directory that cannot be created keeps the changes pending for the next
	// flush instead of dropping them.
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	service.cfg.SessionAffinityDir = filepath.Join(blocked, "nested")
	service.sessionAffinity.bind(sessionAffinityKey{session: "session-pending", model: "model"}, sessionAffinityTarget{channelID: 1})
	service.flushSessionAffinity()
	if records, changed := service.sessionAffinity.snapshot(); !changed || len(records) != 1 {
		t.Fatalf("failed write dropped the change: %d changed=%v", len(records), changed)
	}
}
