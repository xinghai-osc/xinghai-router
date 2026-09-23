//go:build integration

package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func configInvalidationExpectNotification(t *testing.T, conn *pgx.Conn, expected bool) {
	t.Helper()
	timeout := 150 * time.Millisecond
	if expected {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	notification, err := conn.WaitForNotification(ctx)
	if !expected {
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected notification=%v error=%v", notification, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("wait for notification: %v", err)
	}
	if notification.Channel != configInvalidationChannel || notification.Payload != "" {
		t.Fatalf("unexpected notification: %+v", notification)
	}
}

func TestIntegrationConfigInvalidationTransactions(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	migration, err := migrations.ReadFile("migrations/098_config_invalidation.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := db.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("reapply invalidation migration: %v", err)
		}
	}
	conn, err := pgx.ConnectConfig(ctx, db.Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "LISTEN "+configInvalidationChannel); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var groupID string
	if err := tx.QueryRow(ctx, `insert into groups(id,name) values(gen_random_uuid(),'invalidation') returning id`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `update groups set multiplier=2 where id=$1`, groupID); err != nil {
		t.Fatal(err)
	}
	configInvalidationExpectNotification(t, conn, false)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	configInvalidationExpectNotification(t, conn, true)
	configInvalidationExpectNotification(t, conn, false)

	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `delete from groups where id=$1`, groupID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	configInvalidationExpectNotification(t, conn, false)

	for _, table := range []string{
		"pricing_rules", "pricing_tiers", "pricing_time_rules", "exchange_rates", "groups",
		"channel_groups", "user_groups", "model_routes", "model_providers", "site_settings",
		"content_policy_rules", "quota_limits", "channel_quota_limits", "subscription_plans",
		"subscription_plan_model_quotas", "channels", "channel_api_keys", "users", "user_subscriptions",
	} {
		var count int
		if err := db.QueryRow(ctx, `select count(*) from pg_trigger where tgrelid=$1::regclass and not tgisinternal and tgname like 'config_invalidation%'`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Errorf("%s has no invalidation triggers", table)
		}
	}
}

func TestIntegrationConfigInvalidationIgnoresRuntimeUpdates(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	userID, _ := integrationUser(t, db, "config-invalidation@example.com", 10)
	var channelID int64
	if err := db.QueryRow(ctx, `insert into channels(name,base_url,api_key) values('invalidation','https://example.invalid','fake-key') returning id`).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	var keyID, planID, subscriptionID string
	if err := db.QueryRow(ctx, `insert into channel_api_keys(channel_id,key_encrypted) values($1,'fake-key') returning id`, channelID).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `insert into subscription_plans(id,name,billing_period) values(gen_random_uuid(),'invalidation','month') returning id`).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `insert into user_subscriptions(id,user_id,plan_id,status,remaining_requests) values(gen_random_uuid(),$1,$2,'active',100) returning id`, userID, planID).Scan(&subscriptionID); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.ConnectConfig(ctx, db.Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "LISTEN "+configInvalidationChannel); err != nil {
		t.Fatal(err)
	}
	for _, query := range []struct {
		name   string
		sql    string
		id     any
		notify bool
	}{
		{"channel health", `update channels set failure_count=failure_count+1,last_checked_at=now(),updated_at=now(),enabled=enabled,auto_disabled=auto_disabled where id=$1`, channelID, false},
		{"key health", `update channel_api_keys set failure_count=failure_count+1,last_checked_at=now() where id=$1`, keyID, false},
		{"subscription usage", `update user_subscriptions set remaining_requests=remaining_requests-1,remaining_credit=0,updated_at=now() where id=$1`, subscriptionID, false},
		{"wallet", `update user_wallets set balance=balance+1,updated_at=now() where user_id=$1`, userID, false},
		{"user login", `update users set password_hash='fake-updated-hash' where id=$1`, userID, false},
		{"channel disable", `update channels set auto_disabled=true where id=$1`, channelID, true},
		{"key disable", `update channel_api_keys set enabled=false where id=$1`, keyID, true},
		{"user limit", `update users set max_concurrency=2 where id=$1`, userID, true},
		{"user unchanged", `update users set max_concurrency=2 where id=$1`, userID, false},
		{"subscription cancel", `update user_subscriptions set status='cancelled' where id=$1`, subscriptionID, true},
		{"subscription delete", `delete from user_subscriptions where id=$1`, subscriptionID, true},
		{"key delete", `delete from channel_api_keys where id=$1`, keyID, true},
	} {
		t.Run(query.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, query.sql, query.id); err != nil {
				t.Fatal(err)
			}
			configInvalidationExpectNotification(t, conn, query.notify)
		})
	}
}

func configInvalidationEventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for config invalidation")
		case <-ticker.C:
		}
	}
}

func TestIntegrationConfigInvalidationMultipleServices(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := db.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var groupID string
	if err := pool.QueryRow(ctx, `insert into groups(id,name,multiplier) values(gen_random_uuid(),'replicas',1) returning id`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	services := []*Service{configInvalidationTestService(), configInvalidationTestService()}
	for _, service := range services {
		service.db = pool
		service.groupCache.store(groupID, 99)
		service.startConfigInvalidation(ctx)
	}
	pids := func() []int32 {
		rows, err := db.Query(context.Background(), `select pid from pg_stat_activity where datname=current_database() and application_name='xinghai-config-invalidation' order by pid`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var result []int32
		for rows.Next() {
			var pid int32
			if err := rows.Scan(&pid); err != nil {
				t.Fatal(err)
			}
			result = append(result, pid)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Cleanup(func() {
		cancel()
	})
	configInvalidationEventually(t, func() bool {
		if len(pids()) != len(services) {
			return false
		}
		for _, service := range services {
			if _, ok := service.groupCache.lookup(groupID); ok {
				return false
			}
		}
		return true
	})
	if got := pool.Stat().AcquiredConns(); got != 0 {
		t.Fatalf("listeners occupy %d pool slots", got)
	}
	for _, service := range services {
		if got := service.groupMultiplierFor(ctx, groupID); got != 1 {
			t.Fatalf("initial group multiplier = %v", got)
		}
		service.channelCache.store(channelRouteKey{}, []channel{{id: 1}})
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `update groups set multiplier=2 where id=$1`, groupID); err != nil {
		t.Fatal(err)
	}
	for _, service := range services {
		if got, ok := service.groupCache.lookup(groupID); !ok || got != 1 {
			t.Fatalf("uncommitted transaction changed cache = %v, %v", got, ok)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	configInvalidationEventually(t, func() bool {
		for _, service := range services {
			if _, ok := service.groupCache.lookup(groupID); ok {
				return false
			}
			if _, ok := service.channelCache.lookup(channelRouteKey{}); ok {
				return false
			}
		}
		return true
	})
	for _, service := range services {
		if got := service.groupMultiplierFor(ctx, groupID); got != 2 {
			t.Fatalf("replica group multiplier = %v", got)
		}
	}
	before := pids()
	generations := make([]uint64, len(services))
	for i, service := range services {
		_, _, generations[i] = service.groupCache.lookupGeneration(groupID)
	}
	for _, pid := range before {
		if _, err := db.Exec(ctx, `select pg_terminate_backend($1)`, pid); err != nil {
			t.Fatal(err)
		}
	}
	configInvalidationEventually(t, func() bool {
		after := pids()
		if len(after) != len(services) {
			return false
		}
		for _, pid := range after {
			if slices.Contains(before, pid) {
				return false
			}
		}
		for i, service := range services {
			_, present, generation := service.groupCache.lookupGeneration(groupID)
			if present || generation < generations[i]+2 {
				return false
			}
		}
		return true
	})
	cancel()
	configInvalidationEventually(t, func() bool { return len(pids()) == 0 })
}
