package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type keyContext struct{ userID, keyID string }
type contextKey struct{}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.Handle("POST /auth/logout", s.account(s.logout))
	mux.Handle("GET /account/me", s.account(s.accountMe))
	mux.Handle("GET /admin/users", s.permission("users.read", s.listUsers))
	mux.Handle("POST /admin/users/{id}/role", s.permission("system.manage", s.setUserRole))
	mux.Handle("PUT /admin/users/{id}/permissions", s.permission("system.manage", s.setUserPermissions))
	mux.Handle("POST /admin/keys", s.permission("keys.manage", s.createKey))
	mux.Handle("GET /admin/keys", s.permission("keys.manage", s.listKeys))
	mux.Handle("POST /admin/keys/{id}/revoke", s.permission("keys.manage", s.revokeKey))
	mux.Handle("POST /admin/channels", s.permission("channels.manage", s.createChannel))
	mux.Handle("GET /admin/channels", s.permission("channels.read", s.listChannels))
	mux.Handle("POST /admin/channels/{id}/status", s.permission("channels.manage", s.setChannelStatus))
	mux.Handle("GET /admin/request-logs", s.permission("logs.read", s.listLogs))
	mux.Handle("GET /admin/pricing", s.permission("pricing.read", s.listPricing))
	mux.Handle("POST /admin/pricing", s.permission("pricing.manage", s.upsertPricing))
	mux.Handle("GET /admin/audit-logs", s.permission("audit.read", s.listAuditLogs))
	mux.Handle("POST /admin/wallets/adjustments", s.permission("wallets.manage", s.adjustBalance))
	mux.Handle("GET /admin/model-routes", s.permission("routes.manage", s.listModelRoutes))
	mux.Handle("POST /admin/model-routes", s.permission("routes.manage", s.createModelRoute))
	mux.Handle("POST /admin/quota-limits", s.permission("quotas.manage", s.upsertQuota))
	mux.Handle("GET /me", s.api(s.me))
	mux.Handle("GET /me/keys", s.api(s.myKeys))
	mux.Handle("GET /me/usage", s.api(s.myUsage))
	mux.Handle("GET /me/ledger", s.api(s.myLedger))
	mux.Handle("GET /v1/models", s.api(s.models))
	mux.Handle("POST /v1/chat/completions", s.api(s.chatCompletions))
	return s.requestID(mux)
}
func (s *Service) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := randomID()
		if err != nil {
			writeError(w, 500, "internal_error", "could not create request id")
			return
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

type requestIDKey struct{}

func requestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }
func (s *Service) api(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeError(w, 401, "invalid_api_key", "API key required")
			return
		}
		var k keyContext
		err := s.db.QueryRow(r.Context(), `select k.user_id,k.id from api_keys k join users u on u.id=k.user_id where k.secret_hash=$1 and k.revoked_at is null and (k.expires_at is null or k.expires_at>now()) and u.enabled`, hashSecret(token)).Scan(&k.userID, &k.keyID)
		if err != nil {
			writeError(w, 401, "invalid_api_key", "invalid or expired API key")
			return
		}
		if !s.limiter.allow(k.keyID) {
			writeError(w, 429, "rate_limit_exceeded", "too many requests")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, k)))
	})
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	var email, name, role string
	var balance, reserved any
	err := s.db.QueryRow(r.Context(), `select u.email,u.name,u.role,coalesce(w.balance,0),coalesce(w.reserved,0) from users u left join user_wallets w on w.user_id=u.id where u.id=$1`, key.userID).Scan(&email, &name, &role, &balance, &reserved)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load account")
		return
	}
	writeJSON(w, 200, map[string]any{"user_id": key.userID, "key_id": key.keyID, "email": email, "name": name, "role": role, "balance": balance, "reserved": reserved})
}
func (s *Service) myKeys(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select id,name,key_prefix,expires_at,revoked_at,last_used_at,created_at from api_keys where user_id=$1 order by created_at desc`, key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, prefix string
		var expires, revoked, used, created any
		if rows.Scan(&id, &name, &prefix, &expires, &revoked, &used, &created) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "key_prefix": prefix, "expires_at": expires, "revoked_at": revoked, "last_used_at": used, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func (s *Service) myUsage(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select request_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost,status,created_at from usage_records where user_id=$1 order by created_at desc limit 100`, key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
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
	writeJSON(w, 200, map[string]any{"data": data})
}
func (s *Service) myLedger(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select id,amount,balance_after,kind,request_id,note,created_at from wallet_ledger where user_id=$1 order by created_at desc limit 100`, key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, kind, requestID, note string
		var amount, after, created any
		if rows.Scan(&id, &amount, &after, &kind, &requestID, &note, &created) == nil {
			data = append(data, map[string]any{"id": id, "amount": amount, "balance_after": after, "kind": kind, "request_id": requestID, "note": note, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func bearer(r *http.Request) string {
	const p = "Bearer "
	v := r.Header.Get("Authorization")
	if strings.HasPrefix(v, p) {
		return strings.TrimSpace(strings.TrimPrefix(v, p))
	}
	return ""
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": msg, "type": code, "code": code}})
}
func decode(r *http.Request, target any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

var errInvalid = errors.New("invalid request")

func parseExpiry(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	return &t, err
}
