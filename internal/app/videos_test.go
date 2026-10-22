package app

import (
	"context"
	"strings"
	"testing"
)

func TestVideoTaskValidation(t *testing.T) {
	for _, status := range []string{"queued", "in_progress", "completed", "failed"} {
		id, got, err := parseVideoTask([]byte(`{"id":"video_123-A","status":"` + status + `"}`))
		if err != nil || id != "video_123-A" || got != status {
			t.Fatalf("status=%s id=%s got=%s err=%v", status, id, got, err)
		}
	}
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"id":"video_1"}`, `{"id":"video_1","status":"unknown"}`,
		`{"id":"../secret","status":"queued"}`, `{"id":"x/y","status":"queued"}`,
		`{"id":"x%2Fy","status":"queued"}`, `{"id":"x?key=1","status":"queued"}`,
		`{"id":42,"status":"queued"}`, `{"id":"v","status":"queued","error":{}}`,
		`{"id":"v","status":"completed"} {}`, strings.Repeat(" ", maxVideoTaskBody+1),
	} {
		if _, _, err := parseVideoTask([]byte(body)); err == nil {
			t.Errorf("accepted invalid body: %.100s", body)
		}
	}
	for _, id := range []string{"", "..", "a/b", "a.b", "a#b", "a?b", "a%2fb", " a", "汉字", strings.Repeat("a", 257)} {
		if validVideoTaskID(id) {
			t.Errorf("accepted id=%q", id)
		}
	}
	if !validVideoTaskID(strings.Repeat("a", 256)) {
		t.Fatal("rejected boundary ID")
	}
}

func TestVideoContentQuery(t *testing.T) {
	for _, q := range []string{"", "variant=video", "variant=thumbnail", "variant=spritesheet"} {
		if got, err := videoContentQuery(q, true); err != nil || got != q {
			t.Errorf("query=%q got=%q err=%v", q, got, err)
		}
	}
	for _, q := range []string{"variant=other", "variant=", "variant=video&variant=thumbnail", "url=https://evil.test", "variant=video&url=x", "variant=%xx", "variant=video;url=x"} {
		if _, err := videoContentQuery(q, true); err == nil {
			t.Errorf("accepted query=%q", q)
		}
	}
	if _, err := videoContentQuery("variant=video", false); err == nil {
		t.Fatal("status endpoint accepted query")
	}
}

func TestVideoBaseURL(t *testing.T) {
	for _, value := range []string{"https://example.test", "http://127.0.0.1:123/base"} {
		if got, err := videoBaseURL(value + "/"); err != nil || got != value {
			t.Errorf("base=%q got=%q err=%v", value, got, err)
		}
	}
	for _, value := range []string{"", "file:///tmp/a", "https://user:secret@example.test", "https://example.test?key=secret", "https://example.test#fragment", "https://example.test/a%2fb"} {
		if _, err := videoBaseURL(value); err == nil {
			t.Errorf("accepted base=%q", value)
		}
	}
}

func TestVideoRegistrationRejectsInvalidWithoutDatabase(t *testing.T) {
	s := &Service{}
	for _, body := range []string{`{}`, `{"id":"video_1","status":"failed"}`, `{"id":"../secret","status":"queued"}`, `{"id":"video_1","status":"queued"}`} {
		if _, err := s.registerVideoTask(context.Background(), keyContext{}, channel{}, "", []byte(body)); err == nil {
			t.Fatalf("accepted invalid registration: %s", body)
		}
	}
}

func TestVideoTaskResponsePreservesFields(t *testing.T) {
	body, err := videoTaskResponse([]byte(`{"id":"upstream","status":"queued","created_at":9007199254740993,"nested":{"id":"keep"}}`), "video_local")
	if err != nil || !strings.Contains(string(body), `"id":"video_local"`) || !strings.Contains(string(body), `9007199254740993`) || !strings.Contains(string(body), `"nested":{"id":"keep"}`) {
		t.Fatalf("body=%s err=%v", body, err)
	}
}
