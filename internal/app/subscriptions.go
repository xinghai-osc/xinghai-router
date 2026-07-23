package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type subscriptionPlan struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Price              string   `json:"price"`
	Currency           string   `json:"currency"`
	BillingPeriod      string   `json:"billing_period"`
	CreditAmount       string   `json:"credit_amount"`
	GroupID            string   `json:"group_id"`
	GroupName          string   `json:"group_name"`
	ModelWhitelist     []string `json:"model_whitelist"`
	MaxRequestsPerRule *int64   `json:"max_requests_per_period"`
	MaxTokensPerRule   *int64   `json:"max_tokens_per_period"`
	SortOrder          int      `json:"sort_order"`
	Enabled            bool     `json:"enabled"`
	CreatedAt          any      `json:"created_at"`
	UpdatedAt          any      `json:"updated_at"`
}

type userSubscription struct {
	ID                  string   `json:"id"`
	UserID              string   `json:"user_id"`
	PlanID              string   `json:"plan_id"`
	PlanName            string   `json:"plan_name"`
	Status              string   `json:"status"`
	CurrentPeriodStart  any      `json:"current_period_start"`
	CurrentPeriodEnd    any      `json:"current_period_end"`
	AutoRenew           bool     `json:"auto_renew"`
	CancelledAt         any      `json:"cancelled_at"`
	CreatedAt           any      `json:"created_at"`
	UpdatedAt           any      `json:"updated_at"`
	Price               string   `json:"price"`
	BillingPeriod       string   `json:"billing_period"`
	CreditAmount        string   `json:"credit_amount"`
	GroupID             string   `json:"group_id"`
	GroupName           string   `json:"group_name"`
	ModelWhitelist      []string `json:"model_whitelist"`
	MaxRequestsPerRule  *int64   `json:"max_requests_per_period"`
	MaxTokensPerRule    *int64   `json:"max_tokens_per_period"`
}

type subscriptionOrder struct {
	ID             string `json:"id"`
	OrderNo        string `json:"order_no"`
	SubscriptionID string `json:"subscription_id"`
	PlanID         string `json:"plan_id"`
	PlanName       string `json:"plan_name"`
	Provider       string `json:"provider"`
	PaymentType    string `json:"payment_type"`
	Amount         string `json:"amount"`
	Status         string `json:"status"`
	ProviderTrade  string `json:"provider_trade_no,omitempty"`
	PeriodKind     string `json:"period_kind"`
	PaidAt         any    `json:"paid_at"`
	CreatedAt      any    `json:"created_at"`
}

// ---- Admin: plan management ----

func (s *Service) listSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select p.id,p.name,p.description,p.price::text,p.currency,p.billing_period,p.credit_amount::text,coalesce(p.group_id::text,''),coalesce(g.name,''),p.model_whitelist,p.max_requests_per_period,p.max_tokens_per_period,p.sort_order,p.enabled,p.created_at,p.updated_at from subscription_plans p left join groups g on g.id=p.group_id order by p.sort_order,p.name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
		return
	}
	defer rows.Close()
	plans := []subscriptionPlan{}
	for rows.Next() {
		var plan subscriptionPlan
		var groupID, groupName string
		var maxReq, maxTok *int64
		var models []string
		if err = rows.Scan(&plan.ID, &plan.Name, &plan.Description, &plan.Price, &plan.Currency, &plan.BillingPeriod, &plan.CreditAmount, &groupID, &groupName, &models, &maxReq, &maxTok, &plan.SortOrder, &plan.Enabled, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
			return
		}
		plan.GroupID, plan.GroupName = groupID, groupName
		plan.ModelWhitelist = models
		if plan.ModelWhitelist == nil {
			plan.ModelWhitelist = []string{}
		}
		plan.MaxRequestsPerRule = maxReq
		plan.MaxTokensPerRule = maxTok
		plans = append(plans, plan)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": plans})
}

func (s *Service) publicSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select p.id,p.name,p.description,p.price::text,p.currency,p.billing_period,p.credit_amount::text,coalesce(g.name,''),p.model_whitelist,p.sort_order from subscription_plans p left join groups g on g.id=p.group_id where p.enabled order by p.sort_order,p.name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
		return
	}
	defer rows.Close()
	plans := []map[string]any{}
	for rows.Next() {
		var id, name, description, price, currency, billing, credit, groupName string
		var models []string
		var sortOrder int
		if err = rows.Scan(&id, &name, &description, &price, &currency, &billing, &credit, &groupName, &models, &sortOrder); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
			return
		}
		if models == nil {
			models = []string{}
		}
		plans = append(plans, map[string]any{"id": id, "name": name, "description": description, "price": price, "currency": currency, "billing_period": billing, "credit_amount": credit, "group_name": groupName, "model_whitelist": models, "sort_order": sortOrder})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": plans})
}

func (s *Service) createSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := readSubscriptionPlanInput(r, s, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create plan")
		return
	}
	_, err = s.db.Exec(r.Context(), `insert into subscription_plans(id,name,description,price,currency,billing_period,credit_amount,group_id,model_whitelist,max_requests_per_period,max_tokens_per_period,sort_order,enabled) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id, plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingPeriod, plan.CreditAmount, nullableGroupRef(plan.GroupID), plan.ModelWhitelist, plan.MaxRequestsPerRule, plan.MaxTokensPerRule, plan.SortOrder, plan.Enabled)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "plan name already exists")
		return
	}
	s.audit(r, "subscription_plan.created", "subscription_plan", id, map[string]any{"name": plan.Name, "price": plan.Price, "billing_period": plan.BillingPeriod})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) updateSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := readSubscriptionPlanInput(r, s, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := s.db.Exec(r.Context(), `update subscription_plans set name=$1,description=$2,price=$3,currency=$4,billing_period=$5,credit_amount=$6,group_id=$7,model_whitelist=$8,max_requests_per_period=$9,max_tokens_per_period=$10,sort_order=$11,enabled=$12,updated_at=now() where id=$13`,
		plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingPeriod, plan.CreditAmount, nullableGroupRef(plan.GroupID), plan.ModelWhitelist, plan.MaxRequestsPerRule, plan.MaxTokensPerRule, plan.SortOrder, plan.Enabled, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "plan name already exists")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	s.audit(r, "subscription_plan.updated", "subscription_plan", r.PathValue("id"), map[string]any{"name": plan.Name, "enabled": plan.Enabled})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) deleteSubscriptionPlan(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `delete from subscription_plans where id=$1`, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete plan")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	s.audit(r, "subscription_plan.deleted", "subscription_plan", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}

func readSubscriptionPlanInput(r *http.Request, s *Service, existingID string) (subscriptionPlan, error) {
	var in struct {
		Name            string   `json:"name"`
		Description     string   `json:"description"`
		Price           string   `json:"price"`
		Currency        string   `json:"currency"`
		BillingPeriod   string   `json:"billing_period"`
		CreditAmount    string   `json:"credit_amount"`
		GroupID         string   `json:"group_id"`
		ModelWhitelist  []string `json:"model_whitelist"`
		MaxRequests     *int64   `json:"max_requests_per_period"`
		MaxTokens       *int64   `json:"max_tokens_per_period"`
		SortOrder       int      `json:"sort_order"`
		Enabled         *bool    `json:"enabled"`
	}
	if decode(r, &in) != nil {
		return subscriptionPlan{}, fmt.Errorf("invalid plan payload")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 100 {
		return subscriptionPlan{}, fmt.Errorf("name must be 1-100 characters")
	}
	in.Description = strings.TrimSpace(in.Description)
	if len(in.Description) > maxPlanDescriptionLength {
		return subscriptionPlan{}, fmt.Errorf("description must be at most 2000 characters")
	}
	in.BillingPeriod = strings.ToLower(strings.TrimSpace(in.BillingPeriod))
	if in.BillingPeriod != "month" && in.BillingPeriod != "year" {
		return subscriptionPlan{}, fmt.Errorf("billing_period must be month or year")
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "CNY"
	}
	if len(in.Currency) > 8 {
		return subscriptionPlan{}, fmt.Errorf("currency code too long")
	}
	priceCents, _, ok := parsePaymentAmount(in.Price)
	if !ok || priceCents < 0 || priceCents > maxPlanPriceCents {
		return subscriptionPlan{}, fmt.Errorf("price must be a non-negative decimal up to 100000.00")
	}
	credit, ok := parseCreditAmount(in.CreditAmount)
	if !ok || credit < 0 || credit > maxPlanCreditAmount {
		return subscriptionPlan{}, fmt.Errorf("credit_amount must be a non-negative decimal up to 1000000")
	}
	if in.SortOrder < minPlanSortOrder || in.SortOrder > maxPlanSortOrder {
		return subscriptionPlan{}, fmt.Errorf("sort_order must be between -10000 and 10000")
	}
	if !validQuotaLimit(in.MaxRequests) || !validQuotaLimit(in.MaxTokens) {
		return subscriptionPlan{}, fmt.Errorf("period limits must be between 0 and 1e12")
	}
	groupRef := strings.TrimSpace(in.GroupID)
	if groupRef != "" {
		var exists bool
		if s.db.QueryRow(r.Context(), `select exists(select 1 from groups where id=$1 or name=$2)`, groupRef, groupRef).Scan(&exists) != nil || !exists {
			return subscriptionPlan{}, fmt.Errorf("group does not exist")
		}
		var groupID string
		_ = s.db.QueryRow(r.Context(), `select id from groups where id=$1 or name=$2`, groupRef, groupRef).Scan(&groupID)
		groupRef = groupID
	}
	models := in.ModelWhitelist
	if models == nil {
		models = []string{}
	}
	for i, m := range models {
		models[i] = strings.TrimSpace(m)
		if models[i] == "" || len(models[i]) > 200 {
			return subscriptionPlan{}, fmt.Errorf("invalid model whitelist entry")
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return subscriptionPlan{
		Name:               in.Name,
		Description:        in.Description,
		Price:              formatAmount(priceCents),
		Currency:           in.Currency,
		BillingPeriod:      in.BillingPeriod,
		CreditAmount:       formatCredit(credit),
		GroupID:            groupRef,
		ModelWhitelist:     models,
		MaxRequestsPerRule: in.MaxRequests,
		MaxTokensPerRule:   in.MaxTokens,
		SortOrder:          in.SortOrder,
		Enabled:            enabled,
	}, nil
}

const (
	maxPlanDescriptionLength = 2000
	maxPlanPriceCents        = maxPaymentCents
	maxPlanCreditAmount      = 1_000_000.0
	minPlanSortOrder         = -10000
	maxPlanSortOrder         = 10000
)

func nullableGroupRef(ref string) any {
	if ref == "" {
		return nil
	}
	return ref
}

func parseCreditAmount(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	cents, _, ok := parsePaymentAmount(value)
	if !ok {
		return 0, false
	}
	return float64(cents) / 100, true
}

func formatAmount(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func formatCredit(value float64) string {
	return fmt.Sprintf("%g", value)
}

// ---- Account: browse plans & subscriptions ----

func (s *Service) accountSubscriptions(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select us.id,us.user_id,us.plan_id,p.name,us.status,us.current_period_start,us.current_period_end,us.auto_renew,us.cancelled_at,us.created_at,us.updated_at,p.price::text,p.billing_period,p.credit_amount::text,coalesce(p.group_id::text,''),coalesce(g.name,''),p.model_whitelist,p.max_requests_per_period,p.max_tokens_per_period from user_subscriptions us join subscription_plans p on p.id=us.plan_id left join groups g on g.id=p.group_id where us.user_id=$1 order by us.created_at desc`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	subs := []userSubscription{}
	for rows.Next() {
		var sub userSubscription
		var groupID, groupName string
		var models []string
		var maxReq, maxTok *int64
		if err = rows.Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.PlanName, &sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.AutoRenew, &sub.CancelledAt, &sub.CreatedAt, &sub.UpdatedAt, &sub.Price, &sub.BillingPeriod, &sub.CreditAmount, &groupID, &groupName, &models, &maxReq, &maxTok); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		sub.GroupID, sub.GroupName = groupID, groupName
		sub.ModelWhitelist = models
		if sub.ModelWhitelist == nil {
			sub.ModelWhitelist = []string{}
		}
		sub.MaxRequestsPerRule = maxReq
		sub.MaxTokensPerRule = maxTok
		subs = append(subs, sub)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": subs})
}

func (s *Service) accountSubscriptionOrders(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select o.id,o.order_no,o.subscription_id,o.plan_id,p.name,o.provider,o.payment_type,o.amount::text,o.status,coalesce(o.provider_trade_no,''),o.period_kind,o.paid_at,o.created_at from subscription_orders o join subscription_plans p on p.id=o.plan_id where o.user_id=$1 order by o.created_at desc limit 50`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	defer rows.Close()
	orders := []subscriptionOrder{}
	for rows.Next() {
		var order subscriptionOrder
		if err = rows.Scan(&order.ID, &order.OrderNo, &order.SubscriptionID, &order.PlanID, &order.PlanName, &order.Provider, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTrade, &order.PeriodKind, &order.PaidAt, &order.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
			return
		}
		orders = append(orders, order)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": orders})
}

func (s *Service) createSubscription(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	settings, err := s.loadEpaySettings(r)
	if err != nil || !settings.ready() {
		writeError(w, http.StatusServiceUnavailable, "payment_unavailable", "online payment is not configured")
		return
	}
	var in struct {
		PlanID      string `json:"plan_id"`
		PaymentType string `json:"payment_type"`
		AutoRenew   bool   `json:"auto_renew"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.PlanID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "plan_id is required")
		return
	}
	paymentType := strings.ToLower(strings.TrimSpace(in.PaymentType))
	if !methodEnabled(settings.Methods, paymentType) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payment type")
		return
	}
	var planID, planName string
	var price int64
	var priceStr string
	var billing string
	var enabled bool
	err = s.db.QueryRow(r.Context(), `select id,name,(price*100)::bigint,price::text,billing_period,enabled from subscription_plans where id=$1`, in.PlanID).Scan(&planID, &planName, &price, &priceStr, &billing, &enabled)
	if err != nil || !enabled {
		writeError(w, http.StatusNotFound, "not_found", "plan not found or disabled")
		return
	}
	if price <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "plan requires a positive price")
		return
	}
	subID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	orderID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	randomPart := strings.ReplaceAll(orderID, "-", "")[:12]
	orderNo := fmt.Sprintf("xhsub%d%s", time.Now().UnixMilli(), randomPart)
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	defer tx.Rollback(r.Context())
	periodKind := "new"
	var existingActiveID string
	_ = tx.QueryRow(r.Context(), `select id from user_subscriptions where user_id=$1 and plan_id=$2 and status='active' and (current_period_end is null or current_period_end > now()) limit 1`, account.userID, planID).Scan(&existingActiveID)
	if existingActiveID != "" {
		periodKind = "renewal"
	}
	if _, err = tx.Exec(r.Context(), `insert into user_subscriptions(id,user_id,plan_id,status,auto_renew) values($1,$2,$3,'pending',$4)`, subID, account.userID, planID, in.AutoRenew); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into subscription_orders(id,order_no,subscription_id,user_id,plan_id,provider,payment_type,amount,period_kind) values($1,$2,$3,$4,$5,'epay',$6,$7,$8)`, orderID, orderNo, subID, account.userID, planID, paymentType, priceStr, periodKind); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	params := url.Values{
		"pid":          {settings.MerchantID},
		"type":         {paymentType},
		"out_trade_no": {orderNo},
		"notify_url":   {settings.PublicBaseURL + "/api/payments/epay/notify"},
		"return_url":   {settings.PublicBaseURL + "/console/subscriptions?order=" + url.QueryEscape(orderNo)},
		"name":         {"Xinghai subscription: " + planName},
		"money":        {priceStr},
	}
	params.Set("sign", epaySign(params, settings.MerchantKey))
	params.Set("sign_type", "MD5")
	writeJSON(w, http.StatusCreated, map[string]any{"order_no": orderNo, "amount": priceStr, "status": "pending", "pay_url": settings.BaseURL + "/submit.php?" + params.Encode()})
}

func (s *Service) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	result, err := s.db.Exec(r.Context(), `update user_subscriptions set auto_renew=false,status=case when status='active' then 'active' else status end, cancelled_at=now(), updated_at=now() where id=$1 and user_id=$2 and status in ('active','pending')`, r.PathValue("id"), account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not cancel subscription")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	s.audit(r, "subscription.cancelled", "user_subscription", r.PathValue("id"), nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ---- Admin: view all subscriptions ----

func (s *Service) adminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select us.id,us.user_id,u.email,u.name,us.plan_id,p.name,us.status,us.current_period_start,us.current_period_end,us.auto_renew,us.cancelled_at,us.created_at,us.updated_at from user_subscriptions us join users u on u.id=us.user_id join subscription_plans p on p.id=us.plan_id order by us.created_at desc limit 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, userID, email, name, planID, planName, status string
		var autoRenew bool
		var start, end, cancelled, created, updated any
		if err = rows.Scan(&id, &userID, &email, &name, &planID, &planName, &status, &start, &end, &autoRenew, &cancelled, &created, &updated); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		data = append(data, map[string]any{"id": id, "user_id": userID, "email": email, "user_name": name, "plan_id": planID, "plan_name": planName, "status": status, "current_period_start": start, "current_period_end": end, "auto_renew": autoRenew, "cancelled_at": cancelled, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// ---- Admin: batch extend subscriptions ----

func (s *Service) batchExtendSubscriptions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlanID string `json:"plan_id"`
		Days   int    `json:"days"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	if in.Days <= 0 || in.Days > 3650 {
		writeError(w, http.StatusBadRequest, "invalid_request", "days must be between 1 and 3650")
		return
	}
	planID := strings.TrimSpace(in.PlanID)
	if planID != "" {
		var planExists bool
		if err := s.db.QueryRow(r.Context(), `select exists(select 1 from subscription_plans where id=$1)`, planID).Scan(&planExists); err != nil || !planExists {
			writeError(w, http.StatusNotFound, "not_found", "plan not found")
			return
		}
	}
	const ext = `update user_subscriptions set current_period_end = case when current_period_end is null or current_period_end <= now() then now() + ($1 || ' days')::interval else current_period_end + ($1 || ' days')::interval end, updated_at = now() where status='active'`
	var result pgconn.CommandTag
	var err error
	if planID != "" {
		result, err = s.db.Exec(r.Context(), ext+` and plan_id=$2`, in.Days, planID)
	} else {
		result, err = s.db.Exec(r.Context(), ext, in.Days)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not extend subscriptions")
		return
	}
	affected := result.RowsAffected()
	s.audit(r, "subscription.batch_extended", "subscription_plan", planID, map[string]any{"days": in.Days, "affected": affected})
	writeJSON(w, http.StatusOK, map[string]any{"affected": affected})
}

func (s *Service) accountSubscriptionOrder(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var order subscriptionOrder
	err := s.db.QueryRow(r.Context(), `select o.id,o.order_no,o.subscription_id,o.plan_id,p.name,o.provider,o.payment_type,o.amount::text,o.status,coalesce(o.provider_trade_no,''),o.period_kind,o.paid_at,o.created_at from subscription_orders o join subscription_plans p on p.id=o.plan_id where o.order_no=$1 and o.user_id=$2`, r.PathValue("order_no"), account.userID).Scan(&order.ID, &order.OrderNo, &order.SubscriptionID, &order.PlanID, &order.PlanName, &order.Provider, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTrade, &order.PeriodKind, &order.PaidAt, &order.CreatedAt)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load order")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// activateSubscriptionOrderTx activates the subscription order identified by orderNo
// within the supplied transaction. It is a no-op if the order does not exist,
// belongs to a different provider, or is already paid. The tradeNo is recorded
// for reconciliation. Returns true when the order was activated by this call.
func (s *Service) activateSubscriptionOrderTx(ctx context.Context, tx pgx.Tx, orderNo, tradeNo, notifiedAmount string) (bool, error) {
	var id, subID, userID, planID, status, amountStr string
	err := tx.QueryRow(ctx, `select id,subscription_id,user_id,plan_id,status,amount::text from subscription_orders where order_no=$1 and provider='epay' for update`, orderNo).Scan(&id, &subID, &userID, &planID, &status, &amountStr)
	if err != nil {
		return false, nil
	}
	if amountStr != notifiedAmount {
		return false, fmt.Errorf("subscription order amount mismatch")
	}
	if status == "paid" {
		return false, nil
	}
	if status != "pending" {
		return false, fmt.Errorf("subscription order not pending: %s", status)
	}
	var billing string
	var creditStr string
	var groupID any
	var modelWhitelist []string
	var maxReq, maxTok *int64
	if err = tx.QueryRow(ctx, `select billing_period,credit_amount::text,group_id,model_whitelist,max_requests_per_period,max_tokens_per_period from subscription_plans where id=$1`, planID).Scan(&billing, &creditStr, &groupID, &modelWhitelist, &maxReq, &maxTok); err != nil {
		return false, err
	}
	periodStart := time.Now()
	periodEnd := periodStart.AddDate(0, map[string]int{"month": 1, "year": 1}[billing], 0)
	if _, err = tx.Exec(ctx, `update subscription_orders set status='paid',provider_trade_no=$1,paid_at=now(),updated_at=now() where id=$2`, tradeNo, id); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `update user_subscriptions set status='active',current_period_start=$1,current_period_end=$2,auto_renew=true,updated_at=now() where id=$3 and status in ('pending','active')`, periodStart, periodEnd, subID); err != nil {
		return false, err
	}
	if credit, ok := parseCreditAmount(creditStr); ok && credit > 0 {
		if _, err = tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, userID); err != nil {
			return false, err
		}
		var balanceStr string
		if err = tx.QueryRow(ctx, `update user_wallets set balance=balance+$1::numeric,updated_at=now() where user_id=$2 returning balance::text`, credit, userID).Scan(&balanceStr); err != nil {
			return false, err
		}
		ledgerID, err := randomID()
		if err != nil {
			return false, err
		}
		if _, err = tx.Exec(ctx, `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,'subscription_topup',$5,$6)`, ledgerID, userID, credit, balanceStr, orderNo, "Subscription credit"); err != nil {
			return false, err
		}
	}
	if groupID != nil {
		var gid string
		if v, ok := groupID.(string); ok {
			gid = v
		}
		if gid != "" {
			if _, err = tx.Exec(ctx, `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`, userID, gid); err != nil {
				return false, err
			}
		}
	}
	_ = maxReq
	_ = maxTok
	_ = modelWhitelist
	return true, nil
}

// subscriptionCoversModel reports whether the user has an active subscription whose plan
// whitelists the requested model (empty whitelist = all models) and whose per-period
// request/token limits have not been reached.
func (s *Service) subscriptionCoversModel(ctx context.Context, userID, model string) bool {
	rows, err := s.db.Query(ctx, `select us.id, p.model_whitelist, p.max_requests_per_period, p.max_tokens_per_period from user_subscriptions us join subscription_plans p on p.id=us.plan_id where us.user_id=$1 and us.status='active' and us.current_period_end > now()`, userID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var subID string
		var whitelist []string
		var maxReq, maxTok *int64
		if err := rows.Scan(&subID, &whitelist, &maxReq, &maxTok); err != nil {
			continue
		}
		covered := len(whitelist) == 0
		if !covered {
			for _, m := range whitelist {
				if m == model {
					covered = true
					break
				}
			}
		}
		if !covered {
			continue
		}
		if maxReq == nil && maxTok == nil {
			return true
		}
		var count, tokens int64
		_ = s.db.QueryRow(ctx, `select count(*), coalesce(sum(total_tokens),0) from request_logs where user_id=$1 and created_at >= (select current_period_start from user_subscriptions where id=$2)`, userID, subID).Scan(&count, &tokens)
		if (maxReq == nil || count < *maxReq) && (maxTok == nil || tokens < *maxTok) {
			return true
		}
	}
	return false
}
