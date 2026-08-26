package app

import (
	"context"
	"net/http"
	"strings"
)

// keyQuotaWindows are the windows a self-service user may choose from. The
// admin upsertQuota handler also accepts 'minute'; users are limited to the
// windows that make sense for a per-key budget.
var keyQuotaWindows = map[string]bool{"day": true, "month": true, "total": true}

const maxKeyQuotaCost = 1_000_000_000.0

func validKeyQuotaCost(value *float64) bool {
	if value == nil {
		return true
	}
	return validNonNegativeFinite(*value) && *value <= maxKeyQuotaCost
}

// keyQuotaList returns the quota_limits rows that belong to one of the caller's
// own keys, along with the current usage for each row's window. Only rows
// scoped to the key (model IS NULL) are shown — model-specific quotas are an
// admin-only concern.
func (s *Service) keyQuotaList(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	keyID := strings.TrimSpace(r.PathValue("id"))
	if keyID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "key id is required")
		return
	}
	var owner string
	if err := s.db.QueryRow(r.Context(), `select user_id from api_keys where id=$1`, keyID).Scan(&owner); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	if owner != account.userID {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	limits, err := s.listKeyQuotaLimits(r.Context(), keyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	usage, err := s.keyQuotaUsage(r.Context(), keyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"limits": limits, "usage": usage})
}

func (s *Service) listKeyQuotaLimits(ctx context.Context, keyID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `select id,"window",max_requests,max_tokens,max_cost,created_at from quota_limits where api_key_id=$1 and model is null order by "window"`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, window string
		var maxRequests, maxTokens *int64
		var maxCost *float64
		var created any
		if err := rows.Scan(&id, &window, &maxRequests, &maxTokens, &maxCost, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id":           id,
			"window":       window,
			"max_requests": maxRequests,
			"max_tokens":   maxTokens,
			"max_cost":     maxCost,
			"created_at":   created,
		})
	}
	return result, rows.Err()
}

// keyQuotaUsage returns the current requests / tokens / cost for each window
// that has a limit row, using the same aggregation logic as checkQuota so the
// UI shows the same numbers the gateway enforces.
func (s *Service) keyQuotaUsage(ctx context.Context, keyID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `select q."window",agg.requests,agg.tokens,agg.cost
	from quota_limits q
	left join lateral (
		select count(rl.*) as requests, coalesce(sum(rl.total_tokens),0) as tokens, coalesce(sum(ur.cost),0) as cost
		from request_logs rl
		left join usage_records ur on ur.request_id=rl.request_id
		where rl.api_key_id=$1 and (q."window"='total' or rl.created_at >= now() - ('1 '||q."window")::interval)
	) agg on true
	where q.api_key_id=$1 and q.model is null
	order by q."window"`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var window string
		var requests, tokens int64
		var cost float64
		if err := rows.Scan(&window, &requests, &tokens, &cost); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"window":   window,
			"requests": requests,
			"tokens":   tokens,
			"cost":     cost,
		})
	}
	return result, rows.Err()
}

// keyQuotaUpsert creates or updates a per-key quota limit. Self-service users
// can only set limits on their own keys, only with day/month/total windows,
// and only for the whole key (no per-model quotas).
func (s *Service) keyQuotaUpsert(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	keyID := strings.TrimSpace(r.PathValue("id"))
	if keyID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "key id is required")
		return
	}
	var in struct {
		Window      string   `json:"window"`
		MaxRequests *int64   `json:"max_requests"`
		MaxTokens   *int64   `json:"max_tokens"`
		MaxCost     *float64 `json:"max_cost"`
	}
	if decode(r, &in) != nil || !keyQuotaWindows[in.Window] || (in.MaxRequests == nil && in.MaxTokens == nil && in.MaxCost == nil) {
		writeError(w, http.StatusBadRequest, "invalid_request", "window and at least one limit are required")
		return
	}
	if !validQuotaLimit(in.MaxRequests) || !validQuotaLimit(in.MaxTokens) || !validKeyQuotaCost(in.MaxCost) {
		writeError(w, http.StatusBadRequest, "invalid_request", "limits must be non-negative and within range")
		return
	}
	var owner string
	if err := s.db.QueryRow(r.Context(), `select user_id from api_keys where id=$1 and revoked_at is null`, keyID).Scan(&owner); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	if owner != account.userID {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens,max_cost) values($1,$2,$3,null,$4,$5,$6,$7) on conflict (coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window") do update set max_requests=excluded.max_requests,max_tokens=excluded.max_tokens,max_cost=excluded.max_cost`, id, account.userID, keyID, in.Window, in.MaxRequests, in.MaxTokens, in.MaxCost)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not save quota")
		return
	}
	s.audit(r, "key_quota.updated", "api_key", keyID, map[string]any{"window": in.Window, "max_requests": in.MaxRequests, "max_tokens": in.MaxTokens, "max_cost": in.MaxCost, "self_service": true})
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// keyQuotaDelete removes a per-key quota limit by window.
func (s *Service) keyQuotaDelete(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	keyID := strings.TrimSpace(r.PathValue("id"))
	window := strings.TrimSpace(r.URL.Query().Get("window"))
	if keyID == "" || window == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "key id and window are required")
		return
	}
	var owner string
	if err := s.db.QueryRow(r.Context(), `select user_id from api_keys where id=$1`, keyID).Scan(&owner); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	if owner != account.userID {
		writeError(w, http.StatusNotFound, "not_found", "API key not found")
		return
	}
	_, err := s.db.Exec(r.Context(), `delete from quota_limits where api_key_id=$1 and model is null and "window"=$2`, keyID, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete quota")
		return
	}
	s.audit(r, "key_quota.deleted", "api_key", keyID, map[string]any{"window": window, "self_service": true})
	w.WriteHeader(http.StatusNoContent)
}
