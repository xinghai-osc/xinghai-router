package app

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	EncryptionKey      string
	ListenAddr         string
	RequestTimeout     time.Duration
	RateLimitPerMinute int
	TrustedProxies     string
	GeetestCaptchaID   string
	GeetestCaptchaKey  string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
}

// GeetestEnabled reports whether Geetest CAPTCHA verification is configured.
func (c Config) GeetestEnabled() bool {
	return c.GeetestCaptchaID != "" && c.GeetestCaptchaKey != ""
}

// EmailVerificationEnabled reports whether registration email codes are on.
func (c Config) EmailVerificationEnabled() bool {
	return c.SMTPHost != "" && c.SMTPFrom != ""
}

var insecureEncryptionKeys = map[string]bool{
	"change-this-encryption-key-before-production-2026": true,
	"replace-this-with-a-separate-random-secret-at-least-24-characters": true,
	"a-sufficiently-long-encryption-key": true,
}

func isInsecureEncryptionKey(key string) bool {
	key = strings.TrimSpace(key)
	if insecureEncryptionKeys[key] {
		return true
	}
	lower := strings.ToLower(key)
	return strings.Contains(lower, "change-this") || strings.Contains(lower, "replace-this")
}

func LoadConfig() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), RedisURL: os.Getenv("REDIS_URL"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), ListenAddr: env("LISTEN_ADDR", ":8080"), RequestTimeout: 90 * time.Second, RateLimitPerMinute: 60, TrustedProxies: strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")), GeetestCaptchaID: os.Getenv("GEETEST_CAPTCHA_ID"), GeetestCaptchaKey: os.Getenv("GEETEST_CAPTCHA_KEY"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "465"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: os.Getenv("SMTP_FROM")}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.EncryptionKey) < 24 {
		return c, fmt.Errorf("ENCRYPTION_KEY must contain at least 24 characters")
	}
	if isInsecureEncryptionKey(c.EncryptionKey) {
		return c, fmt.Errorf("ENCRYPTION_KEY is a documented placeholder; set a unique random secret")
	}
	if _, err := parseTrustedProxies(c.TrustedProxies); err != nil {
		return c, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	return c, nil
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
