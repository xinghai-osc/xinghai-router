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

type keyContext struct{ userID, keyID, groupID string }
type contextKey struct{}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/email-code", s.sendEmailCode)
	mux.Handle("GET /model-catalog", s.optionalAccount(s.modelCatalog))
	mux.HandleFunc("GET /site-settings", s.siteSettings)
	mux.HandleFunc("GET /rankings", s.rankings)
	mux.Handle("POST /auth/logout", s.account(s.logout))
	mux.Handle("GET /account/me", s.account(s.accountMe))
	mux.Handle("PUT /account/profile", s.account(s.updateAccountProfile))
	mux.Handle("PUT /account/password", s.account(s.changeAccountPassword))
	mux.Handle("PUT /account/preferences", s.account(s.updateAccountPreferences))
	mux.Handle("GET /account/keys", s.account(s.accountKeys))
	mux.Handle("POST /account/keys", s.account(s.createAccountKey))
	mux.Handle("PUT /account/keys/{id}", s.account(s.updateAccountKey))
	mux.Handle("PUT /account/keys/{id}/group", s.account(s.setAccountKeyGroup))
	mux.Handle("GET /account/usage", s.account(s.accountUsage))
	mux.Handle("GET /account/ledger", s.account(s.accountLedger))
	mux.Handle("GET /account/payments", s.account(s.listAccountPayments))
	mux.Handle("POST /account/payments", s.account(s.createAccountPayment))
	mux.Handle("GET /account/payments/{order_no}", s.account(s.getAccountPayment))
	mux.Handle("GET /account/groups", s.account(s.accountGroups))
	mux.HandleFunc("GET /payments/epay/notify", s.epayNotify)
	mux.HandleFunc("POST /payments/epay/notify", s.epayNotify)
	mux.Handle("GET /activity-logs", s.account(s.listActivityLogs))
	mux.Handle("GET /subscription-plans", s.optionalAccount(s.publicSubscriptionPlans))
	mux.Handle("GET /account/subscriptions", s.account(s.accountSubscriptions))
	mux.Handle("POST /account/subscriptions", s.account(s.createSubscription))
	mux.Handle("POST /account/subscriptions/{id}/cancel", s.account(s.cancelSubscription))
	mux.Handle("GET /account/subscription-orders", s.account(s.accountSubscriptionOrders))
	mux.Handle("GET /account/subscription-orders/{order_no}", s.account(s.accountSubscriptionOrder))
	mux.Handle("GET /admin/subscription-plans", s.permission("system.manage", s.listSubscriptionPlans))
	mux.Handle("POST /admin/subscription-plans", s.permission("system.manage", s.createSubscriptionPlan))
	mux.Handle("PUT /admin/subscription-plans/{id}", s.permission("system.manage", s.updateSubscriptionPlan))
	mux.Handle("DELETE /admin/subscription-plans/{id}", s.permission("system.manage", s.deleteSubscriptionPlan))
	mux.Handle("GET /admin/subscriptions", s.permission("users.read", s.adminListSubscriptions))
	mux.Handle("POST /admin/subscriptions/extend", s.permission("system.manage", s.batchExtendSubscriptions))
	mux.Handle("GET /admin/users", s.permission("users.read", s.listUsers))
	mux.Handle("PUT /admin/users/{id}", s.permission("system.manage", s.updateUser))
	mux.Handle("POST /admin/users/{id}/role", s.permission("system.manage", s.setUserRole))
	mux.Handle("PUT /admin/users/{id}/permissions", s.permission("system.manage", s.setUserPermissions))
	mux.Handle("GET /admin/groups", s.permission("users.read", s.listGroups))
	mux.Handle("GET /group", s.permission("users.read", s.listGroupNames))
	mux.Handle("POST /admin/groups", s.permission("system.manage", s.createGroup))
	mux.Handle("POST /admin/groups/import", s.permission("system.manage", s.importGroups))
	mux.Handle("PUT /admin/groups/{id}", s.permission("system.manage", s.updateGroup))
	mux.Handle("PUT /admin/users/{id}/groups", s.permission("system.manage", s.setUserGroups))
	mux.Handle("POST /admin/keys", s.permission("keys.manage", s.createKey))
	mux.Handle("GET /admin/keys", s.permission("keys.manage", s.listKeys))
	mux.Handle("POST /admin/keys/{id}/revoke", s.permission("keys.manage", s.revokeKey))
	mux.Handle("PUT /admin/keys/{id}/group", s.permission("keys.manage", s.setKeyGroup))
	mux.Handle("POST /admin/channels", s.permission("channels.manage", s.createChannel))
	mux.Handle("PUT /admin/channels/{id}", s.permission("channels.manage", s.updateChannel))
	mux.Handle("POST /admin/channels/models", s.permission("channels.manage", s.fetchChannelModels))
	mux.Handle("GET /admin/channels", s.permission("channels.read", s.listChannels))
	mux.Handle("GET /admin/providers", s.permission("system.manage", s.listProviders))
	mux.Handle("POST /admin/providers", s.permission("system.manage", s.saveProvider))
	mux.Handle("DELETE /admin/providers/{id}", s.permission("system.manage", s.deleteProvider))
	mux.Handle("POST /admin/channels/{id}/status", s.permission("channels.manage", s.setChannelStatus))
	mux.Handle("PUT /admin/channels/{id}/groups", s.permission("channels.manage", s.setChannelGroups))
	mux.Handle("GET /admin/request-logs", s.permission("logs.read", s.listLogs))
	mux.Handle("GET /admin/pricing", s.permission("pricing.read", s.listPricing))
	mux.Handle("POST /admin/pricing", s.permission("pricing.manage", s.upsertPricing))
	mux.Handle("GET /admin/site-settings", s.permission("system.manage", s.adminSiteSettings))
	mux.Handle("PUT /admin/site-settings", s.permission("system.manage", s.updateSiteSettings))
	mux.Handle("GET /admin/reliability-settings", s.permission("system.manage", s.getReliabilitySettings))
	mux.Handle("PUT /admin/reliability-settings", s.permission("system.manage", s.updateReliabilitySettings))
	mux.Handle("GET /admin/payment-settings", s.permission("system.manage", s.getPaymentSettings))
	mux.Handle("PUT /admin/payment-settings", s.permission("system.manage", s.updatePaymentSettings))
	mux.Handle("POST /admin/payment-methods", s.permission("system.manage", s.createPaymentMethod))
	mux.Handle("PUT /admin/payment-methods/{id}", s.permission("system.manage", s.updatePaymentMethod))
	mux.Handle("DELETE /admin/payment-methods/{id}", s.permission("system.manage", s.deletePaymentMethod))
	mux.Handle("POST /admin/pricing/newapi/sync", s.permission("pricing.manage", s.syncNewAPIPricing))
	mux.Handle("GET /admin/audit-logs", s.permission("audit.read", s.listAuditLogs))
	mux.Handle("POST /admin/wallets/adjustments", s.permission("wallets.manage", s.adjustBalance))
	mux.Handle("GET /admin/model-routes", s.permission("routes.manage", s.listModelRoutes))
	mux.Handle("POST /admin/model-routes", s.permission("routes.manage", s.createModelRoute))
	mux.Handle("POST /admin/quota-limits", s.permission("quotas.manage", s.upsertQuota))
	mux.Handle("POST /admin/migrate", s.permission("system.manage", s.runMigration))
	mux.Handle("GET /admin/migrate", s.permission("system.manage", s.getMigrationStatus))
	mux.Handle("GET /me", s.api(s.me))
	mux.Handle("GET /me/keys", s.api(s.myKeys))
	mux.Handle("GET /me/usage", s.api(s.myUsage))
	mux.Handle("GET /me/ledger", s.api(s.myLedger))
	mux.Handle("GET /me/groups", s.api(s.myGroups))
	mux.Handle("GET /v1/models", s.api(s.models))
	mux.Handle("POST /v1/chat/completions", s.api(s.chatCompletions))
	mux.Handle("POST /v1/messages", s.api(s.anthropicMessages))
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
		err := s.db.QueryRow(r.Context(), `select k.user_id,k.id,coalesce(k.group_id::text,'') from api_keys k join users u on u.id=k.user_id where k.secret_hash=$1 and k.revoked_at is null and (k.expires_at is null or k.expires_at>now()) and u.enabled and (k.group_id is null or exists(select 1 from user_groups ug where ug.user_id=k.user_id and ug.group_id=k.group_id))`, hashSecret(token)).Scan(&k.userID, &k.keyID, &k.groupID)
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
	rows, err := s.db.Query(r.Context(), `select k.id,k.name,k.key_prefix,k.expires_at,k.revoked_at,k.last_used_at,k.created_at,coalesce(k.group_id::text,''),coalesce(g.name,'') from api_keys k left join groups g on g.id=k.group_id where k.user_id=$1 order by k.created_at desc`, key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
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
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
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
