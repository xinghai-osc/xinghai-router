package app

import (
	"context"
	"log"
	"time"
)

func (s *Service) cleanupRewardRisk(ctx context.Context) {
	if s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, query := range []string{
		`delete from risk_contexts where id in (select id from risk_contexts where expires_at<clock_timestamp() order by expires_at limit 10000)`,
		`delete from risk_bans where id in (select id from risk_bans where released_at<clock_timestamp()-interval '180 days' order by released_at limit 1000)`,
		`delete from risk_observations where id in (select o.id from risk_observations o where o.created_at<clock_timestamp()-interval '30 days' and not exists(select 1 from reward_claims c where c.risk_event_id=o.id and c.status='pending') and not exists(select 1 from risk_bans b where b.observation_id=o.id) order by o.created_at limit 10000)`,
	} {
		if _, err := s.db.Exec(ctx, query); err != nil {
			log.Printf("reward risk cleanup failed: %v", err)
			return
		}
	}
}
