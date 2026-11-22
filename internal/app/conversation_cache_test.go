package app

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidConversationCacheID(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../secret", `..\\secret`, "id/part", "id?part"} {
		if validConversationCacheID(id) {
			t.Fatalf("validConversationCacheID(%q) = true, want false", id)
		}
	}
	for _, id := range []string{"entry-a", "550e8400-e29b-41d4-a716-446655440000"} {
		if !validConversationCacheID(id) {
			t.Fatalf("validConversationCacheID(%q) = false, want true", id)
		}
	}
}

func TestReadConversationFilesSkipsBodies(t *testing.T) {
	dir := t.TempDir()
	createdAt := time.Now()
	body := `{"payload":"` + strings.Repeat("x", 1<<20) + `"}`
	if err := writeConversationFile(dir, conversationFile{
		ID:           "large-entry",
		RequestID:    "request-large",
		Model:        "model",
		CreatedAt:    createdAt,
		RequestBody:  json.RawMessage(body),
		ResponseBody: json.RawMessage(body),
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := readConversationFiles(dir, createdAt, conversationCacheSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "large-entry" {
		t.Fatalf("entries = %#v, want one metadata summary", entries)
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "payload") {
		t.Fatal("list summary retained request or response body")
	}
}

func TestConversationCacheIndexReusesSnapshotAndAddsEntries(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	first := conversationFile{ID: "entry-a", Model: "model", CreatedAt: now}
	if err := writeConversationFile(dir, first); err != nil {
		t.Fatal(err)
	}
	index := newConversationCacheIndex()
	entries, err := index.get(dir, now, conversationCacheSettings{})
	if err != nil || len(entries) != 1 {
		t.Fatalf("initial index entries=%d err=%v, want 1 entry", len(entries), err)
	}

	second := conversationFile{ID: "entry-b", Model: "model", CreatedAt: now.Add(time.Second)}
	if err := writeConversationFile(dir, second); err != nil {
		t.Fatal(err)
	}
	index.add(conversationCacheSummary{ID: second.ID, Model: second.Model, CreatedAt: second.CreatedAt})
	entries, err = index.get(dir, now.Add(2*time.Second), conversationCacheSettings{})
	if err != nil || len(entries) != 2 {
		t.Fatalf("updated index entries=%d err=%v, want 2 entries", len(entries), err)
	}
}

func TestListConversationCachePaginatesDeterministically(t *testing.T) {
	dir := t.TempDir()
	createdAt := time.Now()
	for _, entry := range []conversationFile{
		{ID: "entry-a", RequestID: "request-a", Model: "model", CreatedAt: createdAt},
		{ID: "entry-c", RequestID: "request-c", Model: "model", CreatedAt: createdAt},
		{ID: "entry-b", RequestID: "request-b", Model: "model", CreatedAt: createdAt},
	} {
		if err := writeConversationFile(dir, entry); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(dir, createdAt.Format("2006-01-02"), "entry-a.json")
	for path, mode := range map[string]os.FileMode{dir: 0o700, filepath.Dir(path): 0o700, path: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("%s mode=%v err=%v want=%04o", path, info.Mode().Perm(), err, mode)
		}
	}

	service := &Service{cfg: Config{ConversationCacheDir: dir}}
	request := httptest.NewRequest("GET", "/admin/conversation-cache?page=1&page_size=2", nil)
	recorder := httptest.NewRecorder()
	service.listConversationCache(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Total    int `json:"total"`
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Page != 1 || page.PageSize != 2 {
		t.Fatalf("page metadata = total %d, page %d, size %d", page.Total, page.Page, page.PageSize)
	}
	if len(page.Data) != 2 || page.Data[0].ID != "entry-c" || page.Data[1].ID != "entry-b" {
		t.Fatalf("first page IDs = %#v, want [entry-c entry-b]", page.Data)
	}

	request = httptest.NewRequest("GET", "/admin/conversation-cache?page=2&page_size=2", nil)
	recorder = httptest.NewRecorder()
	service.listConversationCache(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("second page status = %d, want 200", recorder.Code)
	}
	if err := json.NewDecoder(recorder.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "entry-a" {
		t.Fatalf("second page IDs = %#v, want [entry-a]", page.Data)
	}
}

func TestRetainConversationCacheReadsOldDays(t *testing.T) {
	dir := t.TempDir()
	old := conversationFile{ID: "old-entry", Model: "model", CreatedAt: time.Now().AddDate(0, 0, -10)}
	if err := writeConversationFile(dir, old); err != nil {
		t.Fatal(err)
	}

	entries, err := readConversationFiles(dir, time.Now(), conversationCacheSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries without retention = %d, want 0", len(entries))
	}

	entries, err = readConversationFiles(dir, time.Now(), conversationCacheSettings{Retain: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "old-entry" {
		t.Fatalf("entries with retention = %#v, want old-entry", entries)
	}

	if _, err := findConversationFile(dir, "old-entry", conversationCacheSettings{}); err == nil {
		t.Fatal("old entry is reachable without retention")
	}
	if entry, err := findConversationFile(dir, "old-entry", conversationCacheSettings{Retain: true}); err != nil || entry.ID != "old-entry" {
		t.Fatalf("old entry with retention = %#v, err = %v", entry, err)
	}
}

func TestCleanupConversationCacheDirHonoursRetention(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, entry := range []conversationFile{
		{ID: "expired", CreatedAt: now.Add(-48 * time.Hour)},
		{ID: "fresh", CreatedAt: now},
	} {
		if err := writeConversationFile(dir, entry); err != nil {
			t.Fatal(err)
		}
	}

	if removed := cleanupConversationCacheDir(dir, now, conversationCacheSettings{Retain: true}); removed != 0 {
		t.Fatalf("removed %d directories with retention, want 0", removed)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("retained directories = %d, want 2", len(entries))
	}

	if removed := cleanupConversationCacheDir(dir, now, conversationCacheSettings{}); removed != 1 {
		t.Fatalf("removed %d directories without retention, want 1", removed)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("remaining directories = %d, want 1", len(entries))
	}
}
