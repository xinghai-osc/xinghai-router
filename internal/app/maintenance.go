package app

import (
	"context"
	"log"
	"time"
)

const (
	authCleanupInterval   = time.Hour
	pendingOrderMaxAge    = 24 * time.Hour
	pendingOrderAgeSQL    = "24 hours"
)

func (s *Service) startAuthCleanupScheduler(ctx context.Context) {
	go func() {
		// Run once shortly after boot, then on a fixed interval.
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.cleanupExpiredAuthState(ctx)
				s.expireStalePendingOrders(ctx)
				timer.Reset(authCleanupInterval)
			}
		}
	}()
}

func (s *Service) cleanupExpiredAuthState(ctx context.Context) {
	if s.db == nil {
		return
	}
	sessionN, codeN := int64(0), int64(0)
	if tag, err := s.db.Exec(ctx, `delete from user_sessions where expires_at < now()`); err != nil {
		log.Printf("auth cleanup: delete expired sessions: %v", err)
	} else {
		sessionN = tag.RowsAffected()
	}
	if tag, err := s.db.Exec(ctx, `delete from email_verification_codes where expires_at < now() or consumed_at is not null`); err != nil {
		log.Printf("auth cleanup: delete expired email codes: %v", err)
	} else {
		codeN = tag.RowsAffected()
	}
	if sessionN > 0 || codeN > 0 {
		log.Printf("auth cleanup: removed %d expired sessions and %d email verification codes", sessionN, codeN)
	}
}

func (s *Service) expireStalePendingOrders(ctx context.Context) {
	if s.db == nil {
		return
	}
	payN, subOrderN, subN := int64(0), int64(0), int64(0)
	if tag, err := s.db.Exec(ctx, `update payment_orders set status='expired', updated_at=now() where status='pending' and created_at < now() - $1::interval`, pendingOrderAgeSQL); err != nil {
		log.Printf("order cleanup: expire payment orders: %v", err)
	} else {
		payN = tag.RowsAffected()
	}
	if tag, err := s.db.Exec(ctx, `update subscription_orders set status='expired', updated_at=now() where status='pending' and created_at < now() - $1::interval`, pendingOrderAgeSQL); err != nil {
		log.Printf("order cleanup: expire subscription orders: %v", err)
	} else {
		subOrderN = tag.RowsAffected()
	}
	// Cancel pending subscriptions whose only unpaid orders have aged out, so
	// they do not remain indefinitely selectable as pending.
	if tag, err := s.db.Exec(ctx, `update user_subscriptions us set status='cancelled', cancelled_at=coalesce(us.cancelled_at, now()), updated_at=now()
		where us.status='pending'
		  and us.created_at < now() - $1::interval
		  and not exists (
			select 1 from subscription_orders o
			where o.subscription_id=us.id and o.status in ('pending','paid')
		  )`, pendingOrderAgeSQL); err != nil {
		log.Printf("order cleanup: cancel pending subscriptions: %v", err)
	} else {
		subN = tag.RowsAffected()
	}
	if payN > 0 || subOrderN > 0 || subN > 0 {
		log.Printf("order cleanup: expired %d payment orders, %d subscription orders, cancelled %d pending subscriptions older than %s", payN, subOrderN, subN, pendingOrderMaxAge)
	}
}
