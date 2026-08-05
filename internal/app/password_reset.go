package app

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const passwordResetTTL = 30 * time.Minute

// resetLink builds the console URL that carries the reset token. The Go service
// is reached through the Nuxt proxy, which preserves the public Host header, so
// the link points at the console the user actually came from.
func resetLink(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/auth/reset?token=" + url.QueryEscape(token)
}

func (s *Service) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if !s.loadSystemConfig(r.Context()).emailVerificationEnabled() {
		writeError(w, http.StatusNotFound, "not_found", "password reset is not enabled")
		return
	}
	var in struct {
		Email string `json:"email"`
		geetestPayload
	}
	if decode(r, &in) != nil || !validEmail(in.Email) {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	clientIP := requestMetadata(r).clientIP
	if s.limiter != nil {
		if !s.limiter.allowN("auth:password-reset:ip:"+clientIP, authPasswordResetPerMinute) || !s.limiter.allowN("auth:password-reset:email:"+email, authPasswordResetPerMinute) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many password reset requests")
			return
		}
	}
	if err := s.verifyGeetest(r.Context(), in.geetestPayload); err != nil {
		writeError(w, http.StatusForbidden, "captcha_failed", err.Error())
		return
	}
	ctx := r.Context()
	var userID string
	if err := s.db.QueryRow(ctx, `select id from users where email=$1 and enabled`, email).Scan(&userID); err != nil {
		// Always return the same success shape so callers cannot probe whether
		// an address is registered. Nothing is sent for unknown accounts.
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}
	token, err := randomSecret("xh_reset_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create reset token")
		return
	}
	if _, err = s.db.Exec(ctx, `update password_reset_tokens set consumed_at=now() where email=$1 and consumed_at is null`, email); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create reset token")
		return
	}
	siteName := s.siteName(ctx)
	subject := fmt.Sprintf("%s 密码重置 / Password reset", siteName)
	link := html.EscapeString(resetLink(r, token))
	body := fmt.Sprintf(`<div style="max-width:480px;margin:0 auto;padding:32px;font-family:-apple-system,'Segoe UI',sans-serif;color:#1a1a2e">
	<h2 style="margin:0 0 8px;font-size:20px">%s</h2>
	<p style="margin:0 0 24px;color:#666;font-size:14px">我们收到了重置密码的请求 / We received a request to reset your password</p>
	<p style="margin:0 0 24px;color:#666;font-size:14px">点击下方按钮设置新密码 / Click the button below to set a new password:</p>
	<a href="%s" style="display:inline-block;padding:12px 24px;border-radius:8px;background:#4f46e5;color:#fff;text-decoration:none;font-size:14px">重置密码 / Reset password</a>
	<p style="margin:24px 0 0;color:#999;font-size:12px">链接 30 分钟内有效。若非本人操作请忽略本邮件。<br/>This link expires in 30 minutes. Ignore this email if you did not request it.</p>
</div>`, siteName, link)
	if err := s.sendEmail(ctx, email, subject, body); err != nil {
		writeError(w, http.StatusBadGateway, "email_send_failed", "could not send the reset email")
		return
	}
	if _, err = s.db.Exec(ctx, `insert into password_reset_tokens(email, token_hash, expires_at) values($1, $2, $3)`, email, hashSecret(token), time.Now().Add(passwordResetTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not store the reset token")
		return
	}
	s.auditActor(r, userID, "account.password_reset_requested", "user", userID, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Service) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "token and password are required")
		return
	}
	if !validPasswordLength(in.Password) {
		writeError(w, http.StatusBadRequest, "invalid_request", "password must be between 8 and 72 characters")
		return
	}
	tokenHash := hashSecret(strings.TrimSpace(in.Token))
	clientIP := requestMetadata(r).clientIP
	if s.limiter != nil {
		if !s.limiter.allowN("auth:password-reset-confirm:ip:"+clientIP, authPasswordResetConfirmPerMinute) || !s.limiter.allowN("auth:password-reset-confirm:token:"+tokenHash, authPasswordResetConfirmPerMinute) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many password reset attempts")
			return
		}
	}
	ctx := r.Context()
	var email string
	var expiresAt time.Time
	if err := s.db.QueryRow(ctx, `select email, expires_at from password_reset_tokens where token_hash=$1 and consumed_at is null order by created_at desc limit 1`, tokenHash).Scan(&email, &expiresAt); err != nil || time.Now().After(expiresAt) {
		writeError(w, http.StatusBadRequest, "invalid_token", "this reset link is invalid or has expired")
		return
	}
	var userID string
	if err := s.db.QueryRow(ctx, `select id from users where email=$1 and enabled and password_hash is not null`, email).Scan(&userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_token", "this reset link is invalid or has expired")
		return
	}
	newHash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not secure password")
		return
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset password")
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `update users set password_hash=$1, must_change_password=false where id=$2`, newHash, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset password")
		return
	}
	if _, err = tx.Exec(ctx, `delete from user_sessions where user_id=$1`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not revoke sessions")
		return
	}
	if _, err = tx.Exec(ctx, `update password_reset_tokens set consumed_at=now() where token_hash=$1`, tokenHash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not consume reset token")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset password")
		return
	}
	s.auditActor(r, userID, "account.password_reset", "user", userID, map[string]any{"sessions_revoked": true})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
