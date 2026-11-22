package app

import (
	"context"
	"errors"
	"fmt"
)

func (s *Service) bootstrapAdmin(ctx context.Context) error {
	email := s.cfg.BootstrapAdminEmail
	if email == "" {
		return nil
	}
	if !validEmail(email) {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL is not a valid email address")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var closed bool
	if err := tx.QueryRow(ctx, `select closed from admin_bootstrap_state where singleton for update`).Scan(&closed); err != nil {
		return fmt.Errorf("load administrator bootstrap state: %w", err)
	}
	if closed {
		return nil
	}
	if !validPasswordLength(s.cfg.BootstrapAdminPass) {
		return errors.New("first administrator setup requires BOOTSTRAP_ADMIN_PASSWORD (8 to 72 bytes)")
	}
	name := s.cfg.BootstrapAdminName
	if name == "" {
		name = email
	}
	hash, err := hashPassword(s.cfg.BootstrapAdminPass)
	if err != nil {
		return fmt.Errorf("hash administrator bootstrap password: %w", err)
	}
	if _, err := tx.Exec(ctx, `lock table users in share row exclusive mode`); err != nil {
		return fmt.Errorf("lock users for administrator bootstrap: %w", err)
	}
	tag, err := tx.Exec(ctx, `update users set email=$1,name=$2,enabled=true,password_hash=$3,must_change_password=true
		where id=1 and email='admin@reserved.local' and name='admin' and role='admin' and not enabled and password_hash is null
		and not exists(select 1 from users where id<>1)`, email, name, hash)
	if err != nil {
		return fmt.Errorf("initialize reserved administrator: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("administrator bootstrap requires a fresh database with only the reserved account")
	}
	if _, err := tx.Exec(ctx, `update admin_bootstrap_state set closed=true where singleton`); err != nil {
		return fmt.Errorf("close administrator bootstrap: %w", err)
	}
	return tx.Commit(ctx)
}
