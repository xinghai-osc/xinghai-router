package app

import (
	"testing"
	"time"
)

func TestSessionAffinityListDropsExpiredAssignments(t *testing.T) {
	now := time.Now()
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.now = func() time.Time { return now }
	cache.bind(sessionAffinityKey{session: "session-live", userID: "user-1", model: "model"}, sessionAffinityTarget{channelID: 3, keyID: "key-b"})
	cache.bind(sessionAffinityKey{session: "session-old", model: "model"}, sessionAffinityTarget{channelID: 4})

	records := cache.list()
	if len(records) != 2 {
		t.Fatalf("list = %d records, want both live assignments", len(records))
	}
	if records[0].Session != "session-live" || records[0].ChannelID != 3 || records[0].KeyID != "key-b" {
		t.Fatalf("first record = %#v", records[0])
	}

	// An assignment whose lease lapsed is not part of the admin view, because
	// the gateway would not route to it either.
	now = now.Add(time.Hour + time.Second)
	if records := cache.list(); len(records) != 0 {
		t.Fatalf("list = %#v, want no live assignments", records)
	}
	if got := newSessionAffinityCache(time.Minute, 4).list(); len(got) != 0 {
		t.Fatalf("empty cache listed %#v", got)
	}
}

func TestSessionAffinityPurgeDropsSingleAssignments(t *testing.T) {
	now := time.Now()
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.now = func() time.Time { return now }
	key := sessionAffinityKey{session: "session-purge", userID: "user-1", groupID: "group-1", model: "model"}
	cache.bind(key, sessionAffinityTarget{channelID: 5, keyID: "key-a"})
	// The same session id under another model is a different assignment.
	cache.bind(sessionAffinityKey{session: "session-purge", userID: "user-1", groupID: "group-1", model: "other"}, sessionAffinityTarget{channelID: 6})

	if !cache.purge(key) {
		t.Fatal("purge did not report the assignment it removed")
	}
	if _, ok := cache.get(key); ok {
		t.Fatal("purged assignment is still routable")
	}
	if len(cache.order) != 1 || cache.order[0].model != "other" {
		t.Fatalf("purge dropped the wrong assignments: %#v", cache.order)
	}
	if cache.purge(key) {
		t.Fatal("purging a removed assignment reported a removal")
	}
	if cache.purge(sessionAffinityKey{session: ""}) {
		t.Fatal("purging an empty session reported a removal")
	}
	if _, changed := cache.snapshot(); !changed {
		t.Fatal("purge did not schedule the snapshot to drop the assignment")
	}
}

func TestSessionAffinityPurgeSessionAndUser(t *testing.T) {
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.bind(sessionAffinityKey{session: "shared", userID: "user-1", model: "a"}, sessionAffinityTarget{channelID: 1})
	cache.bind(sessionAffinityKey{session: "shared", userID: "user-1", model: "b"}, sessionAffinityTarget{channelID: 2})
	cache.bind(sessionAffinityKey{session: "shared", userID: "user-2", model: "a"}, sessionAffinityTarget{channelID: 3})
	cache.bind(sessionAffinityKey{session: "other", userID: "user-1", model: "a"}, sessionAffinityTarget{channelID: 4})

	if removed := cache.purgeSession("missing"); removed != 0 {
		t.Fatalf("purgeSession removed %d assignments for an unknown session", removed)
	}
	// One session id always belongs to one user in practice, so the session
	// purge covers every model that user pinned under it.
	if removed := cache.purgeSession("shared"); removed != 3 {
		t.Fatalf("purgeSession removed %d assignments, want 3", removed)
	}
	if len(cache.list()) != 1 || cache.list()[0].ChannelID != 4 {
		t.Fatalf("remaining assignments = %#v", cache.list())
	}
	if removed := cache.purgeUser("user-1"); removed != 1 {
		t.Fatalf("purgeUser removed %d assignments, want 1", removed)
	}
	if removed := cache.purgeUser(""); removed != 0 {
		t.Fatalf("purgeUser removed %d assignments for an empty user", removed)
	}
	if len(cache.entries) != 0 {
		t.Fatalf("entries were left behind: %#v", cache.entries)
	}
}

func TestSessionAffinityClearEmptiesCacheAndSnapshot(t *testing.T) {
	cache := newSessionAffinityCache(time.Hour, 8)
	cache.bind(sessionAffinityKey{session: "session-a", model: "model"}, sessionAffinityTarget{channelID: 1})
	cache.bind(sessionAffinityKey{session: "session-b", model: "model"}, sessionAffinityTarget{channelID: 2})
	if _, changed := cache.snapshot(); !changed {
		t.Fatal("bindings were not scheduled for persistence")
	}

	if removed := cache.clear(); removed != 2 {
		t.Fatalf("clear removed %d assignments, want 2", removed)
	}
	if len(cache.entries) != 0 || len(cache.order) != 0 {
		t.Fatalf("clear left entries behind: %d/%d", len(cache.entries), len(cache.order))
	}
	records, changed := cache.snapshot()
	if !changed || len(records) != 0 {
		t.Fatalf("snapshot after clear = %d records changed=%v", len(records), changed)
	}
	// A cleared cache that is cleared again has nothing to persist.
	if removed := cache.clear(); removed != 0 {
		t.Fatalf("second clear removed %d assignments", removed)
	}
	if _, changed := cache.snapshot(); changed {
		t.Fatal("clearing an empty cache scheduled a snapshot write")
	}
	var nilCache *sessionAffinityCache
	if nilCache.clear() != 0 {
		t.Fatal("clearing a missing cache reported a removal")
	}
}
