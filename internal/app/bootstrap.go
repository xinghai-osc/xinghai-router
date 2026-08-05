package app

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// bootstrapAdmin promotes the account identified by BOOTSTRAP_ADMIN_EMAIL to an
// administrator on startup. It claims the reserved id-1 placeholder created by
// migration 058 on fresh installs, promotes an existing account when one already
// holds the email, and otherwise inserts a new admin. When no explicit password
// is set and a new account is created, a temporary one is generated and printed
// once to the router log; must_change_password forces a reset on first sign-in.
func (s *Service) bootstrapAdmin(ctx context.Context) error {
	email := s.cfg.BootstrapAdminEmail
	if email == "" {
		return nil
	}
	if !validEmail(email) {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL is not a valid email address")
	}

	var id int64
	existing := true
	err := s.db.QueryRow(ctx, `select id from users where email=$1`, email).Scan(&id)
	switch {
	case err == nil:
	case errors.Is(err, pgx.ErrNoRows):
		existing = false
	default:
		return fmt.Errorf("bootstrap admin lookup: %w", err)
	}

	password := s.cfg.BootstrapAdminPass
	generated := false
	if password == "" {
		if !existing {
			generated = true
			password, err = randomSecret("")
			if err != nil {
				return fmt.Errorf("bootstrap admin password: %w", err)
			}
		}
	} else if !validPasswordLength(password) {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must be between 8 and 72 characters")
	}

	passwordHash := ""
	mustChange := false
	if password != "" {
		if passwordHash, err = hashPassword(password); err != nil {
			return fmt.Errorf("bootstrap admin password hash: %w", err)
		}
		mustChange = generated
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if existing {
		if _, err = tx.Exec(ctx, `update users set role='admin',enabled=true,
			password_hash=case when $1='' then password_hash else $1 end,
			must_change_password=case when $1='' then must_change_password else $2 end
			where id=$3`, passwordHash, mustChange, id); err != nil {
			return fmt.Errorf("bootstrap admin promote: %w", err)
		}
	} else {
		name := s.cfg.BootstrapAdminName
		if name == "" {
			name = email
		}
		res, err := tx.Exec(ctx, `update users set email=$1,name=$2,role='admin',enabled=true,password_hash=$3,must_change_password=$4 where id=1 and enabled=false and password_hash is null`, email, name, passwordHash, mustChange)
		if err != nil {
			return fmt.Errorf("bootstrap admin claim: %w", err)
		}
		if res.RowsAffected() == 0 {
			if err = tx.QueryRow(ctx, `insert into users(email,name,role,password_hash,must_change_password) values($1,$2,'admin',$3,$4) returning id`, email, name, passwordHash, mustChange).Scan(&id); err != nil {
				return fmt.Errorf("bootstrap admin create: %w", err)
			}
		} else {
			id = 1
		}
	}
	if _, err = tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, id); err != nil {
		return fmt.Errorf("bootstrap admin wallet: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}

	if generated {
		log.Printf("bootstrap admin: created administrator %s (id %d)", email, id)
		log.Printf("bootstrap admin: temporary password for %s is %s (must change on first sign-in)", email, password)
	} else {
		log.Printf("bootstrap admin: %s is an administrator (id %d)", email, id)
	}
	return nil
}
