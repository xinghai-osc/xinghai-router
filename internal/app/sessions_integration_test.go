//go:build integration

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIntegrationSecureSessionLifecycle(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	userID, _ := integrationUser(t, db, "secure-session@example.com", 0)
	keyID := integrationKey(t, db, userID, "sk-xh-fake-sensitive-key")
	s := integrationService(t, db)
	s.cfg.SessionCookieSecure = true
	handler := s.Handler()
	request := func(method, path, body string, cookie *http.Cookie, marker bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "https://router.example"+path, strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if marker {
			req.Header.Set("X-Xinghai-Request", "1")
			req.Header.Set("Origin", "https://router.example")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	cookieFrom := func(rec *httptest.ResponseRecorder) *http.Cookie {
		t.Helper()
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == sessionCookieName && cookie.Value != "" {
				if !cookie.HttpOnly || !cookie.Secure {
					t.Fatalf("unsafe session cookie: %v", cookie)
				}
				return cookie
			}
		}
		t.Fatalf("no session cookie: status=%d body=%s", rec.Code, rec.Body.String())
		return nil
	}
	loginBody := `{"email":"secure-session@example.com","password":"password123"}`
	if rec := request("POST", "/auth/login", loginBody, nil, false); rec.Code != 403 {
		t.Fatalf("login CSRF status=%d", rec.Code)
	}
	login := request("POST", "/auth/login", loginBody, nil, true)
	if login.Code != 200 || strings.Contains(login.Body.String(), "token") {
		t.Fatalf("login=%d %s", login.Code, login.Body.String())
	}
	cookie := cookieFrom(login)
	var stored string
	if err := db.QueryRow(context.Background(), `select token_hash from user_sessions where user_id=$1`, userID).Scan(&stored); err != nil || stored != hashSecret(cookie.Value) {
		t.Fatalf("session not hashed: %v", err)
	}
	if rec := request("GET", "/account/me", "", cookie, true); rec.Code != 200 {
		t.Fatalf("me=%d %s", rec.Code, rec.Body.String())
	}
	secretPath := "/account/keys/" + keyID + "/secret"
	if rec := request("GET", secretPath, "", cookie, true); rec.Code != 403 || !strings.Contains(rec.Body.String(), "reauthentication_required") {
		t.Fatalf("reveal without verification=%d %s", rec.Code, rec.Body.String())
	}
	if rec := request("POST", "/auth/reauthenticate", `{"password":"incorrect-password"}`, cookie, true); rec.Code != 401 {
		t.Fatalf("wrong password=%d %s", rec.Code, rec.Body.String())
	}
	verified := request("POST", "/auth/reauthenticate", `{"password":"password123"}`, cookie, true)
	if verified.Code != 200 {
		t.Fatalf("verify=%d %s", verified.Code, verified.Body.String())
	}
	rotated := cookieFrom(verified)
	if rotated.Value == cookie.Value {
		t.Fatal("reauthentication did not rotate session")
	}
	if rec := request("GET", "/account/me", "", cookie, true); rec.Code != 401 {
		t.Fatalf("old token remains valid: %d", rec.Code)
	}
	if rec := request("GET", secretPath, "", rotated, false); rec.Code != 403 {
		t.Fatalf("cross-origin reveal=%d", rec.Code)
	}
	if rec := request("GET", secretPath, "", rotated, true); rec.Code != 200 || !strings.Contains(rec.Body.String(), "sk-xh-fake-sensitive-key") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("verified reveal=%d %s", rec.Code, rec.Body.String())
	}
	if _, err := db.Exec(context.Background(), `update user_sessions set reauthenticated_at=$1 where token_hash=$2`, time.Now().Add(-recentAuthDuration-time.Minute), hashSecret(rotated.Value)); err != nil {
		t.Fatal(err)
	}
	if rec := request("GET", secretPath, "", rotated, true); rec.Code != 403 {
		t.Fatalf("expired verification=%d", rec.Code)
	}
	if rec := request("POST", "/auth/logout", "", rotated, true); rec.Code != 204 || len(rec.Result().Cookies()) == 0 || rec.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout=%d %s", rec.Code, rec.Body.String())
	}
	if rec := request("GET", "/account/me", "", rotated, true); rec.Code != 401 {
		t.Fatalf("logged-out session=%d", rec.Code)
	}
	var oldHash string
	if err := db.QueryRow(context.Background(), `select password_hash from users where id=$1`, userID).Scan(&oldHash); err != nil {
		t.Fatal(err)
	}
	first := cookieFrom(request("POST", "/auth/login", loginBody, nil, true))
	second := cookieFrom(request("POST", "/auth/login", loginBody, nil, true))
	changed := request("PUT", "/account/password", `{"current_password":"password123","new_password":"new-password-123"}`, first, true)
	if changed.Code != 200 {
		t.Fatalf("password change=%d %s", changed.Code, changed.Body.String())
	}
	if rec := request("GET", "/account/me", "", first, true); rec.Code != 200 {
		t.Fatalf("current session lost after password change: %d", rec.Code)
	}
	if rec := request("GET", "/account/me", "", second, true); rec.Code != 401 {
		t.Fatalf("other session not revoked: %d", rec.Code)
	}
	if _, _, err := s.createSessionToken(context.Background(), userID, oldHash); err == nil {
		t.Fatal("stale verified password minted a new session")
	}
	if _, err := db.Exec(context.Background(), `update users set enabled=false where id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.createSessionToken(context.Background(), userID); err == nil {
		t.Fatal("disabled user received an OAuth session")
	}
}
