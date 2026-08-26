package app

import (
	"context"
	"log"
	"time"
)

const (
	authCleanupInterval = time.Hour
	pendingOrderMaxAge  = 24 * time.Hour
	pendingOrderAgeSQL  = "24 hours"
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
				s.cleanupContentAudits(ctx)
				timer.Reset(authCleanupInterval)
			}
		}
	}()
}

func (s *Service) cleanupExpiredAuthState(ctx context.Context) {
	if s.db == nil {
		return
	}
	sessionN, codeN, resetN := int64(0), int64(0), int64(0)
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
	if tag, err := s.db.Exec(ctx, `delete from password_reset_tokens where expires_at < now() or consumed_at is not null`); err != nil {
		log.Printf("auth cleanup: delete expired password reset tokens: %v", err)
	} else {
		resetN = tag.RowsAffected()
	}
	if sessionN > 0 || codeN > 0 || resetN > 0 {
		log.Printf("auth cleanup: removed %d expired sessions, %d email verification codes, and %d password reset tokens", sessionN, codeN, resetN)
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
	lapsedN := int64(0)
	if tag, err := s.db.Exec(ctx, `update user_subscriptions set status='expired', updated_at=now()
		where status='active' and current_period_end is not null and current_period_end < now()`); err != nil {
		log.Printf("order cleanup: expire lapsed active subscriptions: %v", err)
	} else {
		lapsedN = tag.RowsAffected()
	}
	// Revoke the plan group from users whose subscriptions to it no longer
	// cover it, so an expired subscription does not keep granting group access.
	// Only groups tied to a subscription plan are touched, and membership is
	// kept while any active subscription still covers the group (e.g. another
	// plan or a renewed subscription sharing the same group).
	groupN := int64(0)
	if tag, err := s.db.Exec(ctx, `delete from user_groups ug
		where exists (
			select 1
			from subscription_plans p
			join user_subscriptions us on us.plan_id=p.id
			where p.group_id=ug.group_id and us.user_id=ug.user_id
		)
		  and not exists (
			select 1
			from subscription_plans p
			join user_subscriptions us on us.plan_id=p.id
			where p.group_id=ug.group_id and us.user_id=ug.user_id
			  and us.status='active'
			  and (us.current_period_end is null or us.current_period_end > now())
		  )`); err != nil {
		log.Printf("order cleanup: revoke lapsed subscription groups: %v", err)
	} else {
		groupN = tag.RowsAffected()
	}
	if payN > 0 || subOrderN > 0 || subN > 0 || lapsedN > 0 || groupN > 0 {
		log.Printf("order cleanup: expired %d payment orders, %d subscription orders, cancelled %d pending subscriptions, marked %d lapsed actives older than %s, revoked %d subscription groups", payN, subOrderN, subN, lapsedN, pendingOrderMaxAge, groupN)
		s.subscriptionCache.clear()
		if groupN > 0 {
			s.invalidateChannels()
		}
	}
}
