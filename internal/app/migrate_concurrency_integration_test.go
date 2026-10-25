package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func migrationIntegrationPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; use an isolated PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("new PostgreSQL pool: %v", err)
	}
	id, err := randomID()
	if err != nil {
		admin.Close()
		t.Fatalf("random schema id: %v", err)
	}
	schema := "migration_test_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+quoted); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.ConnConfig.RuntimeParams["lock_timeout"] = "0"
	first, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		admin.Close()
		t.Fatalf("new first migration pool: %v", err)
	}
	second, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		first.Close()
		admin.Close()
		t.Fatalf("new second migration pool: %v", err)
	}
	t.Cleanup(func() {
		first.Close()
		second.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "drop schema "+quoted+" cascade"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		admin.Close()
	})
	return first, second
}

func TestIntegrationMigrateConcurrentStartup(t *testing.T) {
	first, second := migrationIntegrationPools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, db := range []*pgxpool.Pool{first, second} {
		wg.Add(1)
		go func(db *pgxpool.Pool) {
			defer wg.Done()
			<-start
			errs <- migrate(ctx, db)
		}(db)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate: %v", err)
		}
	}
	var applied int
	if err := first.QueryRow(ctx, `select count(*) from schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	expected := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			expected++
		}
	}
	if applied != expected {
		t.Fatalf("applied migrations=%d, want %d", applied, expected)
	}
	for _, db := range []*pgxpool.Pool{first, second} {
		var timeout string
		if err := db.QueryRow(ctx, `show lock_timeout`).Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if timeout != "0" {
			t.Fatalf("migration leaked lock_timeout=%s into serving pool", timeout)
		}
	}
}

func TestIntegrationMigrateDDLWaitAndRetry(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "lock_timeout"
		if canceled {
			name = "canceled_owner"
		}
		t.Run(name, func(t *testing.T) {
			first, second := migrationIntegrationPools(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			initial, err := migrations.ReadFile("migrations/001_initial.sql")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := first.Exec(ctx, string(initial)); err != nil {
				t.Fatal(err)
			}
			if _, err := first.Exec(ctx, `create table schema_migrations (name text primary key, applied_at timestamptz not null default now()); insert into schema_migrations(name) values ('001_initial.sql')`); err != nil {
				t.Fatal(err)
			}
			holder, err := first.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if _, err := holder.Exec(ctx, `lock table users in row exclusive mode`); err != nil {
				t.Fatal(err)
			}
			waitCtx := ctx
			if canceled {
				var stop context.CancelFunc
				waitCtx, stop = context.WithTimeout(ctx, 250*time.Millisecond)
				defer stop()
			}
			started := time.Now()
			err = migrate(waitCtx, second)
			if err == nil || !strings.Contains(err.Error(), "migration 002_phase2.sql:") {
				t.Fatalf("expected failure within second migration, got %v", err)
			}
			if canceled {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("canceled migration error=%v", err)
				}
			} else {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
					t.Fatalf("expected DDL lock timeout, got %v", err)
				}
				if elapsed := time.Since(started); elapsed > 2*migrationDDLTimeout {
					t.Fatalf("DDL wait took %s", elapsed)
				}
			}
			var applied int
			if err := first.QueryRow(ctx, `select count(*) from schema_migrations`).Scan(&applied); err != nil || applied != 1 {
				t.Fatalf("failed migration recorded: count=%d error=%v", applied, err)
			}
			var weightExists bool
			if err := first.QueryRow(ctx, `select exists(select 1 from information_schema.columns where table_schema=current_schema() and table_name='channels' and column_name='weight')`).Scan(&weightExists); err != nil || weightExists {
				t.Fatalf("failed migration changed schema: weight=%t error=%v", weightExists, err)
			}
			if err := holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if err := migrate(ctx, first); err != nil {
				t.Fatalf("retry after failed migration: %v", err)
			}
		})
	}
}

func TestIntegrationMigrateCancellationAndRetry(t *testing.T) {
	first, second := migrationIntegrationPools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	holder, err := first.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire lock holder: %v", err)
	}
	defer holder.Release()
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = holder.Conn().Close(cleanupCtx)
	}()
	if _, err := holder.Exec(ctx, `select pg_advisory_lock($1::bigint)`, migrationAdvisoryLockKey); err != nil {
		t.Fatalf("hold migration lock: %v", err)
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	err = migrate(waitCtx, second)
	waitCancel()
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
		t.Fatalf("canceled migrate error=%v", err)
	}
	var metadataCreated bool
	if err := first.QueryRow(ctx, `select to_regclass('schema_migrations') is not null`).Scan(&metadataCreated); err != nil || metadataCreated {
		t.Fatalf("schema_migrations created before acquiring advisory lock: created=%t error=%v", metadataCreated, err)
	}
	if _, err := holder.Exec(ctx, `select pg_advisory_unlock($1::bigint)`, migrationAdvisoryLockKey); err != nil {
		t.Fatalf("release migration lock: %v", err)
	}
	if err := migrate(ctx, second); err != nil {
		t.Fatalf("retry migrate: %v", err)
	}
}
