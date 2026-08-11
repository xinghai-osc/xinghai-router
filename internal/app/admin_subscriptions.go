package app

import (
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
	rows, err := s.db.Query(r.Context(), `select us.id::text,us.plan_id::text,p.name,us.status,to_char(us.current_period_start,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.current_period_end,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),us.auto_renew,to_char(us.cancelled_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),p.max_requests_per_period,us.remaining_requests,p.max_credit_per_period,us.remaining_credit from user_subscriptions us join subscription_plans p on p.id=us.plan_id where us.user_id=$1 order by us.created_at desc`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, planID, planName, status string
		var start, end, cancelled, created, updated *string
		var autoRenew bool
		var maxReq, remainingReq *int64
		var maxCredit, remainingCredit *float64
		if err = rows.Scan(&id, &planID, &planName, &status, &start, &end, &autoRenew, &cancelled, &created, &updated, &maxReq, &remainingReq, &maxCredit, &remainingCredit); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		data = append(data, map[string]any{"id": id, "plan_id": planID, "plan_name": planName, "status": status, "current_period_start": start, "current_period_end": end, "auto_renew": autoRenew, "cancelled_at": cancelled, "created_at": created, "updated_at": updated, "max_requests_per_period": maxReq, "max_credit_per_period": maxCredit, "remaining_requests": remainingReq, "remaining_credit": remainingCredit, "model_usage": []map[string]any{}})
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
		usageRows, err := s.db.Query(r.Context(), `select uq.subscription_id::text,uq.model,q.max_requests_per_period,uq.remaining_requests,q.max_credit_per_period,uq.remaining_credit from user_subscription_model_usage uq join user_subscriptions us on us.id=uq.subscription_id join subscription_plan_model_quotas q on q.plan_id=us.plan_id and q.model=uq.model where uq.subscription_id = any($1) order by uq.model`, subIDs)
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
		PlanID    string `json:"plan_id"`
		StartAt   string `json:"start_at"`
		EndAt     string `json:"end_at"`
		AutoRenew bool   `json:"auto_renew"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	planID := strings.TrimSpace(in.PlanID)
	if planID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "plan_id is required")
		return
	}
	start := time.Now()
	if in.StartAt != "" {
		t, err := time.Parse(time.RFC3339, in.StartAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "start_at must be a valid timestamp")
			return
		}
		start = t
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
	if end != nil && !end.After(start) {
		writeError(w, http.StatusBadRequest, "invalid_request", "end_at must be after start_at")
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
	var billing, creditStr, groupID string
	err = tx.QueryRow(r.Context(), `select billing_period,coalesce(credit_amount::text,''),coalesce(group_id::text,'') from subscription_plans where id=$1`, planID).Scan(&billing, &creditStr, &groupID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load plan")
		return
	}
	subID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	periodEnd := end
	if periodEnd == nil {
		e := subscriptionPeriodEnd(start, billing)
		periodEnd = &e
	}
	if _, err = tx.Exec(r.Context(), `insert into user_subscriptions(id,user_id,plan_id,status,current_period_start,current_period_end,auto_renew) values($1,$2,$3,'active',$4,$5,$6)`, subID, userID, planID, start, *periodEnd, in.AutoRenew); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if err = s.initSubscriptionCountersTx(r.Context(), tx, subID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not initialise subscription quota")
		return
	}
	if credit, ok := parseCreditAmount(creditStr); ok && credit > 0 {
		if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict do nothing`, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not credit wallet")
			return
		}
		var balanceStr string
		if err = tx.QueryRow(r.Context(), `update user_wallets set balance=balance+$1::numeric,updated_at=now() where user_id=$2 returning balance::text`, credit, userID).Scan(&balanceStr); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not credit wallet")
			return
		}
		ledgerID, err := randomID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not credit wallet")
			return
		}
		if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,'subscription_topup',$5,$6)`, ledgerID, userID, credit, balanceStr, subID, "Admin grant"); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not credit wallet")
			return
		}
	}
	if groupID != "" {
		if _, err = tx.Exec(r.Context(), `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`, userID, groupID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not assign group")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	s.audit(r, "subscription.admin_created", "user_subscription", subID, map[string]any{"user_id": userID, "plan_id": planID, "start": start.Format(time.RFC3339), "end": periodEnd.Format(time.RFC3339)})
	writeJSON(w, http.StatusOK, map[string]any{"id": subID})
}

func (s *Service) adminUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Status             *string `json:"status"`
		CurrentPeriodStart *string `json:"current_period_start"`
		CurrentPeriodEnd   *string `json:"current_period_end"`
		AutoRenew          *bool   `json:"auto_renew"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	var status string
	var start, end any
	var autoRenew bool
	err := s.db.QueryRow(r.Context(), `select status,current_period_start,current_period_end,auto_renew from user_subscriptions where id=$1`, id).Scan(&status, &start, &end, &autoRenew)
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
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `update user_subscriptions set status=$1,current_period_start=$2,current_period_end=$3,auto_renew=$4,updated_at=now() where id=$5`, status, start, end, autoRenew, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	if status == "active" && origStatus != "active" {
		// Reactivating a subscription starts a fresh period, so refill its quota
		// counters (a subscription manually activated from pending has none yet).
		if err = s.initSubscriptionCountersTx(r.Context(), tx, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	s.audit(r, "subscription.admin_updated", "user_subscription", id, map[string]any{"status": status, "current_period_start": start, "current_period_end": end})
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func adminSubscriptionTime(v any) *time.Time {
	t, ok := v.(time.Time)
	if !ok {
		return nil
	}
	return &t
}
