package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateSiteSettingsRejectsOversizedFieldsBeforeDatabase(t *testing.T) {
	cases := []string{
		`{"name":"Site","icon_url":"https://cdn.example.com/` + strings.Repeat("a", 2040) + `"}`,
		`{"name":"Site","smtp_host":"` + strings.Repeat("h", 256) + `"}`,
		`{"name":"Site","geetest_captcha_id":"` + strings.Repeat("g", 257) + `"}`,
		`{"name":"Site","geetest_captcha_key":"` + strings.Repeat("k", 257) + `"}`,
		`{"name":"Site","smtp_password":"` + strings.Repeat("p", 4097) + `"}`,
	}
	for _, body := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/site-settings", strings.NewReader(body))
		(&Service{}).updateSiteSettings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	}
}
