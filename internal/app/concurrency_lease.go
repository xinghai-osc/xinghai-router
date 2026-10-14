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

// concurrencyCountScript counts the unexpired leases per key. Members whose
// score is at or before Redis's own clock are already expired, so the exclusive
// lower bound keeps the read consistent with the acquire script's eviction.
const concurrencyCountScript = `
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
local out = {}
for i, key in ipairs(KEYS) do
  out[i] = redis.call('ZCOUNT', key, '(' .. now, '+inf')
end
return out
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

// counts returns the number of live leases held for each key, in key order. It
// reads the same keys the acquire path writes, so the answer is shared by every
// replica instead of reflecting only this process.
func (m *concurrencyLeaseManager) counts(ctx context.Context, scope string, keys []string) ([]int, error) {
	counts := make([]int, len(keys))
	if len(keys) == 0 {
		return counts, nil
	}
	client, err := newRedisClient(m.redisURL, 1)
	if err != nil {
		return nil, errConcurrencyUnavailable
	}
	defer client.close()
	redisKeys := make([]string, len(keys))
	for i, key := range keys {
		redisKeys[i] = "xh:concurrency:{leases}:" + scope + ":" + key
	}
	args := append([]string{"EVAL", concurrencyCountScript, strconv.Itoa(len(redisKeys))}, redisKeys...)
	reply, err := client.command(ctx, args...)
	if err != nil {
		return nil, errConcurrencyUnavailable
	}
	values, ok := reply.([]any)
	if !ok || len(values) != len(keys) {
		return nil, errConcurrencyUnavailable
	}
	for i, value := range values {
		count, ok := value.(int64)
		if !ok {
			return nil, errConcurrencyUnavailable
		}
		if count < 0 {
			count = 0
		}
		counts[i] = int(count)
	}
	return counts, nil
}

// trackConcurrency counts one request against the user and group scopes of
// this process. A max of 0 or less counts without ever rejecting, which is what
// cluster mode uses: the shared leases enforce the limit there, while the local
// counters still report this replica's share. It returns the scope that refused
// admission, or an empty scope with the release to run once the request ends.
func (s *Service) trackConcurrency(key keyContext, userMax, groupMax int) (func(), string) {
	trackUser := key.userID != ""
	if trackUser && !s.userLimiter.acquire(key.userID, userMax) {
		return nil, "user"
	}
	trackGroup := key.groupID != ""
	if trackGroup && !s.groupLimiter.acquire(key.groupID, groupMax) {
		if trackUser {
			s.userLimiter.release(key.userID)
		}
		return nil, "group"
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if trackUser {
				s.userLimiter.release(key.userID)
			}
			if trackGroup {
				s.groupLimiter.release(key.groupID)
			}
		})
	}, ""
}

func (s *Service) acquireRequestConcurrency(ctx context.Context, key keyContext) (context.Context, func(), string, error) {
	if s.cfg.DeploymentMode == "cluster" {
		releaseLocal, _ := s.trackConcurrency(key, 0, 0)
		userMax, groupMax := 0, 0
		if err := s.db.QueryRow(ctx, `select coalesce(u.max_concurrency,0),coalesce((select g.max_concurrency from groups g where g.id=nullif($2,'')::uuid),0) from users u where u.id=$1`, key.userID, key.groupID).Scan(&userMax, &groupMax); err != nil {
			releaseLocal()
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
			releaseLocal()
			return ctx, nil, "", errConcurrencyUnavailable
		}
		leaseCtx, releaseLease, blockedScope, err := s.concurrencyLeases.acquire(ctx, limits)
		if err != nil {
			releaseLocal()
			return ctx, nil, "", err
		}
		if blockedScope != "" {
			releaseLocal()
			return ctx, nil, blockedScope, nil
		}
		var once sync.Once
		return leaseCtx, func() {
			once.Do(func() {
				releaseLease()
				releaseLocal()
			})
		}, "", nil
	}
	userMax := s.userConcurrencyLimitFor(ctx, key.userID)
	groupMax := 0
	if key.groupID != "" {
		groupMax = s.groupConcurrencyLimitFor(ctx, key.groupID)
	}
	release, blockedScope := s.trackConcurrency(key, userMax, groupMax)
	if blockedScope != "" {
		return ctx, nil, blockedScope, nil
	}
	return ctx, release, "", nil
}

// acquireChannelConcurrency admits one in-flight request against a channel's
// max_concurrency. Unlike the user and group limits, a channel that is at its
// limit only makes that candidate unusable: the caller skips to the next
// channel, and reports channel_concurrency_exceeded only when every candidate
// was busy. The returned release must be held for the whole upstream exchange,
// streaming included, so the lease is not dropped while the body is still being
// relayed.
func (s *Service) acquireChannelConcurrency(ctx context.Context, ch channel) (context.Context, func(), bool, error) {
	key := strconv.FormatInt(ch.id, 10)
	// In cluster mode the shared lease enforces the limit, so this replica's
	// counter only has to report its own share; in single mode it enforces too.
	localMax := ch.maxConcurrency
	if s.cfg.DeploymentMode == "cluster" {
		localMax = 0
	}
	if s.channelLimiter != nil {
		if !s.channelLimiter.acquire(key, localMax) {
			return ctx, nil, true, nil
		}
	} else if localMax > 0 {
		return ctx, nil, false, errConcurrencyUnavailable
	}
	var localOnce sync.Once
	releaseLocal := func() {
		localOnce.Do(func() {
			if s.channelLimiter != nil {
				s.channelLimiter.release(key)
			}
		})
	}
	if ch.maxConcurrency <= 0 || s.cfg.DeploymentMode != "cluster" {
		return ctx, releaseLocal, false, nil
	}
	if s.concurrencyLeases == nil {
		releaseLocal()
		return ctx, nil, false, errConcurrencyUnavailable
	}
	leaseCtx, releaseLease, blockedScope, err := s.concurrencyLeases.acquire(ctx, []concurrencyLimit{{key: key, max: ch.maxConcurrency, scope: "channel"}})
	if err != nil {
		releaseLocal()
		return ctx, nil, false, err
	}
	if blockedScope != "" {
		releaseLocal()
		return ctx, nil, true, nil
	}
	var once sync.Once
	return leaseCtx, func() {
		once.Do(func() {
			releaseLease()
			releaseLocal()
		})
	}, false, nil
}
