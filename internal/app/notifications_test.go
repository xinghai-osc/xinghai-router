package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateNotificationRejectsInvalidBeforeDatabase(t *testing.T) {
	cases := []string{
		`not-json`,
		`{}`,
		`{"title":""}`,
		`{"title":"   "}`,
		`{"title":"` + strings.Repeat("t", 201) + `"}`,
		`{"title":"ok","content":"` + strings.Repeat("c", 5001) + `"}`,
		`{"title":"ok","unknown":1}`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/notifications", strings.NewReader(body))
		(&Service{}).createNotification(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestUpdateNotificationRejectsInvalidBeforeDatabase(t *testing.T) {
	cases := []string{
		`not-json`,
		`{"title":""}`,
		`{"title":"` + strings.Repeat("t", 201) + `"}`,
		`{"title":"ok","content":"` + strings.Repeat("c", 5001) + `"}`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/notifications/some-id", strings.NewReader(body))
		(&Service{}).updateNotification(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}
