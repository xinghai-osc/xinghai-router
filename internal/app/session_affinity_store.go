package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	// sessionAffinityFileName is the snapshot the gateway restores on startup.
	sessionAffinityFileName = "sessions.json"
	// sessionAffinityFlushInterval is how often a changed snapshot is written.
	sessionAffinityFlushInterval = 30 * time.Second
)

// sessionAffinityFile is the on-disk shape of the affinity table. The timestamp
// is informational: every record carries its own lease expiry, so a snapshot
// copied between hosts cannot resurrect assignments that already lapsed.
type sessionAffinityFile struct {
	SavedAt  time.Time               `json:"saved_at"`
	Sessions []sessionAffinityRecord `json:"sessions"`
}

// sessionAffinityPath returns the snapshot path, or "" when persistence is off.
func (s *Service) sessionAffinityPath() string {
	if s == nil || s.cfg.SessionAffinityDir == "" {
		return ""
	}
	return filepath.Join(s.cfg.SessionAffinityDir, sessionAffinityFileName)
}

// loadSessionAffinity restores the assignments written by a previous process so
// a restart or redeploy does not reshuffle every conversation onto a new
// channel, which would discard the upstream prompt caches they built. A missing,
// unreadable or stale snapshot is not fatal: the gateway simply starts empty.
func (s *Service) loadSessionAffinity() {
	path := s.sessionAffinityPath()
	if path == "" || s.sessionAffinity == nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("session affinity restore: %v", err)
		}
		return
	}
	var file sessionAffinityFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("session affinity restore: %v", err)
		return
	}
	restored := s.sessionAffinity.restore(file.Sessions)
	if restored > 0 {
		log.Printf("event=session_affinity_restored sessions=%d path=%s", restored, path)
	}
}

// flushSessionAffinity writes the current snapshot when something changed since
// the last write. It is called periodically and once on shutdown; writes go
// through a temporary file so a crash can never leave a truncated snapshot.
func (s *Service) flushSessionAffinity() {
	path := s.sessionAffinityPath()
	if path == "" || s.sessionAffinity == nil {
		return
	}
	s.sessionAffinityWrite.Lock()
	defer s.sessionAffinityWrite.Unlock()
	records, changed := s.sessionAffinity.snapshot()
	if !changed {
		return
	}
	if err := writeSessionAffinityFile(path, records); err != nil {
		log.Printf("session affinity snapshot: %v", err)
		// Keep the changes marked so the next tick retries them.
		s.sessionAffinity.markDirty()
	}
}

func writeSessionAffinityFile(path string, records []sessionAffinityRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if records == nil {
		records = []sessionAffinityRecord{}
	}
	data, err := json.Marshal(sessionAffinityFile{SavedAt: time.Now(), Sessions: records})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// list returns every live assignment in renewal order, dropping the ones whose
// lease already lapsed so the admin view and its total agree with what the
// gateway would actually route.
func (c *sessionAffinityCache) list() []sessionAffinityRecord {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	live := make([]sessionAffinityRecord, 0, len(c.order))
	for _, key := range c.order {
		entry, ok := c.entries[key]
		if !ok || !now.Before(entry.expires) {
			continue
		}
		live = append(live, sessionAffinityRecord{
			Session:   key.session,
			UserID:    key.userID,
			GroupID:   key.groupID,
			Model:     key.model,
			ChannelID: entry.target.channelID,
			KeyID:     entry.target.keyID,
			ExpiresAt: entry.expires,
		})
	}
	return live
}

// purge drops one assignment. It reports whether the assignment existed, which
// lets the admin API distinguish a deletion from a session that was never
// pinned. The change is marked dirty so the snapshot on disk drops it too.
func (c *sessionAffinityCache) purge(key sessionAffinityKey) bool {
	if c == nil || key.session == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; !ok {
		return false
	}
	delete(c.entries, key)
	remaining := c.order[:0]
	for _, existing := range c.order {
		if existing != key {
			remaining = append(remaining, existing)
		}
	}
	c.order = remaining
	c.dirty = true
	return true
}

// purgeSession drops every assignment whose session id matches the given value,
// regardless of the user, group or model it is scoped to.
func (c *sessionAffinityCache) purgeSession(session string) int {
	if c == nil || session == "" {
		return 0
	}
	return c.purgeMatching(func(key sessionAffinityKey) bool { return key.session == session })
}

// purgeUser drops every assignment of one user and reports how many were
// removed.
func (c *sessionAffinityCache) purgeUser(userID string) int {
	if c == nil || userID == "" {
		return 0
	}
	return c.purgeMatching(func(key sessionAffinityKey) bool { return key.userID == userID })
}

// clear drops every assignment and reports how many were removed. The empty
// state is marked dirty so the snapshot on disk is emptied as well.
func (c *sessionAffinityCache) clear() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := len(c.entries)
	c.entries = map[sessionAffinityKey]sessionAffinityEntry{}
	c.order = nil
	if removed > 0 {
		c.dirty = true
	}
	return removed
}

func (c *sessionAffinityCache) purgeMatching(match func(sessionAffinityKey) bool) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := c.order[:0]
	removed := 0
	for _, key := range c.order {
		if !match(key) {
			remaining = append(remaining, key)
			continue
		}
		delete(c.entries, key)
		removed++
	}
	c.order = remaining
	if removed > 0 {
		c.dirty = true
	}
	return removed
}

// startSessionAffinityFlusher persists renewed assignments on a fixed cadence.
// The shutdown flush in Close is what makes the last renewals durable; this loop
// bounds how much a crash can lose in between.
func (s *Service) startSessionAffinityFlusher(ctx context.Context) {
	if s.sessionAffinityPath() == "" || s.sessionAffinity == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(sessionAffinityFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.flushSessionAffinity()
			}
		}
	}()
}
