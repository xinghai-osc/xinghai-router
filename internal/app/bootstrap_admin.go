package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultBootstrapAdminEmail = "admin@localhost"
	defaultBootstrapAdminName  = "Administrator"
	bootstrapAdminPasswordLen  = 24
)

func bootstrapAdminEmail() string {
	if v := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")); v != "" {
		return strings.ToLower(v)
	}
	return defaultBootstrapAdminEmail
}

func bootstrapAdminName() string {
	if v := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_NAME")); v != "" {
		return v
	}
	return defaultBootstrapAdminName
}

func randomPassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*-_"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

func ensureBootstrapAdmin(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap admin begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(458111)`); err != nil {
		return fmt.Errorf("bootstrap admin lock: %w", err)
	}

	var hasAccountAdmin bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from users where role='admin' and password_hash is not null)`).Scan(&hasAccountAdmin); err != nil {
		return fmt.Errorf("bootstrap admin check: %w", err)
	}
	if hasAccountAdmin {
		return nil
	}

	email := bootstrapAdminEmail()
	name := bootstrapAdminName()
	password, err := randomPassword(bootstrapAdminPasswordLen)
	if err != nil {
		return fmt.Errorf("bootstrap admin password: %w", err)
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("bootstrap admin hash: %w", err)
	}

	var id string
	err = tx.QueryRow(ctx, `insert into users(email,name,role,password_hash) values($1,$2,'admin',$3)
		on conflict (email) do update set role='admin', password_hash=excluded.password_hash, name=excluded.name, enabled=true
		returning id`, email, name, passwordHash).Scan(&id)
	if err != nil {
		return fmt.Errorf("bootstrap admin insert: %w", err)
	}
	if _, err = tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, id); err != nil {
		return fmt.Errorf("bootstrap admin wallet: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("bootstrap admin commit: %w", err)
	}

	log.Printf("bootstrap admin created: email=%s password=%s", email, password)
	log.Printf("IMPORTANT: change the bootstrap admin password immediately after first login")
	return nil
}
