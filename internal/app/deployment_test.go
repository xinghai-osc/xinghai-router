package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDeploymentConfiguration(t *testing.T) {
	for _, test := range []struct {
		name      string
		cfg       Config
		wantError string
	}{
		{name: "default"},
		{name: "cluster", cfg: Config{DeploymentMode: "cluster", RedisURL: "redis://localhost:6379"}},
		{name: "invalid mode", cfg: Config{DeploymentMode: "replicas"}, wantError: "DEPLOYMENT_MODE"},
		{name: "missing Redis", cfg: Config{DeploymentMode: "cluster"}, wantError: "REDIS_URL"},
		{name: "unsafe fallback", cfg: Config{DeploymentMode: "cluster", RedisURL: "redis://localhost:6379", RedisFailurePolicy: "memory"}, wantError: "requires REDIS_FAILURE_POLICY=deny"},
		{name: "unsafe billing cache", cfg: Config{DeploymentMode: "cluster", RedisURL: "redis://localhost:6379", LocalPromptCache: true}, wantError: "LOCAL_PROMPT_CACHE=false"},
		{name: "deny without Redis", cfg: Config{RedisFailurePolicy: "deny"}, wantError: "REDIS_URL"},
		{name: "invalid policy", cfg: Config{RedisFailurePolicy: "allow"}, wantError: "REDIS_FAILURE_POLICY"},
		{name: "short lease", cfg: Config{ConcurrencyLeaseTTL: time.Second}, wantError: "CONCURRENCY_LEASE_TTL"},
		{name: "long lease", cfg: Config{ConcurrencyLeaseTTL: time.Hour}, wantError: "CONCURRENCY_LEASE_TTL"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.cfg
			err := cfg.normalizeDeployment()
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error=%v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantPolicy := "memory"
			if cfg.DeploymentMode == "cluster" {
				wantPolicy = "deny"
			}
			if cfg.RedisFailurePolicy != wantPolicy || cfg.ConcurrencyLeaseTTL != 30*time.Second {
				t.Fatalf("unexpected configuration: mode=%s policy=%s ttl=%s", cfg.DeploymentMode, cfg.RedisFailurePolicy, cfg.ConcurrencyLeaseTTL)
			}
		})
	}
}

func TestLoadDeploymentEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test@localhost/test")
	t.Setenv("ENCRYPTION_KEY", "unit-test-encryption-key-not-for-prod")
	t.Setenv("DEPLOYMENT_MODE", "cluster")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("REDIS_FAILURE_POLICY", "")
	t.Setenv("LOCAL_PROMPT_CACHE", "false")
	t.Setenv("CONCURRENCY_LEASE_TTL", "15s")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeploymentMode != "cluster" || cfg.RedisFailurePolicy != "deny" || cfg.ConcurrencyLeaseTTL != 15*time.Second {
		t.Fatal("deployment env not applied")
	}
	for _, ttl := range []string{"invalid", "0s", "-1s", "1s", "10m"} {
		t.Setenv("CONCURRENCY_LEASE_TTL", ttl)
		if _, err := LoadConfig(); err == nil {
			t.Fatalf("accepted invalid ttl %s", ttl)
		}
	}
}

func TestNewRejectsUnsafeDeploymentBeforeDatabase(t *testing.T) {
	if _, err := New(context.Background(), Config{DeploymentMode: "cluster"}); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}
