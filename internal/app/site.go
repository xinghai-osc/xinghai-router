package app

import (
	"context"
	"net/http"
	"strings"
)

// systemConfig is the effective runtime configuration for integrations
// (Geetest/Corptcha CAPTCHA and SMTP). Values set in the admin panel take
// precedence; environment variables act as fallbacks.
type systemConfig struct {
	CaptchaProvider   string // "geetest" | "corptcha" | "" (auto)
	GeetestCaptchaID  string
	GeetestCaptchaKey string
	CorptchaSiteID    string
	CorptchaSecret    string
	SMTPHost          string
	SMTPPort          string
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
}

func (c systemConfig) geetestEnabled() bool {
	return c.GeetestCaptchaID != "" && c.GeetestCaptchaKey != ""
}

func (c systemConfig) corptchaEnabled() bool {
	return c.CorptchaSiteID != "" && c.CorptchaSecret != ""
}

// captchaProvider resolves the CAPTCHA provider the gateway actually enforces.
// The admin choice wins; missing credentials degrade gracefully to the other
// configured provider, and an empty result means captcha is off.
func (c systemConfig) captchaProvider() string {
	switch c.CaptchaProvider {
	case "geetest":
		if c.geetestEnabled() {
			return "geetest"
		}
		if c.corptchaEnabled() {
			return "corptcha"
		}
		return ""
	case "corptcha":
		if c.corptchaEnabled() {
			return "corptcha"
		}
		if c.geetestEnabled() {
			return "geetest"
		}
		return ""
	}
	if c.geetestEnabled() {
		return "geetest"
	}
	if c.corptchaEnabled() {
		return "corptcha"
	}
	return ""
}

func (c systemConfig) emailVerificationEnabled() bool {
	return c.SMTPHost != "" && c.SMTPFrom != ""
}

func (s *Service) loadSystemConfig(ctx context.Context) systemConfig {
	cfg := systemConfig{CaptchaProvider: s.cfg.CaptchaProvider, GeetestCaptchaID: s.cfg.GeetestCaptchaID, GeetestCaptchaKey: s.cfg.GeetestCaptchaKey, CorptchaSiteID: s.cfg.CorptchaSiteID, CorptchaSecret: s.cfg.CorptchaSecret, SMTPHost: s.cfg.SMTPHost, SMTPPort: s.cfg.SMTPPort, SMTPUsername: s.cfg.SMTPUsername, SMTPPassword: s.cfg.SMTPPassword, SMTPFrom: s.cfg.SMTPFrom}
	if s.db == nil {
		return cfg
	}
	var captchaProvider, geetestID, geetestKeyEnc, corptchaSiteID, corptchaSecretEnc, smtpHost, smtpPort, smtpUser, smtpPassEnc, smtpFrom string
	err := s.db.QueryRow(ctx, `select captcha_provider,geetest_captcha_id,geetest_captcha_key_encrypted,corptcha_site_id,corptcha_secret_encrypted,smtp_host,smtp_port,smtp_username,smtp_password_encrypted,smtp_from from site_settings where id=true`).Scan(&captchaProvider, &geetestID, &geetestKeyEnc, &corptchaSiteID, &corptchaSecretEnc, &smtpHost, &smtpPort, &smtpUser, &smtpPassEnc, &smtpFrom)
	if err != nil {
		return cfg
	}
	if v := strings.TrimSpace(captchaProvider); v != "" {
		cfg.CaptchaProvider = v
	}
	if v := strings.TrimSpace(geetestID); v != "" {
		cfg.GeetestCaptchaID = v
	}
	if v := strings.TrimSpace(geetestKeyEnc); v != "" {
		if plain, err := crypt(s.cfg.EncryptionKey, v, true); err == nil {
			cfg.GeetestCaptchaKey = plain
		}
	}
	if v := strings.TrimSpace(corptchaSiteID); v != "" {
		cfg.CorptchaSiteID = v
	}
	if v := strings.TrimSpace(corptchaSecretEnc); v != "" {
		if plain, err := crypt(s.cfg.EncryptionKey, v, true); err == nil {
			cfg.CorptchaSecret = plain
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
	var name, iconURL, announcement string
	var autoDisableFailedChannels bool
	if err := s.db.QueryRow(r.Context(), `select name,icon_url,announcement,auto_disable_failed_channels from site_settings where id=true`).Scan(&name, &iconURL, &announcement, &autoDisableFailedChannels); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	sys := s.loadSystemConfig(r.Context())
	var oauthProviders []string
	rows, err := s.db.Query(r.Context(), `select id from oauth_providers where enabled`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				oauthProviders = append(oauthProviders, id)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "announcement": announcement, "auto_disable_failed_channels": autoDisableFailedChannels, "captcha_provider": sys.captchaProvider(), "geetest_enabled": sys.geetestEnabled(), "geetest_captcha_id": sys.GeetestCaptchaID, "corptcha_site_id": sys.CorptchaSiteID, "email_verification_enabled": sys.emailVerificationEnabled(), "oauth_providers": oauthProviders})
}

func (s *Service) adminSiteSettings(w http.ResponseWriter, r *http.Request) {
	var name, iconURL, announcement string
	var autoDisableFailedChannels bool
	var captchaProvider, geetestID, geetestKeyEnc, corptchaSiteID, corptchaSecretEnc, smtpHost, smtpPort, smtpUser, smtpPassEnc, smtpFrom, publicBaseURL string
	err := s.db.QueryRow(r.Context(), `select name,icon_url,announcement,auto_disable_failed_channels,captcha_provider,geetest_captcha_id,geetest_captcha_key_encrypted,corptcha_site_id,corptcha_secret_encrypted,smtp_host,smtp_port,smtp_username,smtp_password_encrypted,smtp_from,public_base_url from site_settings where id=true`).Scan(&name, &iconURL, &announcement, &autoDisableFailedChannels, &captchaProvider, &geetestID, &geetestKeyEnc, &corptchaSiteID, &corptchaSecretEnc, &smtpHost, &smtpPort, &smtpUser, &smtpPassEnc, &smtpFrom, &publicBaseURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "announcement": announcement, "auto_disable_failed_channels": autoDisableFailedChannels, "captcha_provider": captchaProvider, "geetest_captcha_id": geetestID, "has_geetest_captcha_key": strings.TrimSpace(geetestKeyEnc) != "", "corptcha_site_id": corptchaSiteID, "has_corptcha_secret": strings.TrimSpace(corptchaSecretEnc) != "", "smtp_host": smtpHost, "smtp_port": smtpPort, "smtp_username": smtpUser, "has_smtp_password": strings.TrimSpace(smtpPassEnc) != "", "smtp_from": smtpFrom, "public_base_url": publicBaseURL})
}

func (s *Service) updateSiteSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name                      string  `json:"name"`
		IconURL                   string  `json:"icon_url"`
		Announcement              string  `json:"announcement"`
		AutoDisableFailedChannels *bool   `json:"auto_disable_failed_channels"`
		CaptchaProvider           *string `json:"captcha_provider"`
		GeetestCaptchaID          *string `json:"geetest_captcha_id"`
		GeetestCaptchaKey         string  `json:"geetest_captcha_key"`
		CorptchaSiteID            *string `json:"corptcha_site_id"`
		CorptchaSecret            string  `json:"corptcha_secret"`
		SMTPHost                  *string `json:"smtp_host"`
		SMTPPort                  *string `json:"smtp_port"`
		SMTPUsername              *string `json:"smtp_username"`
		SMTPPassword              string  `json:"smtp_password"`
		SMTPFrom                  *string `json:"smtp_from"`
		PublicBaseURL             *string `json:"public_base_url"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid site settings")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.IconURL = strings.TrimSpace(in.IconURL)
	in.Announcement = strings.TrimSpace(in.Announcement)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_request", "site name must contain 1 to 100 characters")
		return
	}
	if len(in.IconURL) > maxSiteIconURLLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon_url must be at most 2048 characters")
		return
	}
	if len([]rune(in.Announcement)) > maxAnnouncementLen {
		writeError(w, http.StatusBadRequest, "invalid_request", "announcement must be at most 2000 characters")
		return
	}
	if in.IconURL != "" && !validIconURL(in.IconURL) {
		writeError(w, http.StatusBadRequest, "invalid_request", "icon_url must use HTTPS, except for loopback HTTP URLs")
		return
	}
	if in.CaptchaProvider != nil {
		provider := strings.ToLower(strings.TrimSpace(*in.CaptchaProvider))
		if provider != "" && provider != "geetest" && provider != "corptcha" {
			writeError(w, http.StatusBadRequest, "invalid_request", "captcha_provider must be geetest, corptcha or empty")
			return
		}
		*in.CaptchaProvider = provider
	}
	if in.GeetestCaptchaID != nil {
		id := strings.TrimSpace(*in.GeetestCaptchaID)
		if len(id) > maxGeetestFieldLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "geetest_captcha_id must be at most 256 characters")
			return
		}
		*in.GeetestCaptchaID = id
	}
	if in.CorptchaSiteID != nil {
		id := strings.TrimSpace(*in.CorptchaSiteID)
		if len(id) > maxCorptchaFieldLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "corptcha_site_id must be at most 256 characters")
			return
		}
		*in.CorptchaSiteID = id
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
	if in.PublicBaseURL != nil {
		base := strings.TrimSpace(*in.PublicBaseURL)
		if len(base) > maxPublicBaseURLLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "public_base_url must be at most 2048 characters")
			return
		}
		if base != "" && validUpstreamURL(base) != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "public_base_url must be an HTTP or HTTPS URL")
			return
		}
		*in.PublicBaseURL = base
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
	corptchaSecretEnc := ""
	if secret := strings.TrimSpace(in.CorptchaSecret); secret != "" {
		if len(secret) > maxCorptchaFieldLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "corptcha_secret must be at most 256 characters")
			return
		}
		encrypted, err := crypt(s.cfg.EncryptionKey, secret, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not encrypt the corptcha secret")
			return
		}
		corptchaSecretEnc = encrypted
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
	if _, err := s.db.Exec(r.Context(), `update site_settings set name=$1,icon_url=$2,announcement=$3,
		auto_disable_failed_channels=coalesce($4,auto_disable_failed_channels),
		captcha_provider=coalesce($5,captcha_provider),
		geetest_captcha_id=coalesce($6,geetest_captcha_id),
		geetest_captcha_key_encrypted=case when $7='' then geetest_captcha_key_encrypted else $7 end,
		corptcha_site_id=coalesce($8,corptcha_site_id),
		corptcha_secret_encrypted=case when $9='' then corptcha_secret_encrypted else $9 end,
		smtp_host=coalesce($10,smtp_host),
		smtp_port=coalesce($11,smtp_port),
		smtp_username=coalesce($12,smtp_username),
		smtp_password_encrypted=case when $13='' then smtp_password_encrypted else $13 end,
		smtp_from=coalesce($14,smtp_from),
		public_base_url=coalesce($15,public_base_url),
		updated_at=now() where id=true`,
		in.Name, in.IconURL, in.Announcement, in.AutoDisableFailedChannels,
		trimmedPtr(in.CaptchaProvider), trimmedPtr(in.GeetestCaptchaID), geetestKeyEnc,
		trimmedPtr(in.CorptchaSiteID), corptchaSecretEnc,
		trimmedPtr(in.SMTPHost), trimmedPtr(in.SMTPPort), trimmedPtr(in.SMTPUsername), smtpPassEnc, trimmedPtr(in.SMTPFrom), trimmedPtr(in.PublicBaseURL)); err != nil {
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
	maxSiteIconURLLen   = 2048
	maxAnnouncementLen  = 2000
	maxGeetestFieldLen  = 256
	maxCorptchaFieldLen = 256
	maxSMTPHostLen      = 255
	maxSMTPUsernameLen  = 255
	maxSMTPPasswordLen  = 4096
	maxPublicBaseURLLen = 2048
)

// loadPublicBaseURL returns the admin-configured public origin used for
// password-reset email links and OAuth callback URIs. An empty result means
// links are derived from the request host.
func (s *Service) loadPublicBaseURL(ctx context.Context) string {
	if s.db != nil {
		var v string
		if err := s.db.QueryRow(ctx, `select public_base_url from site_settings where id=true`).Scan(&v); err == nil {
			if v = strings.TrimSpace(v); v != "" {
				return strings.TrimRight(v, "/")
			}
		}
	}
	return ""
}

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
