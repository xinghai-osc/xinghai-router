package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Admin: per-user subscription management ----

func (s *Service) adminUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var userExists bool
	if err := s.db.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, userID).Scan(&userExists); err != nil || !userExists {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	rows, err := s.db.Query(r.Context(), `select us.id::text,us.tier_id::text,t.subscription_id::text,s.name,t.level,coalesce(t.name,''),us.status,to_char(us.current_period_start,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.current_period_end,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),us.auto_renew,to_char(us.cancelled_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),coalesce(pr.max_requests_per_period,t.max_requests_per_period),us.remaining_requests,coalesce(pr.max_credit_per_period,t.max_credit_per_period),us.remaining_credit,coalesce(us.billing_period,'month'),coalesce(us.price::text,''),us.currency from user_subscriptions us join subscription_tiers t on t.id=us.tier_id join subscriptions s on s.id=t.subscription_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period where us.user_id=$1 order by s.sort_order,t.level,us.created_at desc`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	tierIDs := []string{}
	for rows.Next() {
		var id, tierID, productID, productName, tierName, status, billingPeriod, price, currency string
		var level int
		var start, end, cancelled, created, updated *string
		var autoRenew bool
		var maxReq, remainingReq *int64
		var maxCredit, remainingCredit *float64
		if err = rows.Scan(&id, &tierID, &productID, &productName, &level, &tierName, &status, &start, &end, &autoRenew, &cancelled, &created, &updated, &maxReq, &remainingReq, &maxCredit, &remainingCredit, &billingPeriod, &price, &currency); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		data = append(data, map[string]any{
			"id": id, "tier_id": tierID, "subscription_id": productID, "product_name": productName,
			"tier_level": level, "tier_name": tierName, "display_name": subscriptionDisplayName(productName, tierName),
			"status": status, "current_period_start": start, "current_period_end": end, "auto_renew": autoRenew,
			"cancelled_at": cancelled, "created_at": created, "updated_at": updated,
			"max_requests_per_period": maxReq, "max_credit_per_period": maxCredit,
			"remaining_requests": remainingReq, "remaining_credit": remainingCredit,
			"billing_period": billingPeriod, "price": price, "currency": currency,
			"model_usage": []map[string]any{},
		})
		tierIDs = append(tierIDs, tierID)
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	if len(data) > 0 {
		subIDs := make([]string, len(data))
		for i := range data {
			subIDs[i] = data[i]["id"].(string)
		}
		usageRows, err := s.db.Query(r.Context(), `select uq.subscription_id::text,uq.model,q.max_requests_per_period,uq.remaining_requests,q.max_credit_per_period,uq.remaining_credit from user_subscription_model_usage uq join user_subscriptions us on us.id=uq.subscription_id join subscription_tier_model_quotas q on q.tier_id=us.tier_id and q.model=uq.model where uq.subscription_id = any($1) order by uq.model`, subIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		defer usageRows.Close()
		usageMap := map[string][]map[string]any{}
		for usageRows.Next() {
			var subID, model string
			var maxReq, remainingReq *int64
			var maxCredit, remainingCredit *float64
			if err = usageRows.Scan(&subID, &model, &maxReq, &remainingReq, &maxCredit, &remainingCredit); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
				return
			}
			usageMap[subID] = append(usageMap[subID], map[string]any{"model": model, "max_requests_per_period": maxReq, "max_credit_per_period": maxCredit, "remaining_requests": remainingReq, "remaining_credit": remainingCredit})
		}
		if err = usageRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		for i := range data {
			if usage, ok := usageMap[data[i]["id"].(string)]; ok {
				data[i]["model_usage"] = usage
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Service) adminCreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var in struct {
		TierID        string `json:"tier_id"`
		BillingPeriod string `json:"billing_period"`
		PeriodDays    *int   `json:"period_days"`
		StartAt       string `json:"start_at"`
		EndAt         string `json:"end_at"`
		AutoRenew     bool   `json:"auto_renew"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	tierID := strings.TrimSpace(in.TierID)
	if tierID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "tier_id is required")
		return
	}
	billingPeriod := strings.ToLower(strings.TrimSpace(in.BillingPeriod))
	if billingPeriod == "" {
		billingPeriod = "month"
	}
	var start *time.Time
	if in.StartAt != "" {
		t, err := time.Parse(time.RFC3339, in.StartAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "start_at must be a valid timestamp")
			return
		}
		start = &t
	}
	var end *time.Time
	if in.EndAt != "" {
		t, err := time.Parse(time.RFC3339, in.EndAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "end_at must be a valid timestamp")
			return
		}
		end = &t
	}
	if start != nil && end != nil && !end.After(*start) {
		writeError(w, http.StatusBadRequest, "invalid_request", "end_at must be after start_at")
		return
	}
	if in.PeriodDays != nil && (*in.PeriodDays <= 0 || *in.PeriodDays > maxSubscriptionPeriodDays) {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("period_days must be between 1 and %d", maxSubscriptionPeriodDays))
		return
	}

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	defer tx.Rollback(r.Context())
	var userExists bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, userID).Scan(&userExists); err != nil || !userExists {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	purchase, err := s.loadTierPurchase(r.Context(), tx, tierID, billingPeriod, false)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "subscription tier not found")
		return
	}
	autoRenew := in.AutoRenew
	subID, _, err := s.activateSubscriptionTx(r.Context(), tx, userID, purchase, subscriptionActivation{
		PeriodDays:     in.PeriodDays,
		StartAt:        start,
		EndAt:          end,
		RenewAutoRenew: &autoRenew,
		ForceNew:       true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	s.audit(r, "subscription.admin_created", "user_subscription", subID, map[string]any{"user_id": userID, "tier_id": tierID, "billing_period": billingPeriod, "period_days": in.PeriodDays})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"id": subID})
}

func (s *Service) adminUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Status             *string  `json:"status"`
		CurrentPeriodStart *string  `json:"current_period_start"`
		CurrentPeriodEnd   *string  `json:"current_period_end"`
		AutoRenew          *bool    `json:"auto_renew"`
		RemainingRequests  *int64   `json:"remaining_requests"`
		RemainingCredit    *float64 `json:"remaining_credit"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	var status string
	var start, end any
	var autoRenew bool
	var maxRequests, remainingRequests *int64
	var maxCredit, remainingCredit *float64
	err := s.db.QueryRow(r.Context(), `select us.status,us.current_period_start,us.current_period_end,us.auto_renew,coalesce(pr.max_requests_per_period,t.max_requests_per_period),us.remaining_requests,coalesce(pr.max_credit_per_period,t.max_credit_per_period),us.remaining_credit from user_subscriptions us join subscription_tiers t on t.id=us.tier_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period where us.id=$1`, id).Scan(&status, &start, &end, &autoRenew, &maxRequests, &remainingRequests, &maxCredit, &remainingCredit)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription")
		return
	}
	origStatus := status
	if in.Status != nil {
		switch *in.Status {
		case "pending", "active", "expired", "cancelled":
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of pending, active, expired, cancelled")
			return
		}
		status = *in.Status
	}
	if in.CurrentPeriodStart != nil {
		if *in.CurrentPeriodStart == "" {
			start = nil
		} else {
			t, err := time.Parse(time.RFC3339, *in.CurrentPeriodStart)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "current_period_start must be a valid timestamp")
				return
			}
			start = t
		}
	}
	if in.CurrentPeriodEnd != nil {
		if *in.CurrentPeriodEnd == "" {
			end = nil
		} else {
			t, err := time.Parse(time.RFC3339, *in.CurrentPeriodEnd)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "current_period_end must be a valid timestamp")
				return
			}
			end = t
		}
	}
	startTime, endTime := adminSubscriptionTime(start), adminSubscriptionTime(end)
	if startTime != nil && endTime != nil && !endTime.After(*startTime) {
		writeError(w, http.StatusBadRequest, "invalid_request", "current_period_end must be after current_period_start")
		return
	}
	if in.AutoRenew != nil {
		autoRenew = *in.AutoRenew
	}
	if in.RemainingRequests != nil {
		if maxRequests == nil || *in.RemainingRequests < 0 || *in.RemainingRequests > *maxRequests {
			writeError(w, http.StatusBadRequest, "invalid_request", "remaining_requests must be between 0 and the period limit")
			return
		}
		remainingRequests = in.RemainingRequests
	}
	if in.RemainingCredit != nil {
		if maxCredit == nil || *in.RemainingCredit < 0 || *in.RemainingCredit > *maxCredit {
			writeError(w, http.StatusBadRequest, "invalid_request", "remaining_credit must be between 0 and the period limit")
			return
		}
		remainingCredit = in.RemainingCredit
	}
	if renewsExpiredSubscription(in.Status, origStatus, endTime, time.Now()) {
		// Moving an expired subscription's period end into the future is a
		// renewal; leaving the status expired made that edit a no-op.
		status = "active"
	}
	reactivating := status == "active" && origStatus != "active"
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `update user_subscriptions set status=$1,current_period_start=$2,current_period_end=$3,auto_renew=$4,remaining_requests=$5,remaining_credit=$6,cancelled_at=case when $8::boolean then null else cancelled_at end,updated_at=now() where id=$7`, status, start, end, autoRenew, remainingRequests, remainingCredit, id, reactivating); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	if reactivating {
		// Reactivating a subscription starts a fresh period, so refill its quota
		// counters (a subscription manually activated from pending has none yet)
		// and hand back the tier group that expiry revoked.
		if err = s.initSubscriptionCountersTx(r.Context(), tx, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
			return
		}
		if err = s.grantSubscriptionGroupTx(r.Context(), tx, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not restore subscription group")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	s.audit(r, "subscription.admin_updated", "user_subscription", id, map[string]any{"status": status, "current_period_start": start, "current_period_end": end, "remaining_requests": remainingRequests, "remaining_credit": remainingCredit})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) adminResetSubscriptionQuota(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
		return
	}
	defer tx.Rollback(r.Context())
	var exists bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from user_subscriptions where id=$1)`, id).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	if err = s.initSubscriptionCountersTx(r.Context(), tx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
		return
	}
	s.audit(r, "subscription.admin_quota_reset", "user_subscription", id, nil)
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) adminVoidSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.Exec(r.Context(), `update user_subscriptions set status='cancelled',auto_renew=false,cancelled_at=coalesce(cancelled_at,now()),updated_at=now() where id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not void subscription")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	s.audit(r, "subscription.admin_voided", "user_subscription", id, nil)
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) adminDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.Exec(r.Context(), `delete from user_subscriptions where id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete subscription")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	s.audit(r, "subscription.admin_deleted", "user_subscription", id, nil)
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- Admin: view all subscriptions ----

func (s *Service) adminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select us.id::text,us.user_id::text,u.email,u.name,us.tier_id::text,t.subscription_id::text,s.name,t.level,coalesce(t.name,''),us.status,to_char(us.current_period_start,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.current_period_end,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),us.auto_renew,to_char(us.cancelled_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),coalesce(pr.max_requests_per_period,t.max_requests_per_period),us.remaining_requests,coalesce(pr.max_credit_per_period,t.max_credit_per_period),us.remaining_credit,(select count(*) from subscription_tier_model_quotas q where q.tier_id=t.id),coalesce(us.billing_period,'month') from user_subscriptions us join users u on u.id=us.user_id join subscription_tiers t on t.id=us.tier_id join subscriptions s on s.id=t.subscription_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period order by us.created_at desc limit 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("query: %v", err))
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, userID, email, name, tierID, productID, productName, tierName, status, billingPeriod string
		var level int
		var start, end, cancelled, created, updated *string
		var autoRenew bool
		var maxReq, remainingReq *int64
		var maxCredit, remainingCredit *float64
		var modelQuotaCount int64
		if err = rows.Scan(&id, &userID, &email, &name, &tierID, &productID, &productName, &level, &tierName, &status, &start, &end, &autoRenew, &cancelled, &created, &updated, &maxReq, &remainingReq, &maxCredit, &remainingCredit, &modelQuotaCount, &billingPeriod); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("scan: %v", err))
			return
		}
		data = append(data, map[string]any{"id": id, "user_id": userID, "email": email, "user_name": name, "tier_id": tierID, "subscription_id": productID, "product_name": productName, "tier_level": level, "tier_name": tierName, "display_name": subscriptionDisplayName(productName, tierName), "plan_name": subscriptionDisplayName(productName, tierName), "status": status, "current_period_start": start, "current_period_end": end, "auto_renew": autoRenew, "cancelled_at": cancelled, "created_at": created, "updated_at": updated, "max_requests_per_period": maxReq, "max_credit_per_period": maxCredit, "remaining_requests": remainingReq, "remaining_credit": remainingCredit, "model_quota_count": modelQuotaCount, "billing_period": billingPeriod})
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("rows: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// ---- Admin: batch extend subscriptions ----

func (s *Service) batchExtendSubscriptions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TierID string `json:"tier_id"`
		PlanID string `json:"plan_id"`
		Days   int    `json:"days"`
		Status string `json:"status"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	if in.Days == 0 || in.Days < -3650 || in.Days > 3650 {
		writeError(w, http.StatusBadRequest, "invalid_request", "days must be between -3650 and 3650, excluding 0")
		return
	}
	statusClause, ok := subscriptionStatusClause(strings.ToLower(strings.TrimSpace(in.Status)))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of active, inactive, all")
		return
	}
	tierID := strings.TrimSpace(in.TierID)
	if tierID == "" {
		tierID = strings.TrimSpace(in.PlanID)
	}
	if tierID != "" {
		var tierExists bool
		if err := s.db.QueryRow(r.Context(), `select exists(select 1 from subscription_tiers where id=$1)`, tierID).Scan(&tierExists); err != nil || !tierExists {
			writeError(w, http.StatusNotFound, "not_found", "tier not found")
			return
		}
	}
	days := fmt.Sprintf("%d days", in.Days)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not extend subscriptions")
		return
	}
	defer tx.Rollback(r.Context())
	ext := `update user_subscriptions set current_period_end = case when current_period_end is null or current_period_end <= now() then now() + $1::interval else current_period_end + $1::interval end, updated_at = now() where ` + statusClause
	args := []any{days}
	if tierID != "" {
		ext += ` and tier_id::text=$2`
		args = append(args, tierID)
	}
	ext += ` returning id::text,status,current_period_end`
	rows, err := tx.Query(r.Context(), ext, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("could not extend subscriptions: %v", err))
		return
	}
	var affected int64
	var renewed []string
	for rows.Next() {
		var id, rowStatus string
		var end *time.Time
		if err = rows.Scan(&id, &rowStatus, &end); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("could not extend subscriptions: %v", err))
			return
		}
		affected++
		// Extending an expired subscription is a renewal: promote it once the
		// new period end is in the future, otherwise the extension grants nothing.
		if extendsIntoActive(rowStatus, in.Days, end, time.Now()) {
			renewed = append(renewed, id)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("could not extend subscriptions: %v", err))
		return
	}
	for _, id := range renewed {
		if err = s.reactivateSubscriptionTx(r.Context(), tx, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("could not renew subscriptions: %v", err))
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not extend subscriptions")
		return
	}
	s.audit(r, "subscription.batch_extended", "subscription_tier", tierID, map[string]any{"days": in.Days, "status": in.Status, "affected": affected, "renewed": len(renewed)})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"affected": affected, "renewed": len(renewed)})
}

// reactivateSubscriptionTx promotes a subscription to active for a fresh period:
// refill its period quota and restore the tier group that expiry revoked.
func (s *Service) reactivateSubscriptionTx(ctx context.Context, tx pgx.Tx, id string) error {
	if _, err := tx.Exec(ctx, `update user_subscriptions set status='active',cancelled_at=null,updated_at=now() where id=$1`, id); err != nil {
		return err
	}
	if err := s.initSubscriptionCountersTx(ctx, tx, id); err != nil {
		return err
	}
	return s.grantSubscriptionGroupTx(ctx, tx, id)
}

// grantSubscriptionGroupTx restores a subscription holder's membership in the
// tier's group. Expiry revokes subscription-bound groups, so a renewal has to
// put the membership back or a key that relies on it stays unable to use the tier.
func (s *Service) grantSubscriptionGroupTx(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `insert into user_groups(user_id,group_id) select us.user_id,t.group_id from user_subscriptions us join subscription_tiers t on t.id=us.tier_id where us.id=$1 and t.group_id is not null on conflict do nothing`, id)
	return err
}

func (s *Service) resetActiveSubscriptionQuotas(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if err := decode(r, &in); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	statusClause, ok := subscriptionStatusClause(strings.ToLower(strings.TrimSpace(in.Status)))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of active, inactive, all")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quotas")
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `select id::text from user_subscriptions where `+statusClause)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load active subscriptions")
		return
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load active subscriptions")
			return
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load active subscriptions")
		return
	}
	for _, id := range ids {
		if err = s.initSubscriptionCountersTx(r.Context(), tx, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quotas")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quotas")
		return
	}
	s.audit(r, "subscription.quotas_reset", "user_subscription", "", map[string]any{"status": in.Status, "affected": len(ids)})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"affected": len(ids), "status": in.Status})
}

func adminSubscriptionTime(v any) *time.Time {
	t, ok := v.(time.Time)
	if !ok {
		return nil
	}
	return &t
}

// renewsExpiredSubscription reports whether an admin edit turns an expired
// subscription back on: the caller did not choose a status itself and either
// moved the period end into the future or cleared it (no expiry).
func renewsExpiredSubscription(requestedStatus *string, currentStatus string, end *time.Time, now time.Time) bool {
	if requestedStatus != nil || currentStatus != "expired" {
		return false
	}
	return end == nil || end.After(now)
}

// extendsIntoActive reports whether a batch extension renews an expired
// subscription: a forward extension that leaves the period end in the future.
func extendsIntoActive(status string, days int, end *time.Time, now time.Time) bool {
	return days > 0 && status == "expired" && end != nil && end.After(now)
}
