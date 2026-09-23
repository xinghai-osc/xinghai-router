package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type userMutation struct {
	authorization bool
	groups        bool
	balance       bool
	identity      bool
}

type userAccess struct {
	accountContext
	enabled bool
}

var errUserMutationForbidden = errors.New("user mutation forbidden")
var errLastAdministrator = errors.New("cannot remove or disable the last enabled administrator")
var errUserMutationSession = errors.New("session changed before user mutation")
var errUserMutationReauthentication = errors.New("recent authentication required before user mutation")

func validateUserMutation(actor accountContext, target *userAccess, mutation userMutation, manage bool) error {
	if manage && !accountHasPermission(actor, "users.manage") {
		return errUserMutationForbidden
	}
	if mutation.authorization && (actor.role != "admin" || !accountHasPermission(actor, "users.authorize") || (target != nil && actor.userID == target.userID)) {
		return errUserMutationForbidden
	}
	if mutation.identity && actor.role != "admin" {
		return errUserMutationForbidden
	}
	if mutation.groups && !accountHasPermission(actor, "system.manage") {
		return errUserMutationForbidden
	}
	if mutation.balance && !accountHasPermission(actor, "wallets.manage") {
		return errUserMutationForbidden
	}
	if target != nil && actor.role != "admin" {
		if target.role != "user" {
			return errUserMutationForbidden
		}
		for _, granted := range target.permissions {
			if granted {
				return errUserMutationForbidden
			}
		}
	}
	return nil
}

func lockUserAccess(ctx context.Context, tx pgx.Tx, userID string) (userAccess, error) {
	var access userAccess
	var granted []string
	err := tx.QueryRow(ctx, `select u.id,u.role,u.enabled,coalesce((select array_agg(p.permission) from user_permissions p where p.user_id=u.id),'{}'::text[]) from users u where u.id=$1 for update`, userID).Scan(&access.userID, &access.role, &access.enabled, &granted)
	if err != nil {
		return access, err
	}
	access.permissions = make(map[string]bool, len(granted))
	for _, permission := range granted {
		access.permissions[permission] = true
	}
	return access, nil
}

func lockUserMutation(ctx context.Context, tx pgx.Tx, actorID, targetID string, mutation userMutation, manage bool) (accountContext, *userAccess, error) {
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(458111)`); err != nil {
		return accountContext{}, nil, err
	}
	actor, err := lockUserAccess(ctx, tx, actorID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !actor.enabled) {
		return accountContext{}, nil, errUserMutationForbidden
	}
	if err != nil {
		return accountContext{}, nil, err
	}
	var target *userAccess
	if targetID != "" {
		access, err := lockUserAccess(ctx, tx, targetID)
		if err != nil {
			return accountContext{}, nil, err
		}
		target = &access
	}
	if err := validateUserMutation(actor.accountContext, target, mutation, manage); err != nil {
		return accountContext{}, nil, err
	}
	if err := validateUserMutationSession(ctx, tx, actor.userID); err != nil {
		return accountContext{}, nil, err
	}
	return actor.accountContext, target, nil
}

func validateUserMutationSession(ctx context.Context, tx pgx.Tx, actorID string) error {
	requestAccount, ok := ctx.Value(accountContextKey{}).(accountContext)
	if !ok || requestAccount.sessionHash == "" {
		return nil
	}
	if requestAccount.userID != actorID {
		return errUserMutationSession
	}
	var expiresAt, checkedAt time.Time
	var reauthenticatedAt *time.Time
	var mustChangePassword bool
	err := tx.QueryRow(ctx, `select s.expires_at,s.reauthenticated_at,u.must_change_password,clock_timestamp() from user_sessions s join users u on u.id=s.user_id where s.token_hash=$1 and s.user_id=$2 and u.enabled for update of s`, requestAccount.sessionHash, actorID).Scan(&expiresAt, &reauthenticatedAt, &mustChangePassword, &checkedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return errUserMutationSession
	}
	if err != nil {
		return err
	}
	if !expiresAt.After(checkedAt) || mustChangePassword {
		return errUserMutationSession
	}
	if reauthenticatedAt == nil || reauthenticatedAt.After(checkedAt) || checkedAt.Sub(*reauthenticatedAt) >= recentAuthDuration {
		return errUserMutationReauthentication
	}
	return nil
}

func preserveAdministrator(ctx context.Context, tx pgx.Tx, target userAccess, resultingRole string, resultingEnabled bool) error {
	if target.role != "admin" || !target.enabled || (resultingRole == "admin" && resultingEnabled) {
		return nil
	}
	var others int
	if err := tx.QueryRow(ctx, `select count(*) from users where role='admin' and enabled and id<>$1`, target.userID).Scan(&others); err != nil {
		return err
	}
	if others == 0 {
		return errLastAdministrator
	}
	return nil
}

func writeUserMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errUserMutationSession):
		writeError(w, http.StatusUnauthorized, "unauthorized", "session changed before user update; sign in again")
	case errors.Is(err, errUserMutationReauthentication):
		writeError(w, http.StatusForbidden, "reauthentication_required", "verify your password again before updating users")
	case errors.Is(err, errUserMutationForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "user management cannot change this account or its authorization")
	case errors.Is(err, errLastAdministrator):
		writeError(w, http.StatusConflict, "last_administrator", errLastAdministrator.Error())
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "not_found", "user not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "could not validate user authorization")
	}
}
