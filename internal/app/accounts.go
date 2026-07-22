package app

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"
)

type accountContext struct {
	userID      string
	role        string
	permissions map[string]bool
}
type accountContextKey struct{}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Code     string `json:"code"`
		geetestPayload
	}
	if decode(r, &in) != nil || !validAccountInput(in.Email, in.Name, in.Password) {
		writeError(w, http.StatusBadRequest, "invalid_request", "a valid email, name, and password of at least 8 characters are required")
		return
	}
	if s.loadSystemConfig(r.Context()).emailVerificationEnabled() {
		if strings.TrimSpace(in.Code) == "" {
			writeError(w, http.StatusBadRequest, "code_required", "the email verification code is required")
			return
		}
		if err := s.verifyEmailCode(r.Context(), in.Email, strings.TrimSpace(in.Code)); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
			return
		}
	} else if err := s.verifyGeetest(r.Context(), in.geetestPayload); err != nil {
		writeError(w, http.StatusForbidden, "captcha_failed", err.Error())
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	passwordHash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not secure password")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `select pg_advisory_xact_lock(458110)`); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	var hasAccountAdmin bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where role='admin' and password_hash is not null)`).Scan(&hasAccountAdmin); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	role := "user"
	if !hasAccountAdmin {
		role = "admin"
	}
	var id string
	err = tx.QueryRow(r.Context(), `insert into users(email,name,role,password_hash) values($1,$2,$3,$4) returning id`, email, strings.TrimSpace(in.Name), role, passwordHash).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "email already exists")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict do nothing`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	s.auditActor(r, id, "account.registered", "user", id, nil)
	s.createSession(w, r, id, http.StatusCreated)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		geetestPayload
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Email) == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}
	if err := s.verifyGeetest(r.Context(), in.geetestPayload); err != nil {
		writeError(w, http.StatusForbidden, "captcha_failed", err.Error())
		return
	}
	var userID, passwordHash string
	err := s.db.QueryRow(r.Context(), `select id,password_hash from users where email=$1 and enabled and password_hash is not null`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(&userID, &passwordHash)
	if err != nil || !passwordMatches(passwordHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	s.auditActor(r, userID, "account.logged_in", "user", userID, nil)
	s.createSession(w, r, userID, http.StatusOK)
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	_, _ = s.db.Exec(r.Context(), `delete from user_sessions where token_hash=$1`, hashSecret(bearer(r)))
	s.auditActor(r, account.userID, "account.logged_out", "user", account.userID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) accountMe(w http.ResponseWriter, r *http.Request) {
	account := r.Context().Value(accountContextKey{}).(accountContext)
	var email, name, role, avatarURL string
	var leaderboardOptIn, leaderboardMaskName bool
	var balance, reserved any
	err := s.db.QueryRow(r.Context(), `select u.email,u.name,u.role,u.avatar_url,u.leaderboard_opt_in,u.leaderboard_mask_name,coalesce(w.balance,0),coalesce(w.reserved,0) from users u left join user_wallets w on w.user_id=u.id where u.id=$1`, account.userID).Scan(&email, &name, &role, &avatarURL, &leaderboardOptIn, &leaderboardMaskName, &balance, &reserved)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load account")
		return
	}
	permissions := make([]string, 0, len(account.permissions))
	for permission := range account.permissions {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	writeJSON(w, http.StatusOK, map[string]any{"id": account.userID, "email": email, "name": name, "role": role, "avatar_url": avatarURL, "permissions": permissions, "balance": balance, "reserved": reserved, "leaderboard_opt_in": leaderboardOptIn, "leaderboard_mask_name": leaderboardMaskName})
}

func (s *Service) updateAccountPreferences(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		LeaderboardOptIn    *bool `json:"leaderboard_opt_in"`
		LeaderboardMaskName *bool `json:"leaderboard_mask_name"`
	}
	if decode(r, &in) != nil || (in.LeaderboardOptIn == nil && in.LeaderboardMaskName == nil) {
		writeError(w, http.StatusBadRequest, "invalid_request", "nothing to update")
		return
	}
	if in.LeaderboardOptIn != nil {
		if _, err := s.db.Exec(r.Context(), `update users set leaderboard_opt_in=$1 where id=$2`, *in.LeaderboardOptIn, account.userID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not save preferences")
			return
		}
	}
	if in.LeaderboardMaskName != nil {
		if _, err := s.db.Exec(r.Context(), `update users set leaderboard_mask_name=$1 where id=$2`, *in.LeaderboardMaskName, account.userID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not save preferences")
			return
		}
	}
	s.audit(r, "account.preferences_updated", "user", account.userID, map[string]any{"leaderboard_opt_in": in.LeaderboardOptIn, "leaderboard_mask_name": in.LeaderboardMaskName})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func validatePasswordChange(currentPassword, newPassword string) string {
	if currentPassword == "" || newPassword == "" {
		return "current_password and new_password are required"
	}
	if !validPasswordLength(newPassword) {
		return "new password must be between 8 and 72 characters"
	}
	if currentPassword == newPassword {
		return "new password must differ from the current password"
	}
	return ""
}

func (s *Service) changeAccountPassword(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "current_password and new_password are required")
		return
	}
	if msg := validatePasswordChange(in.CurrentPassword, in.NewPassword); msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}
	var passwordHash string
	err := s.db.QueryRow(r.Context(), `select password_hash from users where id=$1 and enabled and password_hash is not null`, account.userID).Scan(&passwordHash)
	if err != nil || !passwordMatches(passwordHash, in.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "current password is incorrect")
		return
	}
	newHash, err := hashPassword(in.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not secure password")
		return
	}
	if _, err = s.db.Exec(r.Context(), `update users set password_hash=$1 where id=$2`, newHash, account.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update password")
		return
	}
	if _, err = s.db.Exec(r.Context(), `delete from user_sessions where user_id=$1 and token_hash<>$2`, account.userID, hashSecret(bearer(r))); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not revoke other sessions")
		return
	}
	s.audit(r, "account.password_changed", "user", account.userID, map[string]any{"other_sessions_revoked": true})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) updateAccountProfile(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		AvatarURL string `json:"avatar_url"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid profile")
		return
	}
	avatarURL := strings.TrimSpace(in.AvatarURL)
	if avatarURL != "" {
		if len(avatarURL) > 2<<20 || !strings.HasPrefix(avatarURL, "data:image/") {
			writeError(w, http.StatusBadRequest, "invalid_request", "avatar must be an image smaller than 2 MB")
			return
		}
		comma := strings.IndexByte(avatarURL, ',')
		if comma < 0 || !strings.HasSuffix(avatarURL[:comma], ";base64") {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid avatar image")
			return
		}
		mime := strings.TrimPrefix(strings.TrimSuffix(avatarURL[:comma], ";base64"), "data:")
		if !map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}[mime] {
			writeError(w, http.StatusBadRequest, "invalid_request", "avatar must be PNG, JPEG, GIF, or WebP")
			return
		}
		if _, err := base64.StdEncoding.DecodeString(avatarURL[comma+1:]); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid avatar image")
			return
		}
	}
	if _, err := s.db.Exec(r.Context(), `update users set avatar_url=$1 where id=$2`, avatarURL, account.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save avatar")
		return
	}
	s.audit(r, "account.avatar_updated", "user", account.userID, nil)
	writeJSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

func (s *Service) accountKeys(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select k.id,k.name,k.key_prefix,k.expires_at,k.revoked_at,k.last_used_at,k.created_at,coalesce(k.group_id::text,''),coalesce(g.name,'') from api_keys k left join groups g on g.id=k.group_id where k.user_id=$1 order by k.created_at desc`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, prefix, groupID, groupName string
		var expires, revoked, used, created any
		if rows.Scan(&id, &name, &prefix, &expires, &revoked, &used, &created, &groupID, &groupName) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "key_prefix": prefix, "group_id": groupID, "group_name": groupName, "expires_at": expires, "revoked_at": revoked, "last_used_at": used, "created_at": created})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Service) createAccountKey(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
		GroupID   string `json:"group_id"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	expires, err := parseExpiry(in.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be RFC3339")
		return
	}
	secret, err := randomSecret("sk-xh-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "key generation failed")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "key generation failed")
		return
	}
	name := strings.TrimSpace(in.Name)
	groupID, err := s.validKeyGroup(r.Context(), account.userID, in.GroupID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "group must belong to user")
		return
	}
	_, err = s.db.Exec(r.Context(), `insert into api_keys(id,user_id,name,key_prefix,secret_hash,expires_at,group_id) values($1,$2,$3,$4,$5,$6,$7)`, id, account.userID, name, secret[:12], hashSecret(secret), expires, groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create API key")
		return
	}
	s.audit(r, "api_key.created", "api_key", id, map[string]any{"user_id": account.userID, "name": name, "self_service": true})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "key": secret, "expires_at": expires, "group_id": groupID})
}

func (s *Service) setAccountKeyGroup(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		GroupID string `json:"group_id"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "group_id is required")
		return
	}
	var exists bool
	if s.db.QueryRow(r.Context(), `select exists(select 1 from api_keys where id=$1 and user_id=$2)`, r.PathValue("id"), account.userID).Scan(&exists) != nil || !exists {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	groupID, err := s.validKeyGroup(r.Context(), account.userID, in.GroupID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "group must belong to user")
		return
	}
	if _, err = s.db.Exec(r.Context(), `update api_keys set group_id=$1 where id=$2 and user_id=$3`, groupID, r.PathValue("id"), account.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update API key group")
		return
	}
	s.audit(r, "api_key.group_changed", "api_key", r.PathValue("id"), map[string]any{"group_id": groupID, "self_service": true})
	writeJSON(w, http.StatusOK, map[string]any{"group_id": groupID})
}

func (s *Service) updateAccountKey(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
		GroupID   string `json:"group_id"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || len(strings.TrimSpace(in.Name)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required and must be at most 100 characters")
		return
	}
	expires, err := parseExpiry(in.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be RFC3339")
		return
	}
	groupID, err := s.validKeyGroup(r.Context(), account.userID, in.GroupID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "group must belong to user")
		return
	}
	result, err := s.db.Exec(r.Context(), `update api_keys set name=$1,expires_at=$2,group_id=$3 where id=$4 and user_id=$5`, strings.TrimSpace(in.Name), expires, groupID, r.PathValue("id"), account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update API key")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	s.audit(r, "api_key.updated", "api_key", r.PathValue("id"), map[string]any{"name": strings.TrimSpace(in.Name), "expires_at": expires, "group_id": groupID, "self_service": true})
	writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "name": strings.TrimSpace(in.Name), "expires_at": expires, "group_id": groupID})
}

func (s *Service) accountUsage(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select request_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost,status,created_at from usage_records where user_id=$1 order by created_at desc limit 100`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var requestID, model, status string
		var prompt, cached, completion int
		var cost, created any
		if rows.Scan(&requestID, &model, &prompt, &cached, &completion, &cost, &status, &created) == nil {
			data = append(data, map[string]any{"request_id": requestID, "model": model, "prompt_tokens": prompt, "cached_prompt_tokens": cached, "completion_tokens": completion, "cost": cost, "status": status, "created_at": created})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Service) accountLedger(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select id,amount,balance_after,kind,request_id,note,created_at from wallet_ledger where user_id=$1 order by created_at desc limit 100`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, kind string
		var requestID, note any
		var amount, after, created any
		if rows.Scan(&id, &amount, &after, &kind, &requestID, &note, &created) == nil {
			data = append(data, map[string]any{"id": id, "amount": amount, "balance_after": after, "kind": kind, "request_id": requestID, "note": note, "created_at": created})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func accountFromContext(r *http.Request) accountContext {
	return r.Context().Value(accountContextKey{}).(accountContext)
}

func (s *Service) optionalAccount(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := accountContext{}
		token := bearer(r)
		if token != "" {
			err := s.db.QueryRow(r.Context(), `select s.user_id,u.role from user_sessions s join users u on u.id=s.user_id where s.token_hash=$1 and s.expires_at>now() and u.enabled`, hashSecret(token)).Scan(&account.userID, &account.role)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), accountContextKey{}, account)))
	})
}

func (s *Service) account(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "account session required")
			return
		}
		var account accountContext
		err := s.db.QueryRow(r.Context(), `select s.user_id,u.role from user_sessions s join users u on u.id=s.user_id where s.token_hash=$1 and s.expires_at>now() and u.enabled`, hashSecret(token)).Scan(&account.userID, &account.role)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		account.permissions = map[string]bool{}
		if account.role != "admin" {
			rows, queryErr := s.db.Query(r.Context(), `select permission from user_permissions where user_id=$1`, account.userID)
			if queryErr != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not load permissions")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var permission string
				if rows.Scan(&permission) == nil {
					account.permissions[permission] = true
				}
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), accountContextKey{}, account)))
	})
}

func (s *Service) permission(permission string, next http.HandlerFunc) http.Handler {
	return s.account(func(w http.ResponseWriter, r *http.Request) {
		account := r.Context().Value(accountContextKey{}).(accountContext)
		if account.role != "admin" && !account.permissions[permission] {
			writeError(w, http.StatusForbidden, "forbidden", "missing permission: "+permission)
			return
		}
		next(w, r)
	})
}

func (s *Service) createSession(w http.ResponseWriter, r *http.Request, userID string, status int) {
	token, err := randomSecret("xh_session_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = s.db.Exec(r.Context(), `insert into user_sessions(id,user_id,token_hash,expires_at) values($1,$2,$3,$4)`, id, userID, hashSecret(token), expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "expires_at": expiresAt})
}

func validPasswordLength(password string) bool {
	return len(password) >= 8 && len(password) <= 72
}

func validAccountInput(email, name, password string) bool {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil && parsed.Address == strings.TrimSpace(email) && len(strings.TrimSpace(name)) > 0 && len(strings.TrimSpace(name)) <= 100 && validPasswordLength(password)
}

func validEmail(email string) bool {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil && parsed.Address == strings.TrimSpace(email)
}
