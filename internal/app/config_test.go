package app

import (
	"strings"
	"testing"
)

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENCRYPTION_KEY", "a-sufficiently-long-encryption-key")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadConfigRequiresEncryptionKeyLength(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://router:pass@localhost:5432/router")
	t.Setenv("ENCRYPTION_KEY", "too-short")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Fatalf("expected ENCRYPTION_KEY error, got %v", err)
	}
}

func TestLoadConfigRejectsPlaceholderEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://router:pass@localhost:5432/router")
	for _, key := range []string{
		"change-this-encryption-key-before-production-2026",
		"replace-this-with-a-separate-random-secret-at-least-24-characters",
		"please-change-this-secret-value-now",
	} {
		t.Setenv("ENCRYPTION_KEY", key)
		if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "placeholder") {
			t.Fatalf("expected placeholder error for %q, got %v", key, err)
		}
	}
}

func TestLoadConfigDefaultsAndFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://router:pass@localhost:5432/router")
	t.Setenv("ENCRYPTION_KEY", "unit-test-encryption-key-not-for-prod")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("GEETEST_CAPTCHA_ID", "")
	t.Setenv("GEETEST_CAPTCHA_KEY", "")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.SMTPPort != "465" {
		t.Fatalf("SMTPPort = %q, want 465", cfg.SMTPPort)
	}
	if cfg.GeetestEnabled() {
		t.Fatal("expected Geetest disabled without both credentials")
	}
	if cfg.EmailVerificationEnabled() {
		t.Fatal("expected email verification disabled without SMTP host/from")
	}

	t.Setenv("GEETEST_CAPTCHA_ID", "id")
	t.Setenv("GEETEST_CAPTCHA_KEY", "key")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.GeetestEnabled() {
		t.Fatal("expected Geetest enabled")
	}
	if !cfg.EmailVerificationEnabled() {
		t.Fatal("expected email verification enabled")
	}
}

func TestLoadConfigRejectsInvalidTrustedProxies(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://router:pass@localhost:5432/router")
	t.Setenv("ENCRYPTION_KEY", "unit-test-encryption-key-not-for-prod")
	t.Setenv("TRUSTED_PROXIES", "not-a-valid-proxy-spec")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected TRUSTED_PROXIES error, got %v", err)
	}
	t.Setenv("TRUSTED_PROXIES", "loopback,10.0.0.0/8")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrustedProxies != "loopback,10.0.0.0/8" {
		t.Fatalf("TrustedProxies = %q", cfg.TrustedProxies)
	}
}
