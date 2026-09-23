package app

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

const configInvalidationChannel = "xinghai_config_invalidation"

func (s *Service) invalidateConfigCaches() {
	if s == nil {
		return
	}
	s.pricingCache.clear()
	s.groupCache.clear()
	s.groupConcurrencyCache.clear()
	s.userConcurrencyCache.clear()
	s.reliabilityData.clear()
	s.contentPolicyData.clear()
	s.conversationCacheData.clear()
	s.channelKeyCache.clear()
	s.channelCache.clear()
	s.subscriptionCache.clear()
	s.channelQuotaCache.clear()
	s.quotaAbsentCache.clear()
	s.rankingsCache.clear()
	s.performanceCache.clear()
}

func (s *Service) startConfigInvalidation(ctx context.Context) {
	if s == nil || s.db == nil || ctx.Err() != nil {
		return
	}
	cfg := s.db.Config().ConnConfig.Copy()
	cfg.RuntimeParams["application_name"] = "xinghai-config-invalidation"
	go s.runConfigInvalidation(ctx, cfg)
}

func (s *Service) runConfigInvalidation(ctx context.Context, cfg *pgx.ConnConfig) {
	retry := time.Second
	unavailable := false
	for ctx.Err() == nil {
		started := time.Now()
		_ = s.listenConfigInvalidation(ctx, cfg, func() {
			if unavailable {
				log.Printf("config_invalidation state=recovered")
				unavailable = false
			}
		})
		if ctx.Err() != nil {
			return
		}
		s.invalidateConfigCaches()
		if !unavailable {
			log.Printf("config_invalidation state=unavailable fallback=ttl")
			unavailable = true
		}
		if time.Since(started) >= time.Minute {
			retry = time.Second
		}
		timer := time.NewTimer(retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		retry = min(retry*2, 30*time.Second)
	}
}

func (s *Service) listenConfigInvalidation(ctx context.Context, cfg *pgx.ConnConfig, ready func()) error {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(connectCtx, cfg.Copy())
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err := conn.Exec(connectCtx, "LISTEN "+configInvalidationChannel); err != nil {
		return err
	}
	cancel()
	s.invalidateConfigCaches()
	ready()
	for {
		waitCtx, waitCancel := context.WithTimeout(ctx, 15*time.Second)
		_, err := conn.WaitForNotification(waitCtx)
		waitExpired := waitCtx.Err() == context.DeadlineExceeded
		waitCancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if !waitExpired {
				return err
			}
			pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
			err = conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return err
			}
			continue
		}
		s.invalidateConfigCaches()
	}
}
