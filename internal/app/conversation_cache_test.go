package app

import (
	"encoding/json"
	"net/http/httptest"
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
