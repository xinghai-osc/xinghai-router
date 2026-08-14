package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type keyContext struct{ userID, keyID, groupID string; dataUsageEnabled bool }
type contextKey struct{}

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.Handle("POST /auth/register", s.ipRateLimit(s.register))
	mux.Handle("POST /auth/login", s.ipRateLimit(s.login))
	mux.Handle("POST /auth/email-code", s.ipRateLimit(s.sendEmailCode))
	mux.Handle("POST /auth/password-reset/request", s.ipRateLimit(s.requestPasswordReset))
	mux.Handle("POST /auth/password-reset/confirm", s.ipRateLimit(s.confirmPasswordReset))
	mux.HandleFunc("GET /auth/oauth/{provider}", s.oauthAuthorize)
	mux.HandleFunc("GET /auth/oauth/{provider}/callback", s.oauthCallback)
	mux.Handle("GET /model-catalog", s.optionalAccount(s.modelCatalog))
	mux.Handle("GET /model-performance", s.ipRateLimitBy(s.performanceLimiter, s.modelPerformance))
	mux.HandleFunc("GET /site-settings", s.siteSettings)
	mux.Handle("GET /rankings", s.ipRateLimitBy(s.rankingsLimiter, s.rankings))
	mux.Handle("GET /public/activity", s.account(s.publicActivity))
	mux.Handle("POST /auth/logout", s.account(s.logout))
	mux.Handle("GET /account/me", s.account(s.accountMe))
	mux.Handle("PUT /account/profile", s.account(s.updateAccountProfile))
	mux.Handle("PUT /account/password", s.account(s.changeAccountPassword))
	mux.Handle("PUT /account/preferences", s.account(s.updateAccountPreferences))
	mux.Handle("GET /account/keys", s.account(s.accountKeys))
	mux.Handle("POST /account/keys", s.account(s.createAccountKey))
	mux.Handle("PUT /account/keys/{id}", s.account(s.updateAccountKey))
	mux.Handle("PUT /account/keys/{id}/group", s.account(s.setAccountKeyGroup))
	mux.Handle("POST /account/keys/{id}/revoke", s.account(s.revokeAccountKey))
	mux.Handle("GET /account/keys/{id}/secret", s.account(s.revealAccountKey))
	mux.Handle("GET /account/keys/{id}/quota", s.account(s.keyQuotaList))
	mux.Handle("POST /account/keys/{id}/quota", s.account(s.keyQuotaUpsert))
	mux.Handle("DELETE /account/keys/{id}/quota", s.account(s.keyQuotaDelete))
	mux.Handle("GET /account/usage", s.account(s.accountUsage))
	mux.Handle("GET /account/usage/daily", s.account(s.accountUsageDaily))
	mux.Handle("GET /account/usage/summary", s.account(s.accountUsageSummary))
	mux.Handle("GET /account/ledger", s.account(s.accountLedger))
	mux.Handle("GET /account/payments", s.account(s.listAccountPayments))
	mux.Handle("POST /account/payments", s.account(s.createAccountPayment))
	mux.Handle("GET /account/payments/{order_no}", s.account(s.getAccountPayment))
	mux.Handle("GET /account/orders", s.account(s.accountOrders))
	mux.Handle("POST /account/redeem", s.account(s.redeemCode))
	mux.Handle("GET /account/redemptions", s.account(s.accountRedemptions))
	mux.Handle("GET /account/groups", s.account(s.accountGroups))
	mux.Handle("GET /account/oauth/connections", s.account(s.listOAuthConnections))
	mux.Handle("POST /account/oauth/{provider}/unlink", s.account(s.unlinkOAuthConnection))
	mux.HandleFunc("GET /payments/epay/notify", s.epayNotify)
	mux.HandleFunc("POST /payments/epay/notify", s.epayNotify)
	mux.Handle("GET /activity-logs", s.account(s.listActivityLogs))
	mux.Handle("GET /subscription-plans", s.optionalAccount(s.publicSubscriptionPlans))
	mux.Handle("GET /account/subscriptions", s.account(s.accountSubscriptions))
	mux.Handle("POST /account/subscriptions", s.account(s.createSubscription))
	mux.Handle("POST /account/subscriptions/{id}/cancel", s.account(s.cancelSubscription))
	mux.Handle("GET /account/subscription-orders", s.account(s.accountSubscriptionOrders))
	mux.Handle("GET /account/subscription-orders/{order_no}", s.account(s.accountSubscriptionOrder))
	mux.Handle("GET /account/invoice/settings", s.account(s.accountInvoiceSettings))
	mux.Handle("GET /account/invoices/eligible-orders", s.account(s.accountInvoiceEligibleOrders))
	mux.Handle("POST /account/invoices/validate", s.account(s.accountInvoiceValidate))
	mux.Handle("POST /account/invoices/tax-status", s.account(s.accountInvoiceTaxStatus))
	mux.Handle("POST /account/invoices", s.account(s.accountInvoiceSubmit))
	mux.Handle("GET /account/invoices", s.account(s.accountInvoices))
	mux.Handle("GET /account/invoices/{id}/pdf", s.account(s.accountInvoicePDF))
	mux.Handle("POST /account/invoices/{id}/cancel", s.account(s.accountInvoiceCancel))
	mux.Handle("GET /admin/subscription-plans", s.permission("system.manage", s.listSubscriptionPlans))
	mux.Handle("POST /admin/subscription-plans", s.permission("system.manage", s.createSubscriptionPlan))
	mux.Handle("PUT /admin/subscription-plans/{id}", s.permission("system.manage", s.updateSubscriptionPlan))
	mux.Handle("DELETE /admin/subscription-plans/{id}", s.permission("system.manage", s.deleteSubscriptionPlan))
	mux.Handle("GET /admin/subscriptions", s.permission("users.read", s.adminListSubscriptions))
	mux.Handle("POST /admin/subscriptions/extend", s.permission("system.manage", s.batchExtendSubscriptions))
	mux.Handle("GET /admin/users/{id}/subscriptions", s.permission("users.read", s.adminUserSubscriptions))
	mux.Handle("POST /admin/users/{id}/subscriptions", s.permission("system.manage", s.adminCreateSubscription))
	mux.Handle("PUT /admin/subscriptions/{id}", s.permission("system.manage", s.adminUpdateSubscription))
	mux.Handle("POST /admin/subscriptions/{id}/void", s.permission("system.manage", s.adminVoidSubscription))
	mux.Handle("DELETE /admin/subscriptions/{id}", s.permission("system.manage", s.adminDeleteSubscription))
	mux.Handle("GET /admin/users", s.permission("users.read", s.listUsers))
	mux.Handle("PUT /admin/users/{id}", s.permission("system.manage", s.updateUser))
	mux.Handle("POST /admin/users/{id}/role", s.permission("system.manage", s.setUserRole))
	mux.Handle("PUT /admin/users/{id}/permissions", s.permission("system.manage", s.setUserPermissions))
	mux.Handle("GET /admin/groups", s.permission("users.read", s.listGroups))
	mux.Handle("GET /group", s.permission("users.read", s.listGroupNames))
	mux.Handle("POST /admin/groups", s.permission("system.manage", s.createGroup))
	mux.Handle("POST /admin/groups/import", s.permission("system.manage", s.importGroups))
	mux.Handle("POST /admin/groups/batch-update", s.permission("system.manage", s.batchUpdateGroups))
	mux.Handle("PUT /admin/groups/{id}", s.permission("system.manage", s.updateGroup))
	mux.Handle("DELETE /admin/groups/{id}", s.permission("system.manage", s.deleteGroup))
	mux.Handle("PUT /admin/users/{id}/groups", s.permission("system.manage", s.setUserGroups))
	mux.Handle("POST /admin/keys", s.permission("keys.manage", s.createKey))
	mux.Handle("GET /admin/keys", s.permission("keys.manage", s.listKeys))
	mux.Handle("POST /admin/keys/{id}/revoke", s.permission("keys.manage", s.revokeKey))
	mux.Handle("PUT /admin/keys/{id}/group", s.permission("keys.manage", s.setKeyGroup))
	mux.Handle("GET /admin/keys/{id}/secret", s.permission("keys.manage", s.revealKey))
	mux.Handle("POST /admin/channels", s.permission("channels.manage", s.createChannel))
	mux.Handle("PUT /admin/channels/{id}", s.permission("channels.manage", s.updateChannel))
	mux.Handle("POST /admin/channels/models", s.permission("channels.manage", s.fetchChannelModels))
	mux.Handle("GET /admin/channels", s.permission("channels.read", s.listChannels))
	mux.Handle("GET /admin/providers", s.permission("system.manage", s.listProviders))
	mux.Handle("POST /admin/providers", s.permission("system.manage", s.saveProvider))
	mux.Handle("DELETE /admin/providers/{id}", s.permission("system.manage", s.deleteProvider))
	mux.Handle("POST /admin/channels/batch-status", s.permission("channels.manage", s.batchSetChannelStatus))
	mux.Handle("POST /admin/channels/{id}/status", s.permission("channels.manage", s.setChannelStatus))
	mux.Handle("POST /admin/channels/{id}/test", s.permission("channels.manage", s.testChannelHandler))
	mux.Handle("PUT /admin/channels/{id}/groups", s.permission("channels.manage", s.setChannelGroups))
	mux.Handle("GET /admin/request-logs", s.permission("logs.read", s.listLogs))
	mux.Handle("GET /admin/usage-logs", s.permission("logs.read", s.listUsageLogs))
	mux.Handle("GET /admin/usage-stats", s.permission("logs.read", s.usageStats))
	mux.Handle("GET /admin/pricing", s.permission("pricing.read", s.listPricing))
	mux.Handle("POST /admin/pricing", s.permission("pricing.manage", s.upsertPricing))
	mux.Handle("GET /admin/pricing/tiers", s.permission("pricing.read", s.listPricingTiers))
	mux.Handle("POST /admin/pricing/tiers", s.permission("pricing.manage", s.savePricingTier))
	mux.Handle("DELETE /admin/pricing/tiers/{id}", s.permission("pricing.manage", s.deletePricingTier))
	mux.Handle("GET /admin/pricing/time-rules", s.permission("pricing.read", s.listPricingTimeRules))
	mux.Handle("POST /admin/pricing/time-rules", s.permission("pricing.manage", s.savePricingTimeRule))
	mux.Handle("DELETE /admin/pricing/time-rules/{id}", s.permission("pricing.manage", s.deletePricingTimeRule))
	mux.Handle("GET /admin/site-settings", s.permission("system.manage", s.adminSiteSettings))
	mux.Handle("PUT /admin/site-settings", s.permission("system.manage", s.updateSiteSettings))
	mux.Handle("GET /admin/reliability-settings", s.permission("system.manage", s.getReliabilitySettings))
	mux.Handle("PUT /admin/reliability-settings", s.permission("system.manage", s.updateReliabilitySettings))
	mux.Handle("GET /admin/conversation-cache/settings", s.permission("system.manage", s.getConversationCacheSettings))
	mux.Handle("PUT /admin/conversation-cache/settings", s.permission("system.manage", s.updateConversationCacheSettings))
	mux.Handle("GET /admin/conversation-cache", s.permission("logs.read", s.listConversationCache))
	mux.Handle("GET /admin/conversation-cache/{id}", s.permission("logs.read", s.getConversationCacheDetail))
	mux.Handle("GET /admin/payment-settings", s.permission("system.manage", s.getPaymentSettings))
	mux.Handle("PUT /admin/payment-settings", s.permission("system.manage", s.updatePaymentSettings))
	mux.Handle("GET /admin/invoice-settings", s.permission("system.manage", s.adminGetInvoiceSettings))
	mux.Handle("PUT /admin/invoice-settings", s.permission("system.manage", s.adminUpdateInvoiceSettings))
	mux.Handle("POST /admin/payment-methods", s.permission("system.manage", s.createPaymentMethod))
	mux.Handle("PUT /admin/payment-methods/{id}", s.permission("system.manage", s.updatePaymentMethod))
	mux.Handle("DELETE /admin/payment-methods/{id}", s.permission("system.manage", s.deletePaymentMethod))
	mux.Handle("POST /admin/pricing/newapi/sync", s.permission("pricing.manage", s.syncNewAPIPricing))
	mux.Handle("GET /admin/audit-logs", s.permission("audit.read", s.listAuditLogs))
	mux.Handle("POST /admin/wallets/adjustments", s.permission("wallets.manage", s.adjustBalance))
	mux.Handle("GET /admin/redemption-codes", s.permission("system.manage", s.listRedemptionCodes))
	mux.Handle("POST /admin/redemption-codes", s.permission("system.manage", s.createRedemptionCodes))
	mux.Handle("PUT /admin/redemption-codes/{id}", s.permission("system.manage", s.updateRedemptionCode))
	mux.Handle("DELETE /admin/redemption-codes/{id}", s.permission("system.manage", s.deleteRedemptionCode))
	mux.Handle("GET /admin/redemption-codes/{id}/redemptions", s.permission("system.manage", s.listCodeRedemptions))
	mux.Handle("GET /admin/model-routes", s.permission("routes.manage", s.listModelRoutes))
	mux.Handle("POST /admin/model-routes", s.permission("routes.manage", s.createModelRoute))
	mux.Handle("PUT /admin/model-routes/{id}", s.permission("routes.manage", s.updateModelRoute))
	mux.Handle("GET /admin/channels/{id}/keys", s.permission("channels.read", s.listChannelKeys))
	mux.Handle("POST /admin/channels/{id}/keys", s.permission("channels.manage", s.createChannelKey))
	mux.Handle("DELETE /admin/channels/{id}/keys/{keyId}", s.permission("channels.manage", s.deleteChannelKey))
	mux.Handle("PUT /admin/channels/{id}/keys/{keyId}", s.permission("channels.manage", s.updateChannelKey))
	mux.Handle("GET /admin/channels/{id}/keys/{keyId}/secret", s.permission("channels.manage", s.revealChannelKey))
	mux.Handle("POST /admin/channels/{id}/keys/{keyId}/test", s.permission("channels.manage", s.testChannelKey))
	mux.Handle("POST /admin/channels/{id}/keys/{keyId}/status", s.permission("channels.manage", s.setChannelKeyStatus))
	mux.Handle("POST /admin/channels/{id}/keys/migrate", s.permission("channels.manage", s.migrateChannelKeys))
	mux.Handle("GET /admin/channels/{id}/routes", s.permission("routes.manage", s.listChannelRoutes))
	mux.Handle("POST /admin/channels/{id}/routes", s.permission("routes.manage", s.createChannelRoute))
	mux.Handle("PUT /admin/channels/{id}/routes/{routeId}", s.permission("routes.manage", s.updateChannelRoute))
	mux.Handle("DELETE /admin/channels/{id}/routes/{routeId}", s.permission("routes.manage", s.deleteChannelRoute))
	mux.Handle("GET /admin/channels/{id}/quota", s.permission("channels.read", s.getChannelQuotaHandler))
	mux.Handle("POST /admin/channels/{id}/quota", s.permission("channels.manage", s.upsertChannelQuotaHandler))
	mux.Handle("DELETE /admin/channels/{id}/quota", s.permission("channels.manage", s.deleteChannelQuotaHandler))
	mux.Handle("GET /admin/channels/{id}/usage-stats", s.permission("channels.read", s.channelUsageStatsHandler))
	mux.Handle("POST /admin/quota-limits", s.permission("quotas.manage", s.upsertQuota))
	mux.Handle("GET /admin/quota-limits", s.permission("quotas.manage", s.listQuotaLimits))
	mux.Handle("DELETE /admin/quota-limits/{id}", s.permission("quotas.manage", s.deleteQuotaLimit))
	mux.Handle("POST /admin/migrate", s.permission("system.manage", s.runMigration))
	mux.Handle("GET /admin/migrate", s.permission("system.manage", s.getMigrationStatus))
	mux.Handle("GET /admin/migrate/requests", s.permission("system.manage", s.listMigrationRequests))
	mux.Handle("GET /admin/oauth/providers", s.permission("system.manage", s.listOAuthProviders))
	mux.Handle("POST /admin/oauth/providers/{provider}", s.permission("system.manage", s.upsertOAuthProvider))
	mux.Handle("DELETE /admin/oauth/providers/{provider}", s.permission("system.manage", s.deleteOAuthProvider))
	mux.Handle("GET /me", s.api(s.me))
	mux.Handle("GET /me/keys", s.api(s.myKeys))
	mux.Handle("GET /me/usage", s.api(s.myUsage))
	mux.Handle("GET /me/ledger", s.api(s.myLedger))
	mux.Handle("GET /me/groups", s.api(s.myGroups))
	mux.Handle("GET /v1/models", s.api(s.models))
	mux.Handle("POST /v1/chat/completions", s.api(s.chatCompletions))
	mux.Handle("POST /v1/responses", s.api(s.responsesCompletions))
	mux.Handle("POST /v1/messages", s.api(s.anthropicMessages))
	return recoverPanic(securityHeaders(s.requestID(mux)))
}
func (s *Service) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := randomID()
		if err != nil {
			writeError(w, 500, "internal_error", "could not create request id")
			return
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		meta := requestMetadata(r)
		ua := meta.userAgent
		if len(ua) > 256 {
			ua = ua[:256]
		}
		ctx = context.WithValue(ctx, clientInfoKey{}, clientInfo{ip: meta.clientIP, userAgent: ua})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type requestIDKey struct{}

type clientInfoKey struct{}

type clientInfo struct {
	ip        string
	userAgent string
}

func clientInfoFromContext(ctx context.Context) clientInfo {
	info, _ := ctx.Value(clientInfoKey{}).(clientInfo)
	return info
}

func requestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }
func (s *Service) api(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeError(w, 401, "invalid_api_key", "API key required")
			return
		}
		var k keyContext
		err := s.db.QueryRow(r.Context(), `select k.user_id,k.id,coalesce(k.group_id::text,''),u.data_usage_enabled from api_keys k join users u on u.id=k.user_id where k.secret_hash=$1 and k.revoked_at is null and (k.expires_at is null or k.expires_at>now()) and u.enabled and (k.group_id is null or exists(select 1 from groups g where g.id=k.group_id and g."public") or exists(select 1 from user_groups ug where ug.user_id=k.user_id and ug.group_id=k.group_id))`, hashSecret(token)).Scan(&k.userID, &k.keyID, &k.groupID, &k.dataUsageEnabled)
		if err != nil {
			writeError(w, 401, "invalid_api_key", "invalid or expired API key")
			return
		}
		if !s.limiter.allow(k.keyID) {
			writeError(w, 429, "rate_limit_exceeded", "too many requests")
			return
		}
		s.touchAPIKey(k.keyID)
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, k)))
	})
}

// touchAPIKey refreshes api_keys.last_used_at at most once per keyTouchInterval per key,
// in the background. Writing it on every request turned a single row into a contended
// write hotspot for busy keys; the stamp may now trail real usage by that interval.
func (s *Service) touchAPIKey(keyID string) {
	if !s.keyTouchCache.storeOnce(keyID, struct{}{}) {
		return
	}
	s.background.submit(func(ctx context.Context) {
		_, _ = s.db.Exec(ctx, `update api_keys set last_used_at=now() where id=$1`, keyID)
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
	rows, err := s.db.Query(r.Context(), `select k.id,k.name,k.key_prefix,k.expires_at,k.revoked_at,k.last_used_at,k.created_at,coalesce(k.group_id::text,''),coalesce(g.name,''),k.secret_encrypted<>'' from api_keys k left join groups g on g.id=k.group_id where k.user_id=$1 order by k.created_at desc`, key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, prefix, groupID, groupName string
		var revealable bool
		var expires, revoked, used, created any
		if rows.Scan(&id, &name, &prefix, &expires, &revoked, &used, &created, &groupID, &groupName, &revealable) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "key_prefix": prefix, "group_id": groupID, "group_name": groupName, "expires_at": expires, "revoked_at": revoked, "last_used_at": used, "created_at": created, "revealable": revealable})
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
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(target)
}

var (
	errInvalid            = errors.New("invalid request")
	errPricingUnavailable = errors.New("pricing unavailable")
	// errChannelCredentials means enabled channels matched the model but none had a
	// usable API key (decryption failure or no key), so the failure is not "no channel".
	errChannelCredentials = errors.New("channel credentials unavailable")
)

func parseExpiry(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	return &t, err
}
