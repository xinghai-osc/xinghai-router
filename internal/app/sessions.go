package app

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionCookieName = "xinghai.session"
const recentAuthDuration = 5 * time.Minute

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (s *Service) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	maxAge := max(1, int(time.Until(expiresAt).Seconds()))
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", MaxAge: maxAge, Expires: expiresAt, HttpOnly: true, Secure: s.cfg.SessionCookieSecure, SameSite: http.SameSiteLaxMode})
	s.clearLegacySessionCookie(w)
	w.Header().Set("Cache-Control", "no-store")
}

func (s *Service) clearLegacySessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "xinghai.admin-token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SessionCookieSecure, SameSite: http.SameSiteLaxMode})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SessionCookieSecure, SameSite: http.SameSiteLaxMode})
	s.clearLegacySessionCookie(w)
	w.Header().Set("Cache-Control", "no-store")
}

func consoleRequestAllowed(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
			return false
		}
	}
	return r.Header.Get("X-Xinghai-Request") == "1"
}

func (s *Service) consoleSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		console := strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/account/") || strings.HasPrefix(path, "/admin/")
		if console {
			w.Header().Set("Cache-Control", "no-store")
			unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
			if unsafe && !consoleRequestAllowed(r) {
				writeError(w, http.StatusForbidden, "csrf_failed", "same-origin console request required")
				return
			}
			if unsafe {
				body, err := readRequestBody(w, r, 8<<20, s.cfg.RequestBodyTimeout)
				if err != nil {
					status, code, message := requestBodyError(err)
					writeError(w, status, code, message)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) requireRecentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !consoleRequestAllowed(r) {
			writeError(w, http.StatusForbidden, "csrf_failed", "same-origin console request required")
			return
		}
		account := accountFromContext(r)
		if account.reauthenticatedAt == nil || time.Since(*account.reauthenticatedAt) < 0 || time.Since(*account.reauthenticatedAt) >= recentAuthDuration {
			writeError(w, http.StatusForbidden, "reauthentication_required", "verify your password again before accessing sensitive data")
			return
		}
		next(w, r)
	}
}

func (s *Service) reauthenticate(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	if s.limiter == nil || !s.limiter.allowN("auth:reauthenticate:user:"+account.userID, 5) {
		writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many verification attempts")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	body, err := readRequestBody(w, r, 4096, s.cfg.RequestBodyTimeout)
	if err != nil {
		status, code, message := requestBodyError(err)
		writeError(w, status, code, message)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if decode(r, &in) != nil || !validPasswordLength(in.Password) {
		writeError(w, http.StatusBadRequest, "invalid_request", "a password between 8 and 72 characters is required")
		return
	}
	var passwordHash string
	err = s.db.QueryRow(r.Context(), `select coalesce(password_hash,'') from users where id=$1 and enabled`, account.userID).Scan(&passwordHash)
	if err != nil || passwordHash == "" || !passwordMatches(passwordHash, in.Password) {
		if passwordHash == "" {
			_ = passwordMatches(dummyPasswordHash, in.Password)
		}
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "password verification failed; passwordless accounts must configure a password first")
		return
	}
	token, err := randomSecret("xh_session_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not rotate session")
		return
	}
	var expiresAt, verifiedAt time.Time
	err = s.db.QueryRow(r.Context(), `update user_sessions set token_hash=$1,reauthenticated_at=now() where token_hash=$2 and user_id=$3 and expires_at>now() and exists(select 1 from users where id=$3 and enabled and password_hash=$4) returning expires_at,reauthenticated_at`, hashSecret(token), hashSecret(sessionToken(r)), account.userID, passwordHash).Scan(&expiresAt, &verifiedAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "session or password changed; sign in again")
		return
	}
	s.setSessionCookie(w, token, expiresAt)
	s.audit(r, "account.reauthenticated", "user", account.userID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"expires_at": verifiedAt.Add(recentAuthDuration)})
}
