package app

import (
	"context"
	"log"
	"time"
)

const authCleanupInterval = time.Hour

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
