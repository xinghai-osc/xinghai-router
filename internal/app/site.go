package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
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

func (s *Service) registrationEmailAllowed(ctx context.Context, email string) (bool, error) {
	if s.db == nil {
		return true, nil
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var whitelistEnabled, aliasBlocked bool
	var whitelist []string
	if err := s.db.QueryRow(ctx, `select registration_email_whitelist_enabled,registration_email_whitelist,registration_email_alias_blocked from site_settings where id=true`).Scan(&whitelistEnabled, &whitelist, &aliasBlocked); err != nil {
		return false, err
	}
	if aliasBlocked && isEmailAlias(email) {
		return false, nil
	}
	if !whitelistEnabled {
		return true, nil
	}
	return emailWhitelistAllowed(whitelist, email), nil
}

func emailWhitelistAllowed(whitelist []string, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	_, domain, ok := strings.Cut(email, "@")
	if !ok || domain == "" {
		return false
	}
	for _, raw := range whitelist {
		allowed := strings.ToLower(strings.TrimSpace(raw))
		if allowed == "" {
			continue
		}
		if strings.HasPrefix(allowed, "@") {
			if domain == strings.TrimPrefix(allowed, "@") {
				return true
			}
			continue
		}
		if email == allowed {
			return true
		}
	}
	return false
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

type featuredCopy map[string]map[string]string

func (s *Service) siteSettings(w http.ResponseWriter, r *http.Request) {
	var name, iconURL, announcement, contactEmail, featuredModel string
	var autoDisableFailedChannels, invitationsEnabled, whitelistEnabled, aliasBlocked, featuredEnabled bool
	var featuredCopyJSON []byte
	if err := s.db.QueryRow(r.Context(), `select name,icon_url,announcement,contact_email,auto_disable_failed_channels,invitations_enabled,registration_email_whitelist_enabled,registration_email_alias_blocked,featured_enabled,featured_model,featured_copy from site_settings where id=true`).Scan(&name, &iconURL, &announcement, &contactEmail, &autoDisableFailedChannels, &invitationsEnabled, &whitelistEnabled, &aliasBlocked, &featuredEnabled, &featuredModel, &featuredCopyJSON); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	copy, err := decodeFeaturedCopy(featuredCopyJSON)
	if err != nil {
		copy = defaultFeaturedCopy()
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
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "announcement": announcement, "contact_email": contactEmail, "auto_disable_failed_channels": autoDisableFailedChannels, "invitations_enabled": invitationsEnabled, "registration_email_whitelist_enabled": whitelistEnabled, "registration_email_alias_blocked": aliasBlocked, "featured_enabled": featuredEnabled, "featured_model": featuredModel, "featured_copy": copy, "captcha_provider": sys.captchaProvider(), "geetest_enabled": sys.geetestEnabled(), "geetest_captcha_id": sys.GeetestCaptchaID, "corptcha_site_id": sys.CorptchaSiteID, "email_verification_enabled": sys.emailVerificationEnabled(), "oauth_providers": oauthProviders})
}

func (s *Service) adminSiteSettings(w http.ResponseWriter, r *http.Request) {
	var name, iconURL, announcement, contactEmail, featuredModel string
	var autoDisableFailedChannels, invitationsEnabled, whitelistEnabled, aliasBlocked, featuredEnabled bool
	var whitelist []string
	var featuredCopyJSON []byte
	var inviterReward, inviteeReward, checkinBaseReward, checkinStreakBonus string
	var checkinMaxBonusDays int
	var captchaProvider, geetestID, geetestKeyEnc, corptchaSiteID, corptchaSecretEnc, smtpHost, smtpPort, smtpUser, smtpPassEnc, smtpFrom, publicBaseURL string
	err := s.db.QueryRow(r.Context(), `select name,icon_url,announcement,contact_email,auto_disable_failed_channels,registration_email_whitelist_enabled,registration_email_whitelist,registration_email_alias_blocked,captcha_provider,geetest_captcha_id,geetest_captcha_key_encrypted,corptcha_site_id,corptcha_secret_encrypted,smtp_host,smtp_port,smtp_username,smtp_password_encrypted,smtp_from,public_base_url,invitations_enabled,inviter_reward::text,invitee_reward::text,checkin_base_reward::text,checkin_streak_bonus::text,checkin_max_bonus_days,featured_enabled,featured_model,featured_copy from site_settings where id=true`).Scan(&name, &iconURL, &announcement, &contactEmail, &autoDisableFailedChannels, &whitelistEnabled, &whitelist, &aliasBlocked, &captchaProvider, &geetestID, &geetestKeyEnc, &corptchaSiteID, &corptchaSecretEnc, &smtpHost, &smtpPort, &smtpUser, &smtpPassEnc, &smtpFrom, &publicBaseURL, &invitationsEnabled, &inviterReward, &inviteeReward, &checkinBaseReward, &checkinStreakBonus, &checkinMaxBonusDays, &featuredEnabled, &featuredModel, &featuredCopyJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load site settings")
		return
	}
	copy, err := decodeFeaturedCopy(featuredCopyJSON)
	if err != nil {
		copy = defaultFeaturedCopy()
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "icon_url": iconURL, "announcement": announcement, "contact_email": contactEmail, "auto_disable_failed_channels": autoDisableFailedChannels, "featured_enabled": featuredEnabled, "featured_model": featuredModel, "featured_copy": copy, "captcha_provider": captchaProvider, "geetest_captcha_id": geetestID, "has_geetest_captcha_key": strings.TrimSpace(geetestKeyEnc) != "", "corptcha_site_id": corptchaSiteID, "has_corptcha_secret": strings.TrimSpace(corptchaSecretEnc) != "", "smtp_host": smtpHost, "smtp_port": smtpPort, "smtp_username": smtpUser, "has_smtp_password": strings.TrimSpace(smtpPassEnc) != "", "smtp_from": smtpFrom, "public_base_url": publicBaseURL, "invitations_enabled": invitationsEnabled, "inviter_reward": inviterReward, "invitee_reward": inviteeReward, "registration_email_whitelist_enabled": whitelistEnabled, "registration_email_whitelist": whitelist, "registration_email_alias_blocked": aliasBlocked, "checkin_base_reward": checkinBaseReward, "checkin_streak_bonus": checkinStreakBonus, "checkin_max_bonus_days": checkinMaxBonusDays})
}

func (s *Service) updateSiteSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name                      string          `json:"name"`
		IconURL                   string          `json:"icon_url"`
		Announcement              string          `json:"announcement"`
		ContactEmail              *string         `json:"contact_email"`
		AutoDisableFailedChannels *bool           `json:"auto_disable_failed_channels"`
		FeaturedEnabled           *bool           `json:"featured_enabled"`
		FeaturedModel             *string         `json:"featured_model"`
		FeaturedCopy              json.RawMessage `json:"featured_copy"`
		CaptchaProvider           *string         `json:"captcha_provider"`
		GeetestCaptchaID          *string         `json:"geetest_captcha_id"`
		GeetestCaptchaKey         string          `json:"geetest_captcha_key"`
		CorptchaSiteID            *string         `json:"corptcha_site_id"`
		CorptchaSecret            string          `json:"corptcha_secret"`
		SMTPHost                  *string         `json:"smtp_host"`
		SMTPPort                  *string         `json:"smtp_port"`
		SMTPUsername              *string         `json:"smtp_username"`
		SMTPPassword              string          `json:"smtp_password"`
		SMTPFrom                  *string         `json:"smtp_from"`
		PublicBaseURL             *string         `json:"public_base_url"`
		InvitationsEnabled        *bool           `json:"invitations_enabled"`
		InviterReward             *float64        `json:"inviter_reward"`
		InviteeReward             *float64        `json:"invitee_reward"`
		CheckinBaseReward         *float64        `json:"checkin_base_reward"`
		CheckinStreakBonus        *float64        `json:"checkin_streak_bonus"`
		CheckinMaxBonusDays       *int            `json:"checkin_max_bonus_days"`
		WhitelistEnabled          *bool           `json:"registration_email_whitelist_enabled"`
		Whitelist                 []string        `json:"registration_email_whitelist"`
		AliasBlocked              *bool           `json:"registration_email_alias_blocked"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid site settings")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.IconURL = strings.TrimSpace(in.IconURL)
	in.Announcement = strings.TrimSpace(in.Announcement)
	if in.FeaturedModel != nil {
		model := strings.TrimSpace(*in.FeaturedModel)
		if len([]rune(model)) > maxFeaturedModelLen {
			writeError(w, http.StatusBadRequest, "invalid_request", "featured_model must be at most 200 characters")
			return
		}
		if model != "" && !validModelName(model) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid featured_model")
			return
		}
		*in.FeaturedModel = model
	}
	var featuredCopyJSON []byte
	if len(in.FeaturedCopy) > 0 {
		copy, err := decodeFeaturedCopy(in.FeaturedCopy)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		featuredCopyJSON, err = json.Marshal(copy)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not encode featured copy")
			return
		}
	}
	if in.ContactEmail != nil {
		email := strings.TrimSpace(*in.ContactEmail)
		if email != "" && (len(email) > maxContactEmailLen || !validEmail(email)) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid contact email")
			return
		}
		*in.ContactEmail = email
	}
	whitelist, err := normalizeEmailWhitelist(in.Whitelist)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
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
	if (in.InviterReward != nil && (*in.InviterReward < 0 || *in.InviterReward > 1e9)) || (in.InviteeReward != nil && (*in.InviteeReward < 0 || *in.InviteeReward > 1e9)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invitation rewards must be between 0 and 1e9")
		return
	}
	if (in.CheckinBaseReward != nil && (*in.CheckinBaseReward < 0 || *in.CheckinBaseReward > 1e9)) || (in.CheckinStreakBonus != nil && (*in.CheckinStreakBonus < 0 || *in.CheckinStreakBonus > 1e9)) || (in.CheckinMaxBonusDays != nil && (*in.CheckinMaxBonusDays < 1 || *in.CheckinMaxBonusDays > 365)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "check-in rewards must be non-negative and bonus days must be between 1 and 365")
		return
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
	var featuredCopyValue *string
	if featuredCopyJSON != nil {
		value := string(featuredCopyJSON)
		featuredCopyValue = &value
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set name=$1,icon_url=$2,announcement=$3,
			contact_email=coalesce($4,contact_email),
			auto_disable_failed_channels=coalesce($5,auto_disable_failed_channels),
			captcha_provider=coalesce($6,captcha_provider),
			geetest_captcha_id=coalesce($7,geetest_captcha_id),
			geetest_captcha_key_encrypted=case when $8='' then geetest_captcha_key_encrypted else $8 end,
			corptcha_site_id=coalesce($9,corptcha_site_id),
			corptcha_secret_encrypted=case when $10='' then corptcha_secret_encrypted else $10 end,
			smtp_host=coalesce($11,smtp_host),
			smtp_port=coalesce($12,smtp_port),
			smtp_username=coalesce($13,smtp_username),
			smtp_password_encrypted=case when $14='' then smtp_password_encrypted else $14 end,
			smtp_from=coalesce($15,smtp_from),
			public_base_url=coalesce($16,public_base_url),
			invitations_enabled=coalesce($17,invitations_enabled),
			inviter_reward=coalesce($18,inviter_reward),
			invitee_reward=coalesce($19,invitee_reward),
			checkin_base_reward=coalesce($20,checkin_base_reward),
			checkin_streak_bonus=coalesce($21,checkin_streak_bonus),
			checkin_max_bonus_days=coalesce($22,checkin_max_bonus_days),
			registration_email_whitelist_enabled=coalesce($23,registration_email_whitelist_enabled),
			registration_email_whitelist=$24,
			registration_email_alias_blocked=coalesce($25,registration_email_alias_blocked),
			featured_enabled=coalesce($26,featured_enabled),
			featured_model=coalesce($27,featured_model),
			featured_copy=coalesce($28::jsonb,featured_copy),
			updated_at=now() where id=true`,
		in.Name, in.IconURL, in.Announcement, trimmedPtr(in.ContactEmail), in.AutoDisableFailedChannels,
		trimmedPtr(in.CaptchaProvider), trimmedPtr(in.GeetestCaptchaID), geetestKeyEnc,
		trimmedPtr(in.CorptchaSiteID), corptchaSecretEnc,
		trimmedPtr(in.SMTPHost), trimmedPtr(in.SMTPPort), trimmedPtr(in.SMTPUsername), smtpPassEnc, trimmedPtr(in.SMTPFrom), trimmedPtr(in.PublicBaseURL), in.InvitationsEnabled, in.InviterReward, in.InviteeReward, in.CheckinBaseReward, in.CheckinStreakBonus, in.CheckinMaxBonusDays, in.WhitelistEnabled, whitelist, in.AliasBlocked, in.FeaturedEnabled, trimmedPtr(in.FeaturedModel), featuredCopyValue); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save site settings")
		return
	}
	s.audit(r, "settings.updated", "site_settings", "site", map[string]any{
		"name":             in.Name,
		"featured_enabled": in.FeaturedEnabled,
		"featured_model":   in.FeaturedModel,
		"featured_copy":    featuredCopyJSON != nil,
	})
	s.adminSiteSettings(w, r)
}

func defaultFeaturedCopy() featuredCopy {
	return featuredCopy{
		"zh":      {"badge": "推荐模型", "title": "探索最新模型", "body": "浏览已接入模型，按价格与分组选择适合你的模型。", "cta": "查看详情"},
		"zh-Hant": {"badge": "推薦模型", "title": "探索最新模型", "body": "瀏覽已接入模型，按價格與分組選擇適合你的模型。", "cta": "查看詳情"},
		"en":      {"badge": "Featured model", "title": "Explore the latest models", "body": "Browse connected models and choose one by price and group.", "cta": "View details"},
	}
}

func decodeFeaturedCopy(raw []byte) (featuredCopy, error) {
	var copy featuredCopy
	if err := json.Unmarshal(raw, &copy); err != nil {
		return nil, errors.New("featured_copy must be valid JSON")
	}
	if len(copy) != len(featuredLocales) {
		return nil, errors.New("featured_copy must contain only zh, zh-Hant and en locales")
	}
	for locale, fields := range copy {
		if !featuredLocales[locale] || len(fields) != len(featuredCopyFields) {
			return nil, errors.New("featured_copy must contain only badge, title, body and cta fields for each locale")
		}
		for field, value := range fields {
			if !featuredCopyFields[field] {
				return nil, errors.New("featured_copy must contain only badge, title, body and cta fields for each locale")
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, errors.New("featured_copy fields must not be empty")
			}
			if err := validatePlainText(value, featuredCopyFieldMaxLength[field]); err != nil {
				return nil, fmt.Errorf("featured_copy.%s.%s %w", locale, field, err)
			}
			fields[field] = value
		}
	}
	for locale := range featuredLocales {
		if _, ok := copy[locale]; !ok {
			return nil, errors.New("featured_copy must contain zh, zh-Hant and en locales")
		}
	}
	return copy, nil
}

func validatePlainText(value string, maxLength int) error {
	if len([]rune(value)) > maxLength {
		return fmt.Errorf("must be at most %d characters", maxLength)
	}
	if strings.ContainsAny(value, "<>") {
		return errors.New("must contain plain text only")
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return errors.New("must contain plain text only")
		}
	}
	return nil
}

var (
	featuredLocales = map[string]bool{
		"zh":      true,
		"zh-Hant": true,
		"en":      true,
	}
	featuredCopyFields = map[string]bool{
		"badge": true,
		"title": true,
		"body":  true,
		"cta":   true,
	}
	featuredCopyFieldMaxLength = map[string]int{
		"badge": 100,
		"title": 200,
		"body":  1000,
		"cta":   100,
	}
)

func isEmailAlias(email string) bool {
	local, _, ok := strings.Cut(email, "@")
	return ok && strings.Contains(local, "+")
}

func normalizeEmailWhitelist(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		email := strings.ToLower(strings.TrimSpace(value))
		if email == "" {
			continue
		}
		if !strings.Contains(email, "@") {
			email = "@" + strings.TrimPrefix(email, ".")
		}
		if strings.HasPrefix(email, "@") {
			if len(email) < 4 || strings.ContainsAny(email[1:], " @\t\r\n") || !strings.Contains(email[1:], ".") {
				return nil, errors.New("registration email whitelist contains an invalid email or domain suffix")
			}
		} else if !validEmail(email) {
			return nil, errors.New("registration email whitelist contains an invalid email or domain suffix")
		}
		if !seen[email] {
			seen[email] = true
			out = append(out, email)
		}
	}
	return out, nil
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
	maxContactEmailLen  = 255
	maxFeaturedModelLen = 200
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
