package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) listPricing(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,model,input_per_million,cached_input_per_million,output_per_million,multiplier,enabled,updated_at from pricing_rules order by model`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, model string
		var input, cached, output, multiplier any
		var enabled bool
		var updated any
		if rows.Scan(&id, &model, &input, &cached, &output, &multiplier, &enabled, &updated) != nil {
			continue
		}
		data = append(data, map[string]any{"id": id, "model": model, "input_per_million": input, "cached_input_per_million": cached, "output_per_million": output, "multiplier": multiplier, "enabled": enabled, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) upsertPricing(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Model      string  `json:"model"`
		Input      float64 `json:"input_per_million"`
		Cached     float64 `json:"cached_input_per_million"`
		Output     float64 `json:"output_per_million"`
		Multiplier float64 `json:"multiplier"`
	}
	if decode(r, &in) != nil || in.Model == "" || in.Input < 0 || in.Cached < 0 || in.Output < 0 {
		writeError(w, 400, "invalid_request", "invalid pricing rule")
		return
	}
	if in.Multiplier == 0 {
		in.Multiplier = 1
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier) values($1,$2,$3,$4,$5,$6) on conflict(model) do update set input_per_million=excluded.input_per_million,cached_input_per_million=excluded.cached_input_per_million,output_per_million=excluded.output_per_million,multiplier=excluded.multiplier,updated_at=now()`, id, in.Model, in.Input, in.Cached, in.Output, in.Multiplier)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save pricing rule")
		return
	}
	s.audit(r, "pricing.updated", "pricing", in.Model, map[string]any{"input": in.Input, "cached": in.Cached, "output": in.Output})
	writeJSON(w, 200, map[string]any{"model": in.Model})
}

func (s *Service) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,action,actor,entity_type,entity_id,details,created_at from audit_logs order by created_at desc limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, action, actor, typ, entity string
		var details []byte
		var created any
		if rows.Scan(&id, &action, &actor, &typ, &entity, &details, &created) == nil {
			data = append(data, map[string]any{"id": id, "action": action, "actor": actor, "entity_type": typ, "entity_id": entity, "details": json.RawMessage(details), "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) audit(r *http.Request, action, entityType, entityID string, details map[string]any) {
	raw, _ := json.Marshal(details)
	id, _ := randomID()
	_, _ = s.db.Exec(r.Context(), `insert into audit_logs(id,action,actor,entity_type,entity_id,details) values($1,$2,$3,$4,$5,$6)`, id, action, accountFromContext(r).userID, entityType, entityID, raw)
}

func (s *Service) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select u.id,u.email,u.name,u.role,u.enabled,u.created_at,coalesce(array_agg(p.permission) filter (where p.permission is not null), '{}') from users u left join user_permissions p on p.user_id=u.id group by u.id order by u.created_at desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, email, name, role string
		var enabled bool
		var created any
		var permissions []string
		rows.Scan(&id, &email, &name, &role, &enabled, &created, &permissions)
		out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "enabled": enabled, "permissions": permissions, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": out})
}

var availablePermissions = map[string]bool{
	"users.read": true, "users.manage": true, "keys.manage": true, "channels.read": true,
	"channels.manage": true, "logs.read": true, "pricing.read": true, "pricing.manage": true,
	"audit.read": true, "wallets.manage": true, "routes.manage": true, "quotas.manage": true,
	"system.manage": true,
}

func (s *Service) setUserRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if decode(r, &in) != nil || (in.Role != "user" && in.Role != "operator" && in.Role != "admin") {
		writeError(w, http.StatusBadRequest, "invalid_request", "role must be user, operator, or admin")
		return
	}
	actor := accountFromContext(r)
	userID := r.PathValue("id")
	if actor.userID == userID && in.Role != "admin" {
		writeError(w, http.StatusBadRequest, "invalid_request", "cannot remove your own administrator role")
		return
	}
	result, err := s.db.Exec(r.Context(), `update users set role=$1 where id=$2`, in.Role, userID)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	s.audit(r, "user.role_changed", "user", userID, map[string]any{"role": in.Role})
	writeJSON(w, http.StatusOK, map[string]string{"role": in.Role})
}

func (s *Service) setUserPermissions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Permissions []string `json:"permissions"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "permissions are required")
		return
	}
	seen := map[string]bool{}
	for _, permission := range in.Permissions {
		if !availablePermissions[permission] || seen[permission] {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid permissions")
			return
		}
		seen[permission] = true
	}
	userID := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	defer tx.Rollback(r.Context())
	var exists bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, userID).Scan(&exists); err != nil || !exists {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if _, err = tx.Exec(r.Context(), `delete from user_permissions where user_id=$1`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	for permission := range seen {
		if _, err = tx.Exec(r.Context(), `insert into user_permissions(user_id,permission) values($1,$2)`, userID, permission); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	s.audit(r, "user.permissions_changed", "user", userID, map[string]any{"permissions": in.Permissions})
	writeJSON(w, http.StatusOK, map[string]any{"permissions": in.Permissions})
}
func (s *Service) createKey(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID    string `json:"user_id"`
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
	}
	if decode(r, &in) != nil || in.UserID == "" || in.Name == "" {
		writeError(w, 400, "invalid_request", "user_id and name are required")
		return
	}
	expires, err := parseExpiry(in.ExpiresAt)
	if err != nil {
		writeError(w, 400, "invalid_request", "expires_at must be RFC3339")
		return
	}
	secret, err := randomSecret("sk-xh-")
	if err != nil {
		writeError(w, 500, "internal_error", "key generation failed")
		return
	}
	id, _ := randomID()
	_, err = s.db.Exec(r.Context(), `insert into api_keys(id,user_id,name,key_prefix,secret_hash,expires_at) values($1,$2,$3,$4,$5,$6)`, id, in.UserID, in.Name, secret[:12], hashSecret(secret), expires)
	if err != nil {
		writeError(w, 400, "invalid_request", "unknown user")
		return
	}
	s.audit(r, "api_key.created", "api_key", id, map[string]any{"user_id": in.UserID, "name": in.Name})
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "key": secret, "expires_at": expires})
}
func (s *Service) listKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,user_id,name,key_prefix,expires_at,revoked_at,last_used_at,created_at from api_keys order by created_at desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, uid, name, prefix string
		var expiry, revoked, used, created any
		rows.Scan(&id, &uid, &name, &prefix, &expiry, &revoked, &used, &created)
		data = append(data, map[string]any{"id": id, "user_id": uid, "name": name, "key_prefix": prefix, "expires_at": expiry, "revoked_at": revoked, "last_used_at": used, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func (s *Service) revokeKey(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `update api_keys set revoked_at=coalesce(revoked_at, now()) where id=$1`, r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "API key not found")
		return
	}
	s.audit(r, "api_key.revoked", "api_key", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Service) createChannel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string   `json:"name"`
		BaseURL  string   `json:"base_url"`
		APIKey   string   `json:"api_key"`
		Models   []string `json:"models"`
		Priority int      `json:"priority"`
	}
	if decode(r, &in) != nil || in.Name == "" || in.APIKey == "" || len(in.Models) == 0 {
		writeError(w, 400, "invalid_request", "name, api_key, and models are required")
		return
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		writeError(w, 400, "invalid_request", "base_url must be an HTTPS URL")
		return
	}
	encrypted, err := crypt(s.cfg.EncryptionKey, in.APIKey, false)
	if err != nil {
		writeError(w, 500, "internal_error", "credential encryption failed")
		return
	}
	models, _ := json.Marshal(in.Models)
	id, _ := randomID()
	_, err = s.db.Exec(r.Context(), `insert into channels(id,name,base_url,api_key,models,priority) values($1,$2,$3,$4,$5,$6)`, id, in.Name, strings.TrimRight(in.BaseURL, "/"), encrypted, models, in.Priority)
	if err != nil {
		writeError(w, 409, "conflict", "channel name already exists")
		return
	}
	s.audit(r, "channel.created", "channel", id, map[string]any{"name": in.Name, "models": in.Models})
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "models": in.Models, "enabled": true})
}
func (s *Service) listChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,name,base_url,models,enabled,priority,created_at,updated_at from channels order by priority,id`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, base string
		var models []byte
		var enabled bool
		var priority int
		var created, updated any
		rows.Scan(&id, &name, &base, &models, &enabled, &priority, &created, &updated)
		var list []string
		json.Unmarshal(models, &list)
		data = append(data, map[string]any{"id": id, "name": name, "base_url": base, "models": list, "enabled": enabled, "priority": priority, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func (s *Service) setChannelStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "enabled is required")
		return
	}
	result, err := s.db.Exec(r.Context(), `update channels set enabled=$1, updated_at=now() where id=$2`, in.Enabled, r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel not found")
		return
	}
	s.audit(r, "channel.status_changed", "channel", r.PathValue("id"), map[string]any{"enabled": in.Enabled})
	writeJSON(w, 200, map[string]bool{"enabled": in.Enabled})
}

func (s *Service) adjustBalance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if decode(r, &in) != nil || in.UserID == "" || in.Amount == 0 || in.Note == "" {
		writeError(w, 400, "invalid_request", "user_id, non-zero amount, and note are required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	defer tx.Rollback(r.Context())
	var balance float64
	if err = tx.QueryRow(r.Context(), `select balance from user_wallets where user_id=$1 for update`, in.UserID).Scan(&balance); err != nil || balance+in.Amount < 0 {
		writeError(w, 400, "invalid_request", "unknown user or insufficient balance")
		return
	}
	if _, err = tx.Exec(r.Context(), `update user_wallets set balance=balance+$1,updated_at=now() where user_id=$2`, in.Amount, in.UserID); err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	id, _ := randomID()
	kind := "adjustment"
	if in.Amount > 0 {
		kind = "topup"
	}
	if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,note) values($1,$2,$3,$4,$5,$6)`, id, in.UserID, in.Amount, balance+in.Amount, kind, in.Note); err != nil {
		writeError(w, 500, "internal_error", "could not record adjustment")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	s.audit(r, "wallet.adjusted", "user", in.UserID, map[string]any{"amount": in.Amount, "note": in.Note})
	writeJSON(w, 200, map[string]any{"balance": balance + in.Amount})
}

func (s *Service) listModelRoutes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,public_model,upstream_model,channel_id,priority,weight,enabled,created_at from model_routes order by public_model,priority`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, public, upstream, channelID string
		var priority, weight int
		var enabled bool
		var created any
		if rows.Scan(&id, &public, &upstream, &channelID, &priority, &weight, &enabled, &created) == nil {
			data = append(data, map[string]any{"id": id, "public_model": public, "upstream_model": upstream, "channel_id": channelID, "priority": priority, "weight": weight, "enabled": enabled, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) createModelRoute(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PublicModel   string `json:"public_model"`
		UpstreamModel string `json:"upstream_model"`
		ChannelID     string `json:"channel_id"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
	}
	if decode(r, &in) != nil || in.PublicModel == "" || in.UpstreamModel == "" || in.ChannelID == "" {
		writeError(w, 400, "invalid_request", "public_model, upstream_model, and channel_id are required")
		return
	}
	if in.Weight <= 0 {
		in.Weight = 100
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight) values($1,$2,$3,$4,$5,$6)`, id, in.PublicModel, in.UpstreamModel, in.ChannelID, in.Priority, in.Weight)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not create model route")
		return
	}
	s.audit(r, "model_route.created", "model_route", id, map[string]any{"public_model": in.PublicModel, "channel_id": in.ChannelID})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Service) upsertQuota(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID      string `json:"user_id"`
		APIKeyID    string `json:"api_key_id"`
		Model       string `json:"model"`
		Window      string `json:"window"`
		MaxRequests *int64 `json:"max_requests"`
		MaxTokens   *int64 `json:"max_tokens"`
	}
	if decode(r, &in) != nil || (in.UserID == "" && in.APIKeyID == "") || (in.Window != "minute" && in.Window != "day" && in.Window != "month") || (in.MaxRequests == nil && in.MaxTokens == nil) {
		writeError(w, 400, "invalid_request", "scope, window, and a limit are required")
		return
	}
	if (in.MaxRequests != nil && *in.MaxRequests < 0) || (in.MaxTokens != nil && *in.MaxTokens < 0) {
		writeError(w, 400, "invalid_request", "limits cannot be negative")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens) values($1,nullif($2,'')::uuid,nullif($3,'')::uuid,nullif($4,''),$5,$6,$7) on conflict (coalesce(user_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window") do update set max_requests=excluded.max_requests,max_tokens=excluded.max_tokens`, id, in.UserID, in.APIKeyID, in.Model, in.Window, in.MaxRequests, in.MaxTokens)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save quota")
		return
	}
	s.audit(r, "quota.updated", "quota", id, map[string]any{"user_id": in.UserID, "api_key_id": in.APIKeyID, "model": in.Model, "window": in.Window})
	writeJSON(w, 200, map[string]any{"id": id})
}
func (s *Service) listLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select request_id,user_id,api_key_id,channel_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code,created_at from request_logs order by created_at desc limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var requestID, model string
		var uid, kid, cid, prompt, completion, total, errorCode, created any
		var status, duration int
		rows.Scan(&requestID, &uid, &kid, &cid, &model, &status, &prompt, &completion, &total, &duration, &errorCode, &created)
		data = append(data, map[string]any{"request_id": requestID, "user_id": uid, "api_key_id": kid, "channel_id": cid, "model": model, "status_code": status, "prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total, "duration_ms": duration, "error_code": errorCode, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
