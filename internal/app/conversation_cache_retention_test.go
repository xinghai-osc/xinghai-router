package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConversationCacheRetentionWindow(t *testing.T) {
	for _, hours := range []int{1, 24, 168, maxConversationCacheRetentionHours} {
		t.Run(time.Duration(hours).String(), func(t *testing.T) {
			dir := t.TempDir()
			now := time.Date(2026, 11, 14, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
			settings := conversationCacheSettings{RetentionHours: hours}
			cutoff := settings.cutoff(now)
			for _, entry := range []conversationFile{
				{ID: "expired", CreatedAt: cutoff.Add(-time.Nanosecond)},
				{ID: "boundary", CreatedAt: cutoff},
				{ID: "live", CreatedAt: cutoff.Add(time.Nanosecond)},
			} {
				if err := writeConversationFile(dir, entry); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := readConversationFiles(dir, now, settings)
			if err != nil || len(entries) != 1 || entries[0].ID != "live" {
				t.Fatalf("entries=%+v err=%v, want only live", entries, err)
			}
			if removed := cleanupConversationCacheDir(dir, now, settings); removed != 2 {
				t.Fatalf("removed=%d, want 2", removed)
			}
			entries, err = readConversationFiles(dir, now, conversationCacheSettings{Retain: true})
			if err != nil || len(entries) != 1 || entries[0].ID != "live" {
				t.Fatalf("remaining entries=%+v err=%v, want only live", entries, err)
			}
		})
	}
}

func TestConversationCacheIndexExpiresAndReloadsPolicy(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, entry := range []conversationFile{
		{ID: "boundary", CreatedAt: now.Add(-time.Hour + time.Second)},
		{ID: "old", CreatedAt: now.Add(-48 * time.Hour)},
	} {
		if err := writeConversationFile(dir, entry); err != nil {
			t.Fatal(err)
		}
	}
	index := newConversationCacheIndex()
	for _, test := range []struct {
		now      time.Time
		settings conversationCacheSettings
		want     int
	}{
		{now, conversationCacheSettings{RetentionHours: 1}, 1},
		{now.Add(time.Second), conversationCacheSettings{RetentionHours: 1}, 0},
		{now.Add(2 * time.Second), conversationCacheSettings{RetentionHours: 72}, 2},
		{now.Add(3 * time.Second), conversationCacheSettings{RetentionHours: 1}, 0},
		{now.Add(4 * time.Second), conversationCacheSettings{Retain: true}, 2},
	} {
		entries, err := index.get(dir, test.now, test.settings)
		if err != nil || len(entries) != test.want {
			t.Fatalf("settings=%+v now=%v entries=%+v err=%v, want %d", test.settings, test.now, entries, err, test.want)
		}
	}
}

func TestConversationCacheRetentionDetailAndList(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, entry := range []conversationFile{
		{ID: "old", CreatedAt: now.Add(-25 * time.Hour)},
		{ID: "live", CreatedAt: now.Add(-23 * time.Hour)},
	} {
		if err := writeConversationFile(dir, entry); err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{cfg: Config{ConversationCacheDir: dir}, conversationCacheData: newTTLCache[struct{}, conversationCacheSettings](time.Hour)}
	for _, test := range []struct {
		settings   conversationCacheSettings
		wantStatus int
		wantTotal  int
	}{
		{conversationCacheSettings{}, 404, 1},
		{conversationCacheSettings{RetentionHours: 48}, 200, 2},
		{conversationCacheSettings{Retain: true, RetentionHours: 1}, 200, 2},
	} {
		s.conversationCacheData.store(struct{}{}, test.settings)
		r := httptest.NewRequest("GET", "/admin/conversation-cache/old", nil)
		r.SetPathValue("id", "old")
		w := httptest.NewRecorder()
		s.getConversationCacheDetail(w, r)
		if w.Code != test.wantStatus {
			t.Fatalf("detail status=%d body=%s, want %d", w.Code, w.Body, test.wantStatus)
		}
		w = httptest.NewRecorder()
		s.listConversationCache(w, httptest.NewRequest("GET", "/admin/conversation-cache", nil))
		var page struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || page.Total != test.wantTotal {
			t.Fatalf("list status=%d body=%s err=%v, want total %d", w.Code, w.Body, err, test.wantTotal)
		}
	}
}

func TestConversationCacheRetentionRejectsInvalidHours(t *testing.T) {
	for _, value := range []string{"0", "-1", "87601", "1.5", `"24"`, "99999999999999999999999999999"} {
		s := &Service{}
		r := httptest.NewRequest("PUT", "/admin/conversation-cache/settings", strings.NewReader(`{"conversation_cache_retention_hours":`+value+`}`))
		w := httptest.NewRecorder()
		s.updateConversationCacheSettings(w, r)
		if w.Code != 400 {
			t.Fatalf("hours=%s status=%d body=%s, want 400", value, w.Code, w.Body)
		}
	}
	s := &Service{}
	settings, err := s.readConversationCacheSettings(context.Background())
	if err != nil || settings.RetentionHours != 24 {
		t.Fatalf("defaults=%+v err=%v", settings, err)
	}
}

func TestConversationCacheCleanupPreservesUnrecognizedFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-48 * time.Hour)
	if err := writeConversationFile(dir, conversationFile{ID: "expired", CreatedAt: old}); err != nil {
		t.Fatal(err)
	}
	day := filepath.Join(dir, old.Format("2006-01-02"))
	for _, name := range []string{"broken.json", "writing.json.tmp", "unknown.json"} {
		if err := os.WriteFile(filepath.Join(day, name), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if removed := cleanupConversationCacheDir(dir, now, conversationCacheSettings{}); removed != 1 {
		t.Fatalf("removed=%d, want 1", removed)
	}
	files, err := os.ReadDir(day)
	if err != nil || len(files) != 3 {
		t.Fatalf("remaining files=%v err=%v, want 3", files, err)
	}
}
