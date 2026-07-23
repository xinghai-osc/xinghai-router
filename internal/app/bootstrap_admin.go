package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
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

type bootstrapConflictDecision string

const (
	bootstrapConflictRefusePromote bootstrapConflictDecision = "refuse_promote"
	bootstrapConflictAlreadyAdmin  bootstrapConflictDecision = "already_admin"
)

func bootstrapConflictPolicy(existingRole string) bootstrapConflictDecision {
	if existingRole == "admin" {
		return bootstrapConflictAlreadyAdmin
	}
	return bootstrapConflictRefusePromote
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
	err = tx.QueryRow(ctx, `insert into users(email,name,role,password_hash,must_change_password) values($1,$2,'admin',$3,true)
		on conflict (email) do nothing
		returning id`, email, name, passwordHash).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var existingRole string
			if scanErr := tx.QueryRow(ctx, `select role from users where email=$1`, email).Scan(&existingRole); scanErr != nil {
				return fmt.Errorf("bootstrap admin conflict lookup: %w", scanErr)
			}
			switch bootstrapConflictPolicy(existingRole) {
			case bootstrapConflictRefusePromote:
				log.Printf("bootstrap admin skipped: email=%s already exists as role=%s (refusing privilege escalation)", email, existingRole)
			case bootstrapConflictAlreadyAdmin:
				log.Printf("bootstrap admin skipped: email=%s already exists as admin (not overwriting credentials)", email)
			default:
				log.Printf("bootstrap admin skipped: email=%s already exists as role=%s", email, existingRole)
			}
			return nil
		}
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
