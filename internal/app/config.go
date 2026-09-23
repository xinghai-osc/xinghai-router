package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL              string
	RedisURL                 string
	DeploymentMode           string
	RedisFailurePolicy       string
	ConcurrencyLeaseTTL      time.Duration
	EncryptionKey            string
	ListenAddr               string
	RateLimitPerMinute       int
	IPRateLimitPerMinute     int
	DBMaxConns               int
	TrustedProxies           string
	GeetestCaptchaID         string
	GeetestCaptchaKey        string
	CaptchaProvider          string
	CorptchaSiteID           string
	CorptchaSecret           string
	SMTPHost                 string
	SMTPPort                 string
	SMTPUsername             string
	SMTPPassword             string
	SMTPFrom                 string
	ConversationCacheDir     string
	LocalPromptCache         bool
	LocalPromptCacheSize     int
	BootstrapAdminEmail      string
	BootstrapAdminName       string
	BootstrapAdminPass       string
	GatewayMaxBodyBytes      int64
	ImageMaxBodyBytes        int64
	WSMaxMessageBytes        int64
	RequestBodyTimeout       time.Duration
	WSIdleTimeout            time.Duration
	HTTPReadHeaderTimeout    time.Duration
	HTTPIdleTimeout          time.Duration
	HTTPMaxHeaderBytes       int
	ChannelCredentialStorage string
	SessionCookieSecure      bool
}

// GeetestEnabled reports whether Geetest CAPTCHA verification is configured.
func (c Config) GeetestEnabled() bool {
	return c.GeetestCaptchaID != "" && c.GeetestCaptchaKey != ""
}

// CorptchaEnabled reports whether Corptcha CAPTCHA verification is configured.
func (c Config) CorptchaEnabled() bool {
	return c.CorptchaSiteID != "" && c.CorptchaSecret != ""
}

// EmailVerificationEnabled reports whether registration email codes are on.
func (c Config) EmailVerificationEnabled() bool {
	return c.SMTPHost != "" && c.SMTPFrom != ""
}

var insecureEncryptionKeys = map[string]bool{
	"change-this-encryption-key-before-production-2026":                 true,
	"replace-this-with-a-separate-random-secret-at-least-24-characters": true,
	"a-sufficiently-long-encryption-key":                                true,
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
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), RedisURL: os.Getenv("REDIS_URL"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), ListenAddr: env("LISTEN_ADDR", ":8080"), RateLimitPerMinute: 60, IPRateLimitPerMinute: envInt("IP_RATE_LIMIT_PER_MINUTE", 10), DBMaxConns: envInt("DB_MAX_CONNS", 0), TrustedProxies: strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")), GeetestCaptchaID: os.Getenv("GEETEST_CAPTCHA_ID"), GeetestCaptchaKey: os.Getenv("GEETEST_CAPTCHA_KEY"), CaptchaProvider: strings.ToLower(strings.TrimSpace(os.Getenv("CAPTCHA_PROVIDER"))), CorptchaSiteID: os.Getenv("CORPTCHA_SITE_ID"), CorptchaSecret: os.Getenv("CORPTCHA_SECRET"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "465"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: os.Getenv("SMTP_FROM"), ConversationCacheDir: env("CONVERSATION_CACHE_DIR", "data/conversations"), LocalPromptCache: envBool("LOCAL_PROMPT_CACHE", true), LocalPromptCacheSize: envInt("LOCAL_PROMPT_CACHE_SIZE", 4096), BootstrapAdminEmail: strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))), BootstrapAdminName: strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_NAME")), BootstrapAdminPass: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")}
	c.DeploymentMode = os.Getenv("DEPLOYMENT_MODE")
	c.RedisFailurePolicy = os.Getenv("REDIS_FAILURE_POLICY")
	if raw := os.Getenv("CONCURRENCY_LEASE_TTL"); raw != "" {
		var err error
		c.ConcurrencyLeaseTTL, err = time.ParseDuration(raw)
		if err != nil || c.ConcurrencyLeaseTTL <= 0 {
			return c, fmt.Errorf("CONCURRENCY_LEASE_TTL must be a positive duration")
		}
	}
	if err := c.normalizeDeployment(); err != nil {
		return c, err
	}
	if err := loadRequestLimits(&c); err != nil {
		return c, err
	}
	c.ChannelCredentialStorage = strings.ToLower(strings.TrimSpace(env("CHANNEL_CREDENTIAL_STORAGE", "plaintext")))
	if c.ChannelCredentialStorage != "plaintext" && c.ChannelCredentialStorage != "encrypted" {
		return c, fmt.Errorf("CHANNEL_CREDENTIAL_STORAGE must be plaintext or encrypted")
	}
	c.SessionCookieSecure = true
	if raw, exists := os.LookupEnv("SESSION_COOKIE_SECURE"); exists {
		var err error
		c.SessionCookieSecure, err = strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return c, fmt.Errorf("SESSION_COOKIE_SECURE must be a boolean")
		}
	}
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
func envInt(k string, fallback int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := parseInt(v); err == nil {
			return n
		}
	}
	return fallback
}
func envBool(k string, fallback bool) bool {
	if v := os.Getenv(k); v != "" {
		return v == "1" || strings.EqualFold(v, "true")
	}
	return fallback
}
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
