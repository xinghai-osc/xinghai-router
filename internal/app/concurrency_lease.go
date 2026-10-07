package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

const concurrencyAcquireScript = `
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
for i, key in ipairs(KEYS) do
  redis.call('ZREMRANGEBYSCORE', key, '-inf', now)
  if redis.call('ZCARD', key) >= tonumber(ARGV[i + 2]) then
    return i
  end
end
for _, key in ipairs(KEYS) do
  redis.call('ZADD', key, now + tonumber(ARGV[2]), ARGV[1])
  if redis.call('PTTL', key) < tonumber(ARGV[2]) then
    redis.call('PEXPIRE', key, ARGV[2])
  end
end
return 0
`

const concurrencyRenewScript = `
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
for _, key in ipairs(KEYS) do
  local expiry = redis.call('ZSCORE', key, ARGV[1])
  if not expiry or tonumber(expiry) <= now then
    return 0
  end
end
for _, key in ipairs(KEYS) do
  redis.call('ZADD', key, now + tonumber(ARGV[2]), ARGV[1])
  if redis.call('PTTL', key) < tonumber(ARGV[2]) then
    redis.call('PEXPIRE', key, ARGV[2])
  end
end
return 1
`

const concurrencyReleaseScript = `
for _, key in ipairs(KEYS) do
  redis.call('ZREM', key, ARGV[1])
end
return 1
`

var errConcurrencyUnavailable = errors.New("concurrency backend unavailable")

type concurrencyLimit struct {
	key   string
	max   int
	scope string
}

type concurrencyLeaseManager struct {
	redisURL string
	ttl      time.Duration
	alert    redisDependencyAlert
}

func (m *concurrencyLeaseManager) acquire(ctx context.Context, limits []concurrencyLimit) (context.Context, func(), string, error) {
	if len(limits) == 0 {
		return ctx, func() {}, "", nil
	}
	client, err := newRedisClient(m.redisURL, 1)
	if err != nil {
		return ctx, nil, "", errConcurrencyUnavailable
	}
	token, err := randomID()
	if err != nil {
		client.close()
		return ctx, nil, "", errConcurrencyUnavailable
	}
	keys := make([]string, len(limits))
	for i, limit := range limits {
		keys[i] = "xh:concurrency:{leases}:" + limit.scope + ":" + limit.key
	}
	args := append([]string{"EVAL", concurrencyAcquireScript, strconv.Itoa(len(keys))}, keys...)
	args = append(args, token, strconv.FormatInt(m.ttl.Milliseconds(), 10))
	for _, limit := range limits {
		args = append(args, strconv.Itoa(limit.max))
	}
	started := time.Now()
	reply, err := client.command(ctx, args...)
	if err != nil {
		if ctx.Err() == nil {
			m.alert.setDegraded(true, "concurrency", "deny")
		}
		client.close()
		return ctx, nil, "", errConcurrencyUnavailable
	}
	blocked, ok := reply.(int64)
	if !ok || blocked < 0 || blocked > int64(len(limits)) {
		m.alert.setDegraded(true, "concurrency", "deny")
		client.close()
		return ctx, nil, "", errConcurrencyUnavailable
	}
	m.alert.setDegraded(false, "concurrency", "deny")
	if blocked > 0 {
		client.close()
		return ctx, nil, limits[blocked-1].scope, nil
	}
	if ctx.Err() != nil || !time.Now().Before(started.Add(m.ttl*2/3)) {
		m.releaseToken(client, keys, token)
		return ctx, nil, "", errConcurrencyUnavailable
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	release := func() {
		once.Do(func() {
			close(stop)
			cancel()
			<-done
			m.releaseToken(client, keys, token)
		})
	}
	go func() {
		defer close(done)
		defer cancel()
		renewArgs := append([]string{"EVAL", concurrencyRenewScript, strconv.Itoa(len(keys))}, keys...)
		renewArgs = append(renewArgs, token, strconv.FormatInt(m.ttl.Milliseconds(), 10))
		ticker := time.NewTicker(m.ttl / 3)
		defer ticker.Stop()
		validUntil := started.Add(m.ttl * 2 / 3)
		for {
			select {
			case <-stop:
				return
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				renewStarted := time.Now()
				renewCtx, renewCancel := context.WithDeadline(leaseCtx, validUntil)
				result, renewErr := client.command(renewCtx, renewArgs...)
				renewCancel()
				if leaseCtx.Err() != nil {
					return
				}
				m.alert.setDegraded(renewErr != nil, "concurrency", "deny")
				if renewErr != nil || result != int64(1) || time.Now().After(validUntil) {
					log.Printf("event=concurrency_lease_lost policy=deny action=cancel_request")
					return
				}
				validUntil = renewStarted.Add(m.ttl * 2 / 3)
			}
		}
	}()
	return leaseCtx, release, "", nil
}

func (m *concurrencyLeaseManager) releaseToken(client *redisLimiter, keys []string, token string) {
	defer client.close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := append([]string{"EVAL", concurrencyReleaseScript, strconv.Itoa(len(keys))}, keys...)
	args = append(args, token)
	if _, err := client.command(ctx, args...); err != nil {
		m.alert.setDegraded(true, "concurrency", "deny")
	}
}

func (s *Service) acquireRequestConcurrency(ctx context.Context, key keyContext) (context.Context, func(), string, error) {
	userMax, groupMax := 0, 0
	if s.cfg.DeploymentMode == "cluster" {
		if err := s.db.QueryRow(ctx, `select coalesce(u.max_concurrency,0),coalesce((select g.max_concurrency from groups g where g.id=nullif($2,'')::uuid),0) from users u where u.id=$1`, key.userID, key.groupID).Scan(&userMax, &groupMax); err != nil {
			return ctx, nil, "", fmt.Errorf("load concurrency limits: %w", err)
		}
		limits := make([]concurrencyLimit, 0, 2)
		if userMax > 0 {
			limits = append(limits, concurrencyLimit{key: key.userID, max: userMax, scope: "user"})
		}
		if key.groupID != "" && groupMax > 0 {
			limits = append(limits, concurrencyLimit{key: key.groupID, max: groupMax, scope: "group"})
		}
		if s.concurrencyLeases == nil {
			return ctx, nil, "", errConcurrencyUnavailable
		}
		return s.concurrencyLeases.acquire(ctx, limits)
	}
	userMax = s.userConcurrencyLimitFor(ctx, key.userID)
	if key.groupID != "" {
		groupMax = s.groupConcurrencyLimitFor(ctx, key.groupID)
	}
	if userMax > 0 && !s.userLimiter.acquire(key.userID, userMax) {
		return ctx, nil, "user", nil
	}
	if groupMax > 0 && !s.groupLimiter.acquire(key.groupID, groupMax) {
		if userMax > 0 {
			s.userLimiter.release(key.userID)
		}
		return ctx, nil, "group", nil
	}
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			if userMax > 0 {
				s.userLimiter.release(key.userID)
			}
			if groupMax > 0 {
				s.groupLimiter.release(key.groupID)
			}
		})
	}, "", nil
}

// acquireChannelConcurrency admits one in-flight request against a channel's
// max_concurrency. Unlike the user and group limits, a channel that is at its
// limit only makes that candidate unusable: the caller skips to the next
// channel, and reports channel_concurrency_exceeded only when every candidate
// was busy. The returned release must be held for the whole upstream exchange,
// streaming included, so the lease is not dropped while the body is still being
// relayed.
func (s *Service) acquireChannelConcurrency(ctx context.Context, ch channel) (context.Context, func(), bool, error) {
	if ch.maxConcurrency <= 0 {
		return ctx, func() {}, false, nil
	}
	key := strconv.FormatInt(ch.id, 10)
	if s.cfg.DeploymentMode == "cluster" {
		if s.concurrencyLeases == nil {
			return ctx, nil, false, errConcurrencyUnavailable
		}
		leaseCtx, release, blockedScope, err := s.concurrencyLeases.acquire(ctx, []concurrencyLimit{{key: key, max: ch.maxConcurrency, scope: "channel"}})
		if err != nil {
			return ctx, nil, false, err
		}
		if blockedScope != "" {
			return ctx, nil, true, nil
		}
		return leaseCtx, release, false, nil
	}
	if s.channelLimiter == nil {
		return ctx, nil, false, errConcurrencyUnavailable
	}
	if !s.channelLimiter.acquire(key, ch.maxConcurrency) {
		return ctx, nil, true, nil
	}
	var once sync.Once
	return ctx, func() { once.Do(func() { s.channelLimiter.release(key) }) }, false, nil
}
