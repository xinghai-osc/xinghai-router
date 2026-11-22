//go:build integration

package app

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
)

func TestIntegrationBootstrapAdminOnce(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	s.cfg.BootstrapAdminEmail = "setup@example.test"
	if err := s.bootstrapAdmin(t.Context()); err == nil {
		t.Fatal("fresh setup accepted a missing password")
	}
	s.cfg.BootstrapAdminPass = "one-time-test-password"
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- s.bootstrapAdmin(t.Context()) }()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(logs.String(), s.cfg.BootstrapAdminPass) {
		t.Fatal("bootstrap password reached service log")
	}
	if got := rewardRiskCount(t, s, `select count(*) from users where id=1 and email=$1 and role='admin' and must_change_password and enabled`, s.cfg.BootstrapAdminEmail); got != 1 {
		t.Fatal("reserved administrator was not initialized")
	}
	rotated, err := hashPassword("rotated-test-password")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `update users set role='user',enabled=false,password_hash=$1,must_change_password=false where id=1`, rotated)
	for _, email := range []string{s.cfg.BootstrapAdminEmail, "alternate@example.test"} {
		s.cfg.BootstrapAdminEmail = email
		if err := s.bootstrapAdmin(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if got := rewardRiskCount(t, s, `select count(*) from users where id=1 and role='user' and not enabled and password_hash=$1 and not must_change_password`, rotated); got != 1 {
		t.Fatal("restart changed credentials, role or activation")
	}
	if got := rewardRiskCount(t, s, `select count(*) from users`); got != 1 {
		t.Fatal("restart created another account")
	}
}

func TestIntegrationBootstrapProtectsExistingIdentities(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	s.cfg.BootstrapAdminEmail = "existing@example.test"
	s.cfg.BootstrapAdminPass = "one-time-test-password"
	id, _ := integrationUser(t, s.db, s.cfg.BootstrapAdminEmail, 0)
	if _, _, err := s.createSessionToken(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err := s.bootstrapAdmin(t.Context()); err == nil {
		t.Fatal("bootstrap accepted a populated database")
	}
	if got := rewardRiskCount(t, s, `select count(*) from users join user_sessions on user_sessions.user_id=users.id where users.id=$1 and role='user'`, id); got != 1 {
		t.Fatal("bootstrap altered the existing session's role")
	}
	rewardRiskExec(t, s, `truncate admin_bootstrap_state`)
	migration, err := migrations.ReadFile("migrations/146_admin_bootstrap_once.sql")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, string(migration))
	if err := s.bootstrapAdmin(t.Context()); err != nil {
		t.Fatalf("legacy deployment with leftover settings cannot start: %v", err)
	}
	if got := rewardRiskCount(t, s, `select count(*) from admin_bootstrap_state where closed`); got != 1 {
		t.Fatal("migration did not close bootstrap for existing accounts")
	}
}

func TestIntegrationResetCredentialVersion(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	const email = "version@example.test"
	id, _ := integrationUser(t, s.db, email, 0)
	checkReset := func(token string, status int) {
		t.Helper()
		body := fmt.Sprintf(`{"token":%q,"password":"final-test-password"}`, token)
		rewardRiskJSON(t, authSecurityRequest(s, "POST", "/auth/password-reset/confirm", body, nil), status)
	}
	issue := func() string {
		t.Helper()
		bound, token, err := s.issuePasswordResetToken(t.Context(), email)
		if err != nil || bound != id {
			t.Fatalf("issue token for expected user: %v", err)
		}
		return token
	}
	stale := issue()
	hash, err := hashPassword("rotated-test-password")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, `update users set password_hash=$1 where id=$2`, hash, id)
	if got := rewardRiskCount(t, s, `select count(*) from password_reset_tokens where token_hash=$1 and consumed_at is not null`, hashSecret(stale)); got != 1 {
		t.Fatal("password change left an active token")
	}
	rewardRiskExec(t, s, `update password_reset_tokens set consumed_at=null where token_hash=$1`, hashSecret(stale))
	checkReset(stale, http.StatusBadRequest)
	current := issue()
	rewardRiskExec(t, s, `update users set email='moved@example.test' where id=$1`, id)
	other, _ := integrationUser(t, s.db, "new-owner@example.test", 0)
	rewardRiskExec(t, s, `update users set email=$1 where id=$2`, email, other)
	checkReset(current, http.StatusBadRequest)
	if got := rewardRiskCount(t, s, `select count(*) from users where id=$1 and email_verified_at is null`, other); got != 1 {
		t.Fatal("old token affected the new email owner")
	}
	_, fresh, err := s.issuePasswordResetToken(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	checkReset(fresh, http.StatusOK)
	checkReset(fresh, http.StatusBadRequest)
}

func TestIntegrationResetMigrationInvalidatesUnboundLinks(t *testing.T) {
	s := integrationService(t, rewardRiskIntegrationPool(t))
	const email = "old-reset@example.test"
	integrationUser(t, s.db, email, 0)
	rewardRiskExec(t, s, `alter table password_reset_tokens alter column user_id drop not null, alter column credential_version drop not null`)
	rewardRiskExec(t, s, `insert into password_reset_tokens(email,token_hash,expires_at) values($1,$2,now()+interval '30 minutes')`, email, hashSecret("fake-legacy-reset"))
	migration, err := migrations.ReadFile("migrations/147_bind_password_reset_credentials.sql")
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, string(migration))
	if got := rewardRiskCount(t, s, `select count(*) from password_reset_tokens where token_hash=$1`, hashSecret("fake-legacy-reset")); got != 0 {
		t.Fatal("legacy reset token survived migration")
	}
	_, token, err := s.issuePasswordResetToken(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	rewardRiskExec(t, s, string(migration))
	if got := rewardRiskCount(t, s, `select count(*) from password_reset_tokens where token_hash=$1 and consumed_at is null`, hashSecret(token)); got != 1 {
		t.Fatal("migration replay revoked a newly bound token")
	}
}
