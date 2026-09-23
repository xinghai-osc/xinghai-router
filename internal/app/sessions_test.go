package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionCookieSecurity(t *testing.T) {
	s := &Service{cfg: Config{SessionCookieSecure: true}}
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "xh_session_fake", time.Now().Add(time.Hour))
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies=%v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Value != "xh_session_fake" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("unsafe cookie: %v", cookie)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || cookies[1].Name != "xinghai.admin-token" || cookies[1].MaxAge != -1 {
		t.Fatal("missing no-store or legacy cookie deletion")
	}
	rec = httptest.NewRecorder()
	s.clearSessionCookie(rec)
	cookie = rec.Result().Cookies()[0]
	if cookie.MaxAge != -1 || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" {
		t.Fatalf("unsafe deletion: %v", cookie)
	}
}

func TestSessionTokenRejectsLegacyBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/account/me", nil)
	req.Header.Set("Authorization", "Bearer xh_session_old")
	req.AddCookie(&http.Cookie{Name: "xinghai.admin-token", Value: "xh_session_old"})
	if sessionToken(req) != "" {
		t.Fatal("legacy browser credential accepted")
	}
	rec := httptest.NewRecorder()
	(&Service{}).account(func(http.ResponseWriter, *http.Request) { t.Fatal("legacy session reached handler") }).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "xh_session_new"})
	if sessionToken(req) != "xh_session_new" {
		t.Fatal("new session cookie not selected")
	}
	if bearer(req) != "xh_session_old" {
		t.Fatal("gateway bearer extraction changed")
	}
}

func TestConsoleCSRFBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, path, method, origin, site, marker string
		want                                     int
	}{
		{"login form", "/auth/login", "POST", "", "", "", 403},
		{"cookie mutation", "/account/preferences", "PUT", "", "", "", 403},
		{"cross site", "/auth/login", "POST", "https://evil.example", "cross-site", "1", 403},
		{"sibling domain", "/admin/users", "POST", "https://evil.example", "same-site", "1", 403},
		{"origin null", "/admin/users", "POST", "null", "", "1", 403},
		{"same origin", "/admin/users", "POST", "https://router.example", "same-origin", "1", 204},
		{"CLI", "/account/preferences", "PUT", "", "", "1", 204},
		{"oauth callback", "/auth/oauth/github/callback", "GET", "", "cross-site", "", 204},
		{"gateway exempt", "/v1/chat/completions", "POST", "", "", "", 204},
		{"signed payment exempt", "/payments/epay/notify", "POST", "", "", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "https://router.example"+tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("X-Xinghai-Request", tc.marker)
			rec := httptest.NewRecorder()
			(&Service{}).consoleSecurity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDataImportRequiresAdministrator(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/migrate", strings.NewReader(`{"source_dsn":"fake"}`))
	req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{userID: "2", role: "user", permissions: map[string]bool{"system.manage": true, "users.authorize": true}}))
	rec := httptest.NewRecorder()
	(&Service{}).runMigration(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin data import status=%d", rec.Code)
	}
}

func TestRequireRecentAuth(t *testing.T) {
	now := time.Now()
	expired := now.Add(-recentAuthDuration - time.Second)
	future := now.Add(time.Hour)
	for _, tc := range []struct {
		name     string
		verified *time.Time
		marker   string
		want     int
		code     string
	}{
		{"never verified", nil, "1", 403, "reauthentication_required"},
		{"expired", &expired, "1", 403, "reauthentication_required"},
		{"future", &future, "1", 403, "reauthentication_required"},
		{"cross origin GET", &now, "", 403, "csrf_failed"},
		{"verified", &now, "1", 204, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/account/keys/fake/secret", nil)
			req.Header.Set("X-Xinghai-Request", tc.marker)
			req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{userID: "1", reauthenticatedAt: tc.verified}))
			rec := httptest.NewRecorder()
			(&Service{}).requireRecentAuth(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(rec, req)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.code) || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s headers=%v", rec.Code, rec.Body.String(), rec.Header())
			}
		})
	}
}
