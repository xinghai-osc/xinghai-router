package app

import (
	"fmt"
	"strings"
	"time"
)

func (c *Config) normalizeDeployment() error {
	c.DeploymentMode = strings.ToLower(strings.TrimSpace(c.DeploymentMode))
	if c.DeploymentMode == "" {
		c.DeploymentMode = "single"
	}
	if c.DeploymentMode != "single" && c.DeploymentMode != "cluster" {
		return fmt.Errorf("DEPLOYMENT_MODE must be single or cluster")
	}
	c.RedisURL = strings.TrimSpace(c.RedisURL)
	c.RedisFailurePolicy = strings.ToLower(strings.TrimSpace(c.RedisFailurePolicy))
	if c.RedisFailurePolicy == "" {
		c.RedisFailurePolicy = "memory"
		if c.DeploymentMode == "cluster" {
			c.RedisFailurePolicy = "deny"
		}
	}
	if c.RedisFailurePolicy != "memory" && c.RedisFailurePolicy != "deny" {
		return fmt.Errorf("REDIS_FAILURE_POLICY must be memory or deny")
	}
	if c.RedisFailurePolicy == "deny" && c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required for REDIS_FAILURE_POLICY=deny")
	}
	if c.DeploymentMode == "cluster" {
		if c.RedisFailurePolicy != "deny" {
			return fmt.Errorf("DEPLOYMENT_MODE=cluster requires REDIS_FAILURE_POLICY=deny; memory fallback cannot enforce global limits")
		}
		if c.LocalPromptCache {
			return fmt.Errorf("DEPLOYMENT_MODE=cluster requires LOCAL_PROMPT_CACHE=false; local prompt discounts differ between replicas")
		}
	}
	if c.ConcurrencyLeaseTTL == 0 {
		c.ConcurrencyLeaseTTL = 30 * time.Second
	}
	if c.ConcurrencyLeaseTTL < 5*time.Second || c.ConcurrencyLeaseTTL > 5*time.Minute {
		return fmt.Errorf("CONCURRENCY_LEASE_TTL must be between 5s and 5m")
	}
	return nil
}
