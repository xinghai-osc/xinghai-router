package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

const (
	emailCodeTTL          = 10 * time.Minute
	emailCodeMaxAttempts  = 5
	emailCodeSendCooldown = 60 * time.Second
)

// sendEmail delivers an HTML message through the configured SMTP server.
// Port 465 uses implicit TLS; other ports upgrade via STARTTLS when offered.
func (s *Service) sendEmail(ctx context.Context, to, subject, html string) error {
	cfg := s.loadSystemConfig(ctx)
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort
	from := cfg.SMTPFrom
	message := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: =?UTF-8?B?" + base64Encode(subject) + "?=",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		html,
	}, "\r\n")

	var client *smtp.Client
	if cfg.SMTPPort == "465" {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.SMTPHost})
		if err != nil {
			return fmt.Errorf("smtp connect failed: %w", err)
		}
		client, err = smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp handshake failed: %w", err)
		}
	} else {
		var err error
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp connect failed: %w", err)
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
				return fmt.Errorf("smtp starttls failed: %w", err)
			}
		}
	}
	defer client.Close()
	if cfg.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt failed: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp finalize failed: %w", err)
	}
	return client.Quit()
}

func base64Encode(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

func generateEmailCode() (string, error) {
	buffer := make([]byte, 3)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	n := int(buffer[0])<<16 | int(buffer[1])<<8 | int(buffer[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func hashEmailCode(email, code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email)) + ":" + code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) sendEmailCode(w http.ResponseWriter, r *http.Request) {
	if !s.loadSystemConfig(r.Context()).emailVerificationEnabled() {
		writeError(w, http.StatusNotFound, "not_found", "email verification is not enabled")
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
		if !s.limiter.allow("auth:email-code:ip:"+clientIP) || !s.limiter.allow("auth:email-code:email:"+email) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many verification code requests")
			return
		}
	}
	if err := s.verifyGeetest(r.Context(), in.geetestPayload); err != nil {
		writeError(w, http.StatusForbidden, "captcha_failed", err.Error())
		return
	}
	ctx := r.Context()
	var exists bool
	if err := s.db.QueryRow(ctx, `select exists(select 1 from users where email=$1)`, email).Scan(&exists); err == nil && exists {
		writeError(w, http.StatusConflict, "email_registered", "this email is already registered")
		return
	}
	var lastSent time.Time
	if err := s.db.QueryRow(ctx, `select coalesce(max(created_at), 'epoch') from email_verification_codes where email=$1 and purpose='register'`, email).Scan(&lastSent); err == nil && time.Since(lastSent) < emailCodeSendCooldown {
		writeError(w, http.StatusTooManyRequests, "send_too_fast", "please wait before requesting another code")
		return
	}
	code, err := generateEmailCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not generate code")
		return
	}
	siteName := s.siteName(ctx)
	subject := fmt.Sprintf("%s 注册验证码 / Registration code", siteName)
	body := fmt.Sprintf(`<div style="max-width:480px;margin:0 auto;padding:32px;font-family:-apple-system,'Segoe UI',sans-serif;color:#1a1a2e">
	<h2 style="margin:0 0 8px;font-size:20px">%s</h2>
	<p style="margin:0 0 24px;color:#666;font-size:14px">您的注册验证码 / Your registration code</p>
	<div style="padding:20px 24px;border-radius:12px;background:#f4f5ff;text-align:center">
		<span style="font-size:32px;font-weight:700;letter-spacing:8px;font-family:ui-monospace,monospace">%s</span>
	</div>
	<p style="margin:24px 0 0;color:#999;font-size:12px">验证码 10 分钟内有效。若非本人操作请忽略本邮件。<br/>This code expires in 10 minutes. Ignore this email if you did not request it.</p>
</div>`, siteName, code)
	if err := s.sendEmail(ctx, email, subject, body); err != nil {
		writeError(w, http.StatusBadGateway, "email_send_failed", "could not send the verification email")
		return
	}
	if _, err := s.db.Exec(ctx, `insert into email_verification_codes(email, code_hash, expires_at) values($1, $2, $3)`, email, hashEmailCode(email, code), time.Now().Add(emailCodeTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not store the code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// verifyEmailCode validates a registration code and consumes it on success.
func (s *Service) verifyEmailCode(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	var id, codeHash string
	var attempts int
	var expiresAt time.Time
	err := s.db.QueryRow(ctx, `select id::text, code_hash, attempts, expires_at from email_verification_codes where email=$1 and purpose='register' and consumed_at is null order by created_at desc limit 1`, email).Scan(&id, &codeHash, &attempts, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		return fmt.Errorf("the verification code is invalid or expired")
	}
	if attempts >= emailCodeMaxAttempts {
		return fmt.Errorf("too many incorrect attempts, request a new code")
	}
	if hashEmailCode(email, code) != codeHash {
		_, _ = s.db.Exec(ctx, `update email_verification_codes set attempts=attempts+1 where id=$1`, id)
		return fmt.Errorf("the verification code is invalid or expired")
	}
	if _, err := s.db.Exec(ctx, `update email_verification_codes set consumed_at=now() where id=$1`, id); err != nil {
		return fmt.Errorf("could not confirm the code")
	}
	return nil
}

// siteName resolves the public site name for email templates.
func (s *Service) siteName(ctx context.Context) string {
	var name string
	if err := s.db.QueryRow(ctx, `select name from site_settings limit 1`).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
		return "Xinghai Router"
	}
	return name
}
