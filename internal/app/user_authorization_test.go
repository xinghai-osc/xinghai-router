package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUserMutationBoundaries(t *testing.T) {
	admin := accountContext{userID: "1", role: "admin"}
	manager := accountContext{userID: "2", role: "operator", permissions: map[string]bool{"users.manage": true}}
	grantedAuthorizer := accountContext{userID: "2", role: "operator", permissions: map[string]bool{"users.manage": true, "users.authorize": true, "system.manage": true}}
	ordinary := &userAccess{accountContext: accountContext{userID: "3", role: "user"}, enabled: true}
	privileged := &userAccess{accountContext: accountContext{userID: "3", role: "user", permissions: map[string]bool{"logs.read": true}}, enabled: true}
	for _, tc := range []struct {
		name     string
		actor    accountContext
		target   *userAccess
		mutation userMutation
		manage   bool
		allowed  bool
	}{
		{"manage ordinary profile", manager, ordinary, userMutation{}, true, true},
		{"manage cannot authorize", manager, ordinary, userMutation{authorization: true}, true, false},
		{"explicit authorizer still cannot delegate", grantedAuthorizer, ordinary, userMutation{authorization: true}, false, false},
		{"ordinary user cannot manage", accountContext{role: "user"}, ordinary, userMutation{}, true, false},
		{"manager cannot edit privileged profile", manager, privileged, userMutation{}, true, false},
		{"manager cannot edit administrator", manager, &userAccess{accountContext: admin}, userMutation{}, true, false},
		{"manager cannot edit operator", manager, &userAccess{accountContext: accountContext{userID: "4", role: "operator"}}, userMutation{}, true, false},
		{"manager cannot edit own privileged profile", manager, &userAccess{accountContext: manager}, userMutation{}, true, false},
		{"manager cannot change ID", manager, ordinary, userMutation{identity: true}, true, false},
		{"manager cannot change balance", manager, ordinary, userMutation{balance: true}, true, false},
		{"manager cannot change groups", manager, ordinary, userMutation{groups: true}, true, false},
		{"admin may edit other administrator", admin, &userAccess{accountContext: accountContext{userID: "4", role: "admin"}}, userMutation{authorization: true}, true, true},
		{"admin cannot self authorize", admin, &userAccess{accountContext: admin}, userMutation{authorization: true}, true, false},
		{"admin may edit own profile", admin, &userAccess{accountContext: admin}, userMutation{}, true, true},
		{"manager may create unprivileged user", manager, nil, userMutation{}, true, true},
		{"manager cannot create privileged user", manager, nil, userMutation{authorization: true}, true, false},
		{"admin may create privileged user", admin, nil, userMutation{authorization: true}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUserMutation(tc.actor, tc.target, tc.mutation, tc.manage)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed = %v, want %v; err=%v", err == nil, tc.allowed, err)
			}
		})
	}
	if !availablePermissions["users.authorize"] || accountHasPermission(manager, "users.authorize") {
		t.Fatal("authorize must be a separate permission without legacy promotion")
	}
}

func TestUserMutationHandlersRejectPrivilegeEscalationBeforeDatabase(t *testing.T) {
	service := &Service{}
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		body    string
		actor   accountContext
		target  string
	}{
		{"create administrator", service.createUser, `{"email":"fake@example.test","name":"fake","password":"fake-test-password","role":"admin"}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.manage": true}}, ""},
		{"create system manager", service.createUser, `{"email":"fake@example.test","name":"fake","password":"fake-test-password","permissions":["system.manage"]}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.manage": true}}, ""},
		{"profile role escalation", service.updateUser, `{"role":"admin"}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.manage": true}}, "2"},
		{"profile permission escalation", service.updateUser, `{"permissions":["users.authorize"]}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.manage": true}}, "2"},
		{"self role even unchanged", service.updateUser, `{"role":"admin"}`, accountContext{userID: "1", role: "admin"}, "1"},
		{"self permissions", service.updateUser, `{"permissions":[]}`, accountContext{userID: "1", role: "admin"}, "1"},
		{"dedicated self role", service.setUserRole, `{"role":"admin"}`, accountContext{userID: "1", role: "admin"}, "1"},
		{"dedicated self permissions", service.setUserPermissions, `{"permissions":[]}`, accountContext{userID: "1", role: "admin"}, "1"},
		{"dedicated granted authorizer role", service.setUserRole, `{"role":"admin"}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.authorize": true}}, "2"},
		{"dedicated granted authorizer permissions", service.setUserPermissions, `{"permissions":["system.manage"]}`, accountContext{userID: "1", role: "operator", permissions: map[string]bool{"users.authorize": true}}, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/admin/users/"+tc.target, strings.NewReader(tc.body))
			r.SetPathValue("id", tc.target)
			r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, tc.actor))
			w := httptest.NewRecorder()
			tc.handler(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

type userAuthorizationTestTx struct {
	pgx.Tx
	users   map[string]userAccess
	calls   []string
	others  int
	lockErr error
	session *userAuthorizationTestSession
}

type userAuthorizationTestSession struct {
	expiresAt          time.Time
	reauthenticatedAt  *time.Time
	mustChangePassword bool
	checkedAt          time.Time
}

func (tx *userAuthorizationTestTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	tx.calls = append(tx.calls, sql)
	if sql != `select pg_advisory_xact_lock(458111)` {
		return pgconn.CommandTag{}, fmt.Errorf("unexpected SQL %q", sql)
	}
	return pgconn.CommandTag{}, tx.lockErr
}

type userAuthorizationTestRow func(...any) error

func (r userAuthorizationTestRow) Scan(dest ...any) error { return r(dest...) }

func (tx *userAuthorizationTestTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	tx.calls = append(tx.calls, sql)
	return userAuthorizationTestRow(func(dest ...any) error {
		if strings.HasPrefix(sql, "select count(*)") {
			*dest[0].(*int) = tx.others
			return nil
		}
		if strings.Contains(sql, "from user_sessions") {
			if !strings.HasSuffix(sql, "for update of s") || !strings.Contains(sql, "clock_timestamp()") {
				return fmt.Errorf("session must be locked and checked against wall clock: %s", sql)
			}
			if tx.session == nil {
				return pgx.ErrNoRows
			}
			*dest[0].(*time.Time) = tx.session.expiresAt
			*dest[1].(**time.Time) = tx.session.reauthenticatedAt
			*dest[2].(*bool) = tx.session.mustChangePassword
			*dest[3].(*time.Time) = tx.session.checkedAt
			return nil
		}
		if !strings.HasSuffix(sql, "for update") {
			return fmt.Errorf("user lookup is not locked: %s", sql)
		}
		user, ok := tx.users[args[0].(string)]
		if !ok {
			return pgx.ErrNoRows
		}
		*dest[0].(*string) = user.userID
		*dest[1].(*string) = user.role
		*dest[2].(*bool) = user.enabled
		var permissions []string
		for permission, granted := range user.permissions {
			if granted {
				permissions = append(permissions, permission)
			}
		}
		*dest[3].(*[]string) = permissions
		return nil
	})
}

func TestUserMutationLocksAndReloadsAuthorization(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		actor   userAccess
		target  userAccess
		allowed bool
	}{
		{"admin", userAccess{accountContext: accountContext{userID: "1", role: "admin"}, enabled: true}, userAccess{accountContext: accountContext{userID: "2", role: "user"}, enabled: true}, true},
		{"actor demoted", userAccess{accountContext: accountContext{userID: "1", role: "user"}, enabled: true}, userAccess{accountContext: accountContext{userID: "2", role: "user"}, enabled: true}, false},
		{"actor disabled", userAccess{accountContext: accountContext{userID: "1", role: "admin"}}, userAccess{accountContext: accountContext{userID: "2", role: "user"}, enabled: true}, false},
		{"target promoted", userAccess{accountContext: accountContext{userID: "1", role: "user", permissions: map[string]bool{"users.manage": true}}, enabled: true}, userAccess{accountContext: accountContext{userID: "2", role: "admin"}, enabled: true}, false},
		{"target explicitly granted", userAccess{accountContext: accountContext{userID: "1", role: "user", permissions: map[string]bool{"users.manage": true}}, enabled: true}, userAccess{accountContext: accountContext{userID: "2", role: "user", permissions: map[string]bool{"system.manage": true}}, enabled: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &userAuthorizationTestTx{users: map[string]userAccess{"1": tc.actor, "2": tc.target}}
			_, _, err := lockUserMutation(ctx, tx, "1", "2", userMutation{}, true)
			if (err == nil) != tc.allowed {
				t.Fatalf("err=%v, allowed=%v", err, tc.allowed)
			}
			if len(tx.calls) < 2 || tx.calls[0] != `select pg_advisory_xact_lock(458111)` {
				t.Fatalf("authorization must be reloaded after serialization lock: %v", tx.calls)
			}
		})
	}
	tx := &userAuthorizationTestTx{users: map[string]userAccess{"1": {accountContext: accountContext{userID: "1", role: "admin"}, enabled: true}, "01": {accountContext: accountContext{userID: "1", role: "admin"}, enabled: true}}}
	if _, _, err := lockUserMutation(ctx, tx, "1", "01", userMutation{authorization: true}, false); !errors.Is(err, errUserMutationForbidden) {
		t.Fatalf("numeric ID aliases must not bypass self authorization: %v", err)
	}
	tx = &userAuthorizationTestTx{lockErr: errors.New("lock failed")}
	if _, _, err := lockUserMutation(ctx, tx, "1", "2", userMutation{}, true); err == nil || len(tx.calls) != 1 {
		t.Fatalf("lock failure must fail closed, err=%v calls=%v", err, tx.calls)
	}
}

func TestUserMutationRechecksSessionAfterLocks(t *testing.T) {
	now := time.Now()
	recent := now.Add(-time.Minute)
	stale := now.Add(-recentAuthDuration)
	future := now.Add(time.Minute)
	for _, tc := range []struct {
		name    string
		session *userAuthorizationTestSession
		want    error
	}{
		{"valid", &userAuthorizationTestSession{expiresAt: future, reauthenticatedAt: &recent, checkedAt: now}, nil},
		{"revoked", nil, errUserMutationSession},
		{"expired", &userAuthorizationTestSession{expiresAt: now, reauthenticatedAt: &recent, checkedAt: now}, errUserMutationSession},
		{"must change password", &userAuthorizationTestSession{expiresAt: future, reauthenticatedAt: &recent, mustChangePassword: true, checkedAt: now}, errUserMutationSession},
		{"reauth expired", &userAuthorizationTestSession{expiresAt: future, reauthenticatedAt: &stale, checkedAt: now}, errUserMutationReauthentication},
		{"reauth absent", &userAuthorizationTestSession{expiresAt: future, checkedAt: now}, errUserMutationReauthentication},
		{"reauth future", &userAuthorizationTestSession{expiresAt: future, reauthenticatedAt: &future, checkedAt: now}, errUserMutationReauthentication},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := accountContext{userID: "1", role: "admin", sessionHash: "fake-session-hash", reauthenticatedAt: &recent}
			ctx := context.WithValue(context.Background(), accountContextKey{}, actor)
			tx := &userAuthorizationTestTx{users: map[string]userAccess{"1": {accountContext: actor, enabled: true}, "2": {accountContext: accountContext{userID: "2", role: "user"}, enabled: true}}, session: tc.session}
			_, _, err := lockUserMutation(ctx, tx, "1", "2", userMutation{authorization: true}, true)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
			if len(tx.calls) != 4 || !strings.Contains(tx.calls[3], "from user_sessions") {
				t.Fatalf("session validation must follow all user locks: %v", tx.calls)
			}
		})
	}
}

func TestPreserveLastEnabledAdministrator(t *testing.T) {
	target := userAccess{accountContext: accountContext{userID: "1", role: "admin"}, enabled: true}
	for _, tc := range []struct {
		role    string
		enabled bool
		others  int
		blocked bool
	}{
		{"user", true, 0, true},
		{"admin", false, 0, true},
		{"operator", false, 0, true},
		{"admin", true, 0, false},
		{"user", true, 1, false},
		{"admin", false, 1, false},
	} {
		tx := &userAuthorizationTestTx{others: tc.others}
		err := preserveAdministrator(context.Background(), tx, target, tc.role, tc.enabled)
		if errors.Is(err, errLastAdministrator) != tc.blocked {
			t.Fatalf("role=%s enabled=%v others=%d err=%v", tc.role, tc.enabled, tc.others, err)
		}
	}
}
