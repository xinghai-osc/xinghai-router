package app

import (
	"context"
	"net/http"
	"strings"
)

// systemConfig is the effective runtime configuration for integrations
// (Geetest CAPTCHA and SMTP). Values set in the admin panel take precedence;
// environment variables act as fallbacks.
type systemConfig struct {
	GeetestCaptchaID  string
	GeetestCaptchaKey string
	SMTPHost          string
	SMTPPort          string
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
}

func (c systemConfig) geetestEnabled() bool {
	return c.GeetestCaptchaID != "" && c.GeetestCaptchaKey != ""
}

func (c systemConfig) emailVerificationEnabled() bool {
	return c.SMTPHost != "" && c.SMTPFrom != ""
}

func (s *Service) loadSystemConfig(ctx context.Context) systemConfig {
	cfg := systemConfig{GeetestCaptchaID: s.cfg.GeetestCaptchaID, GeetestCaptchaKey: s.cfg.GeetestCaptchaKey, SMTPHost: s.cfg.SMTPHost, SMTPPort: s.cfg.SMTPPort, SMTPUsername: s.cfg.SMTPUsername, SMTPPassword: s.cfg.SMTPPassword, SMTPFrom: s.cfg.SMTPFrom}
	if s.db == nil {
		return cfg
	}
	var geetestID, geetestKeyEnc, smtpHost, smtpPort, smtpUser, smtpPassEnc, smtpFrom string
	err := s.db.QueryRow(ctx, `select geetest_captcha_id,geetest_captcha_key_encrypted,smtp_host,smtp_port,smtp_username,smtp_password_encrypted,smtp_from from site_settings where id=true`).Scan(&geetestID, &geetestKeyEnc, &smtpHost, &smtpPort, &smtpUser, &smtpPassEnc, &smtpFrom)
	if err != nil {
		return cfg
	}
	if v := strings.TrimSpace(geetestID); v != "" {
		cfg.GeetestCaptchaID = v
	}
	if v := strings.TrimSpace(geetestKeyEnc); v != "" {
		if plain, err := crypt(s.cfg.EncryptionKey, v, true); err == nil {
			cfg.GeetestCaptchaKey = plain
		}
	}
	if v := strings.TrimSpace(smtpHost); v != "" {
		cfg.SMTPHost = v
	}
	if v := strings.TrimSpace(smtpPort); v != "" {
		cfg.SMTPPort = v
	}
	if v := strings.TrimSpace(smtpUser); v != "" {
		cfg.SMTPUsername = v
	}
	if v := strings.TrimSpace(smtpPassEnc); v != "" {
		if plain, err := crypt(s.cfg.EncryptionKey, v, true); err == nil {
			cfg.SMTPPassword = plain
		}
	}
	if v := strings.TrimSpace(smtpFrom); v != "" {
		cfg.SMTPFrom = v
	}
	return cfg
}

func (s *Service) siteSettings(w http.ResponseWriter, r *http.Request) {
	var name, iconURL string
	var autoDisableFailedChannels bool
	if err := s.db.QueryRow(r.Context(), `select name,icon_url,auto_disable_failed_channels from site_settings where id=true`).Scan(&name, &iconURL, &autoDisableFailedChannels); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	sys := s.loadSystemConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "auto_disable_failed_channels": autoDisableFailedChannels, "geetest_enabled": sys.geetestEnabled(), "geetest_captcha_id": sys.GeetestCaptchaID, "email_verification_enabled": sys.emailVerificationEnabled()})
}

func (s *Service) adminSiteSettings(w http.ResponseWriter, r *http.Request) {
	var name, iconURL string
	var autoDisableFailedChannels bool
	var geetestID, geetestKeyEnc, smtpHost, smtpPort, smtpUser, smtpPassEnc, smtpFrom string
	err := s.db.QueryRow(r.Context(), `select name,icon_url,auto_disable_failed_channels,geetest_captcha_id,geetest_captcha_key_encrypted,smtp_host,smtp_port,smtp_username,smtp_password_encrypted,smtp_from from site_settings where id=true`).Scan(&name, &iconURL, &autoDisableFailedChannels, &geetestID, &geetestKeyEnc, &smtpHost, &smtpPort, &smtpUser, &smtpPassEnc, &smtpFrom)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "auto_disable_failed_channels": autoDisableFailedChannels, "geetest_captcha_id": geetestID, "has_geetest_captcha_key": strings.TrimSpace(geetestKeyEnc) != "", "smtp_host": smtpHost, "smtp_port": smtpPort, "smtp_username": smtpUser, "has_smtp_password": strings.TrimSpace(smtpPassEnc) != "", "smtp_from": smtpFrom})
}

func (s *Service) updateSiteSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name                      string  `json:"name"`
		IconURL                   string  `json:"icon_url"`
		AutoDisableFailedChannels *bool   `json:"auto_disable_failed_channels"`
		GeetestCaptchaID          *string `json:"geetest_captcha_id"`
		GeetestCaptchaKey         string  `json:"geetest_captcha_key"`
		SMTPHost                  *string `json:"smtp_host"`
		SMTPPort                  *string `json:"smtp_port"`
		SMTPUsername              *string `json:"smtp_username"`
		SMTPPassword              string  `json:"smtp_password"`
		SMTPFrom                  *string `json:"smtp_from"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid site settings")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.IconURL = strings.TrimSpace(in.IconURL)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_request", "site name must contain 1 to 100 characters")
		return
	}
	if len(in.IconURL) > maxSiteIconURLLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon_url must be at most 2048 characters")
		return
	}
	if in.IconURL != "" && !validIconURL(in.IconURL) {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon_url must use HTTPS, except for loopback HTTP URLs")
		return
	}
	if in.GeetestCaptchaID != nil {
		id := strings.TrimSpace(*in.GeetestCaptchaID)
		if len(id) > maxGeetestFieldLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "geetest_captcha_id must be at most 256 characters")
			return
		}
		*in.GeetestCaptchaID = id
	}
	if in.SMTPHost != nil {
		host := strings.TrimSpace(*in.SMTPHost)
		if len(host) > maxSMTPHostLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "smtp_host must be at most 255 characters")
			return
		}
		*in.SMTPHost = host
	}
	if in.SMTPUsername != nil {
		user := strings.TrimSpace(*in.SMTPUsername)
		if len(user) > maxSMTPUsernameLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "smtp_username must be at most 255 characters")
			return
		}
		*in.SMTPUsername = user
	}
	if in.SMTPPort != nil {
		if port := strings.TrimSpace(*in.SMTPPort); port != "" && !validSMTPPort(port) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid smtp port")
			return
		}
	}
	if in.SMTPFrom != nil {
		if from := strings.TrimSpace(*in.SMTPFrom); from != "" && !validEmail(from) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid smtp sender address")
			return
		}
	}
	geetestKeyEnc, smtpPassEnc := "", ""
	if key := strings.TrimSpace(in.GeetestCaptchaKey); key != "" {
		if len(key) > maxGeetestFieldLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "geetest_captcha_key must be at most 256 characters")
			return
		}
		encrypted, err := crypt(s.cfg.EncryptionKey, key, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not encrypt the captcha key")
			return
		}
		geetestKeyEnc = encrypted
	}
	if password := strings.TrimSpace(in.SMTPPassword); password != "" {
		if len(password) > maxSMTPPasswordLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "smtp_password must be at most 4096 characters")
			return
		}
		encrypted, err := crypt(s.cfg.EncryptionKey, password, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not encrypt the smtp password")
			return
		}
		smtpPassEnc = encrypted
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set name=$1,icon_url=$2,
		auto_disable_failed_channels=coalesce($3,auto_disable_failed_channels),
		geetest_captcha_id=coalesce($4,geetest_captcha_id),
		geetest_captcha_key_encrypted=case when $5='' then geetest_captcha_key_encrypted else $5 end,
		smtp_host=coalesce($6,smtp_host),
		smtp_port=coalesce($7,smtp_port),
		smtp_username=coalesce($8,smtp_username),
		smtp_password_encrypted=case when $9='' then smtp_password_encrypted else $9 end,
		smtp_from=coalesce($10,smtp_from),
		updated_at=now() where id=true`,
		in.Name, in.IconURL, in.AutoDisableFailedChannels,
		trimmedPtr(in.GeetestCaptchaID), geetestKeyEnc,
		trimmedPtr(in.SMTPHost), trimmedPtr(in.SMTPPort), trimmedPtr(in.SMTPUsername), smtpPassEnc, trimmedPtr(in.SMTPFrom)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save site settings")
		return
	}
	s.audit(r, "settings.updated", "site_settings", "site", map[string]any{"name": in.Name})
	s.adminSiteSettings(w, r)
}

func trimmedPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

const (
	maxSiteIconURLLen  = 2048
	maxGeetestFieldLen = 256
	maxSMTPHostLen     = 255
	maxSMTPUsernameLen = 255
	maxSMTPPasswordLen = 4096
)

func validSMTPPort(port string) bool {
	if len(port) == 0 || len(port) > 5 {
		return false
	}
	n := 0
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 65535
}

func validIconURL(value string) bool {
	return validOutboundURL(value) == nil
}
