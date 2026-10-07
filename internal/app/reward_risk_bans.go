package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func disableUserAccessTx(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx, `update users set enabled=false where id=$1`, userID); err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from user_sessions where user_id=$1`, userID); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `update api_keys set revoked_at=coalesce(revoked_at, now()) where user_id=$1 and revoked_at is null`, userID); err != nil {
		return fmt.Errorf("revoke user API keys: %w", err)
	}
	return nil
}

func (s *Service) autoBanRewardUserTx(ctx context.Context, tx pgx.Tx, userID, observationID string, ruleVersion int, details any) (bool, error) {
	target, err := lockUserAccess(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	if !target.enabled || target.role != "user" || len(target.permissions) != 0 {
		return false, nil
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return false, fmt.Errorf("encode reward ban evidence: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `insert into risk_bans(id,user_id,observation_id,rule_version,details) values($1,$2,$3,$4,$5) on conflict (user_id) where released_at is null do nothing`, id, target.userID, observationID, ruleVersion, raw); err != nil {
		return false, fmt.Errorf("record reward ban: %w", err)
	}
	if err = disableUserAccessTx(ctx, tx, target.userID); err != nil {
		return false, err
	}
	return true, nil
}

func shouldReleaseRewardBan(currentEnabled, requestedEnabled bool) bool {
	return !currentEnabled && requestedEnabled
}

func (s *Service) releaseRewardBanTx(ctx context.Context, tx pgx.Tx, userID, actorID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return errors.New("reward ban release reason must be between 1 and 500 bytes")
	}
	_, err := tx.Exec(ctx, `with released as (
		update risk_bans set released_at=clock_timestamp(),released_by=$2,release_reason=$3
		where user_id=$1 and released_at is null returning released_at
	)
	update users set risk_reviewed_through=greatest(risk_reviewed_through,(select max(released_at) from released))
	where id=$1 and exists(select 1 from released)`, userID, actorID, reason)
	if err != nil {
		return fmt.Errorf("release reward ban: %w", err)
	}
	return nil
}

func (s *Service) unbanRewardUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CaseID string `json:"case_id"`
		Reason string `json:"reason"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "case_id and reason are required")
		return
	}
	in.CaseID = strings.TrimSpace(in.CaseID)
	in.Reason = strings.TrimSpace(in.Reason)
	var caseID pgtype.UUID
	userID := strings.TrimSpace(r.PathValue("id"))
	userNumber, userErr := strconv.ParseInt(userID, 10, 64)
	if caseID.Scan(in.CaseID) != nil || !caseID.Valid || in.Reason == "" || len(in.Reason) > 500 || userErr != nil || userNumber <= 0 || userNumber > maxEditableUserID {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid user, case_id, and reason of at most 500 bytes are required")
		return
	}
	actor := accountFromContext(r)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not unblock user")
		return
	}
	defer tx.Rollback(r.Context())
	actor, target, err := lockUserMutation(r.Context(), tx, actor.userID, userID, userMutation{}, true)
	if err != nil {
		writeUserMutationError(w, err)
		return
	}
	var activeCaseID string
	err = tx.QueryRow(r.Context(), `select id::text from risk_bans where user_id=$1 and id=$2 and released_at is null for update`, target.userID, caseID).Scan(&activeCaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "ban_changed", "the active ban changed; refresh before unblocking")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load active ban")
		return
	}
	if err = s.releaseRewardBanTx(r.Context(), tx, target.userID, actor.userID, in.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not release reward ban")
		return
	}
	if _, err = tx.Exec(r.Context(), `update users set enabled=true where id=$1`, target.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not enable user")
		return
	}
	raw, err := json.Marshal(map[string]any{"case_id": activeCaseID, "reason": in.Reason})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not record unblock")
		return
	}
	auditID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not record unblock")
		return
	}
	meta := requestMetadata(r)
	if _, err = tx.Exec(r.Context(), `insert into audit_logs(id,action,actor,entity_type,entity_id,details,client_ip,forwarded_for,user_agent,browser,browser_version,operating_system,operating_system_version,device_type,is_bot,request_method,request_path,request_id) values($1,'reward_risk.unbanned',$2,'user',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, auditID, actor.userID, target.userID, raw, meta.clientIP, meta.forwardedFor, meta.userAgent, meta.browser, meta.browserVersion, meta.operatingSystem, meta.operatingSystemVersion, meta.deviceType, meta.isBot, r.Method, r.URL.Path, requestID(r.Context())); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not record unblock")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not unblock user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user_id": target.userID, "case_id": activeCaseID, "enabled": true})
}

func lockAPIKeyCreation(ctx context.Context, tx pgx.Tx, requestAccount accountContext, userID string, managed bool) error {
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(458111)`); err != nil {
		return err
	}
	actor, err := lockUserAccess(ctx, tx, requestAccount.userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !actor.enabled) {
		return errUserMutationSession
	}
	if err != nil {
		return err
	}
	if managed && !accountHasPermission(actor.accountContext, "keys.manage") {
		return errUserMutationForbidden
	}
	target := actor
	if userID != actor.userID {
		target, err = lockUserAccess(ctx, tx, userID)
		if err != nil {
			return err
		}
	}
	if !target.enabled || (!managed && target.userID != actor.userID) {
		return errUserMutationForbidden
	}
	if requestAccount.sessionHash == "" {
		return errUserMutationSession
	}
	var expiresAt, checkedAt time.Time
	var mustChangePassword bool
	err = tx.QueryRow(ctx, `select s.expires_at,u.must_change_password,clock_timestamp() from user_sessions s join users u on u.id=s.user_id where s.token_hash=$1 and s.user_id=$2 and u.enabled for update of s`, requestAccount.sessionHash, actor.userID).Scan(&expiresAt, &mustChangePassword, &checkedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return errUserMutationSession
	}
	if err != nil {
		return err
	}
	if !expiresAt.After(checkedAt) || mustChangePassword {
		return errUserMutationSession
	}
	return nil
}
