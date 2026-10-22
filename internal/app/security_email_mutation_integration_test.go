//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestIntegrationAdminEmailChangeInvalidatesVerification(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	s := integrationService(t, db)
	ctx := context.Background()
	admin, _ := integrationUser(t, db, "email-invariant-admin@example.test", 0)
	user, _ := integrationUser(t, db, "original-owner@example.test", 0)
	if _, err := db.Exec(ctx, `update users set role='admin' where id=$1`, admin); err != nil {
		t.Fatal(err)
	}
	var originalVerification time.Time
	if err := db.QueryRow(ctx, `update users set email_verified_at=now() where id=$1 returning email_verified_at`, user).Scan(&originalVerification); err != nil {
		t.Fatal(err)
	}
	actor := accountContext{userID: admin, role: "admin"}
	update := func(email string) {
		t.Helper()
		w := userAuthorizationRequest(s.updateUser, actor, user, fmt.Sprintf(`{"email":%q}`, email))
		if w.Code != http.StatusOK {
			t.Fatalf("email update status=%d body=%s", w.Code, w.Body.String())
		}
	}
	update(" ORIGINAL-OWNER@EXAMPLE.TEST ")
	var verifiedAt *time.Time
	if err := db.QueryRow(ctx, `select email_verified_at from users where id=$1`, user).Scan(&verifiedAt); err != nil {
		t.Fatal(err)
	}
	if verifiedAt == nil || !verifiedAt.Equal(originalVerification) {
		t.Fatal("unchanged normalized address lost verification")
	}
	update("new-owner@example.test")
	if err := db.QueryRow(ctx, `select email_verified_at from users where id=$1`, user).Scan(&verifiedAt); err != nil {
		t.Fatal(err)
	}
	if verifiedAt != nil {
		t.Fatal("new email inherited proof of the old address")
	}
	if _, err := s.findOrCreateOAuthUser(ctx, "github", "new-address-owner", "new-owner@example.test", true, "New owner", ""); !errors.Is(err, errOAuthEmailVerificationRequired) {
		t.Fatalf("changed email permitted OAuth merge: %v", err)
	}
	var connections int
	if err := db.QueryRow(ctx, `select count(*) from user_oauth_connections where user_id=$1`, user).Scan(&connections); err != nil || connections != 0 {
		t.Fatalf("refused merge created connections=%d error=%v", connections, err)
	}
}
