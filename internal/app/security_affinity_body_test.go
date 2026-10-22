package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConsoleRejectedBodyDoesNotClearSessionAffinity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		length int64
		status int
		err    error
	}{
		{"malformed", `{"user_id":`, -1, http.StatusBadRequest, nil},
		{"invalid-user", `{"user_id":"invalid"}`, -1, http.StatusBadRequest, nil},
		{"known-oversized", `{"user_id":1}`, defaultConsoleBodyMaxBytes + 1, http.StatusRequestEntityTooLarge, nil},
		{"unknown-oversized", `{"user_id":1}` + strings.Repeat(" ", defaultConsoleBodyMaxBytes), -1, http.StatusRequestEntityTooLarge, nil},
		{"read-timeout", `{"user_id":1}`, -1, http.StatusRequestTimeout, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := newSessionAffinityCache(time.Hour, 8)
			cache.bind(sessionAffinityKey{session: "first", userID: "1", model: "model"}, sessionAffinityTarget{channelID: 1})
			cache.bind(sessionAffinityKey{session: "second", userID: "2", model: "model"}, sessionAffinityTarget{channelID: 2})
			before := cache.list()
			cache.snapshot()
			s := &Service{sessionAffinity: cache}
			r := httptest.NewRequest(http.MethodDelete, "/admin/session-affinity", strings.NewReader(tc.body))
			r.ContentLength = tc.length
			if tc.err != nil {
				r.Body = io.NopCloser(io.MultiReader(strings.NewReader(tc.body), firstTokenReadFunc(func([]byte) (int, error) { return 0, tc.err })))
			}
			r.Header.Set("X-Xinghai-Request", "1")
			w := httptest.NewRecorder()
			s.consoleSecurity(http.HandlerFunc(s.clearSessionAffinity)).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if after := cache.list(); !reflect.DeepEqual(after, before) {
				t.Fatalf("rejected body changed affinity: before=%+v after=%+v", before, after)
			}
			if _, changed := cache.snapshot(); changed {
				t.Fatal("rejected body marked affinity for persistence")
			}
		})
	}
}
