package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	migrationAdvisoryLockKey int64 = 739481200174
	migrationDDLTimeout            = 5 * time.Second
)

func migrate(ctx context.Context, db *pgxpool.Pool) error {
	conn, err := pgx.ConnectConfig(ctx, db.Config().ConnConfig.Copy())
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), migrationDDLTimeout)
		defer cancel()
		_ = conn.Close(cleanupCtx)
	}()
	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1::bigint)`, migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if _, err := conn.Exec(ctx, `set lock_timeout = '5s'`); err != nil {
		return fmt.Errorf("set migration lock timeout: %w", err)
	}
	if _, err := conn.Exec(ctx, `create table if not exists schema_migrations (name text primary key, applied_at timestamptz not null default now())`); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		var exists bool
		if err := conn.QueryRow(ctx, `select exists(select 1 from schema_migrations where name=$1)`, e.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err == nil {
			_, err = tx.Exec(ctx, `insert into schema_migrations(name) values($1)`, e.Name())
		}
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.Background(), migrationDDLTimeout)
			_ = tx.Rollback(rollbackCtx)
			cancel()
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
