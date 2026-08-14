package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type subscriptionPlan struct {
	ID                 string                       `json:"id"`
	Name               string                       `json:"name"`
	Description        string                       `json:"description"`
	Price              string                       `json:"price"`
	Currency           string                       `json:"currency"`
	BillingPeriod      string                       `json:"billing_period"`
	CreditAmount       string                       `json:"credit_amount"`
	GroupID            string                       `json:"group_id"`
	GroupName          string                       `json:"group_name"`
	ModelWhitelist     []string                     `json:"model_whitelist"`
	MaxRequestsPerRule *int64                       `json:"max_requests_per_period"`
	MaxCreditPerRule   *float64                     `json:"max_credit_per_period"`
	OveragePolicy      string                       `json:"overage_policy"`
	ModelQuotas        []subscriptionPlanModelQuota `json:"model_quotas"`
	SortOrder          int                          `json:"sort_order"`
	Enabled            bool                         `json:"enabled"`
	CreatedAt          any                          `json:"created_at"`
	UpdatedAt          any                          `json:"updated_at"`
}

type subscriptionPlanModelQuota struct {
	Model              string   `json:"model"`
	MaxRequestsPerRule *int64   `json:"max_requests_per_period"`
	MaxCreditPerRule   *float64 `json:"max_credit_per_period"`
}

// subscriptionModelUsage tracks how many requests and how much credit a single
// model has consumed within the subscription's current period. It mirrors the
// per-model pools in subscriptionCoversModel so quota bars can show usage.
type subscriptionModelUsage struct {
	Model    string  `json:"model"`
	Requests int64   `json:"requests"`
	Credit   float64 `json:"credit"`
}

type userSubscription struct {
	ID                  string                       `json:"id"`
	UserID              string                       `json:"user_id"`
	PlanID              string                       `json:"plan_id"`
	PlanName            string                       `json:"plan_name"`
	Status              string                       `json:"status"`
	CurrentPeriodStart  any                          `json:"current_period_start"`
	CurrentPeriodEnd    any                          `json:"current_period_end"`
	AutoRenew           bool                         `json:"auto_renew"`
	CancelledAt         any                          `json:"cancelled_at"`
	CreatedAt           any                          `json:"created_at"`
	UpdatedAt           any                          `json:"updated_at"`
	Price               string                       `json:"price"`
	BillingPeriod       string                       `json:"billing_period"`
	CreditAmount        string                       `json:"credit_amount"`
	GroupID             string                       `json:"group_id"`
	GroupName           string                       `json:"group_name"`
	ModelWhitelist      []string                     `json:"model_whitelist"`
	MaxRequestsPerRule  *int64                       `json:"max_requests_per_period"`
	MaxCreditPerRule    *float64                     `json:"max_credit_per_period"`
	OveragePolicy       string                       `json:"overage_policy"`
	ModelQuotas         []subscriptionPlanModelQuota `json:"model_quotas"`
	UsageRequests       int64                        `json:"usage_requests"`
	UsageCredit         float64                      `json:"usage_credit"`
	ModelUsage          []subscriptionModelUsage     `json:"model_usage"`
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
	rows, err := s.db.Query(r.Context(), `select p.id,p.name,p.description,p.price::text,p.currency,p.billing_period,coalesce(p.credit_amount::text,''),coalesce(p.group_id::text,''),coalesce(coalesce(g.display_name, g.name),''),p.model_whitelist,p.max_requests_per_period,p.max_credit_per_period,p.overage_policy,p.sort_order,p.enabled,p.created_at,p.updated_at from subscription_plans p left join groups g on g.id=p.group_id order by p.sort_order,p.name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
		return
	}
	defer rows.Close()
	plans := []subscriptionPlan{}
	planIDs := []string{}
	for rows.Next() {
		var plan subscriptionPlan
		var groupID, groupName string
		var maxReq *int64
		var maxCredit *float64
		var models []string
		if err = rows.Scan(&plan.ID, &plan.Name, &plan.Description, &plan.Price, &plan.Currency, &plan.BillingPeriod, &plan.CreditAmount, &groupID, &groupName, &models, &maxReq, &maxCredit, &plan.OveragePolicy, &plan.SortOrder, &plan.Enabled, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
			return
		}
		plan.GroupID, plan.GroupName = groupID, groupName
		plan.ModelWhitelist = models
		if plan.ModelWhitelist == nil {
			plan.ModelWhitelist = []string{}
		}
		plan.MaxRequestsPerRule = maxReq
		plan.MaxCreditPerRule = maxCredit
		plan.ModelQuotas = []subscriptionPlanModelQuota{}
		plans = append(plans, plan)
		planIDs = append(planIDs, plan.ID)
	}
	if len(planIDs) > 0 {
		quotaRows, err := s.db.Query(r.Context(), `select plan_id,model,max_requests_per_period,max_credit_per_period from subscription_plan_model_quotas where plan_id = any($1) order by plan_id, model`, planIDs)
		if err == nil {
			defer quotaRows.Close()
			quotaMap := map[string][]subscriptionPlanModelQuota{}
			for quotaRows.Next() {
				var q subscriptionPlanModelQuota
				var planID string
				if err = quotaRows.Scan(&planID, &q.Model, &q.MaxRequestsPerRule, &q.MaxCreditPerRule); err == nil {
					quotaMap[planID] = append(quotaMap[planID], q)
				}
			}
			for i := range plans {
				if qs, ok := quotaMap[plans[i].ID]; ok {
					plans[i].ModelQuotas = qs
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": plans})
}

func (s *Service) publicSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select p.id,p.name,p.description,p.price::text,p.currency,p.billing_period,coalesce(p.credit_amount::text,''),coalesce(coalesce(g.display_name, g.name),''),p.model_whitelist,p.max_requests_per_period,p.max_credit_per_period,p.overage_policy,p.sort_order from subscription_plans p left join groups g on g.id=p.group_id where p.enabled order by p.sort_order,p.name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
		return
	}
	defer rows.Close()
	plans := []map[string]any{}
	planIDs := []string{}
	for rows.Next() {
		var id, name, description, price, currency, billing, credit, groupName, overage string
		var models []string
		var maxReq *int64
		var maxCredit *float64
		var sortOrder int
		if err = rows.Scan(&id, &name, &description, &price, &currency, &billing, &credit, &groupName, &models, &maxReq, &maxCredit, &overage, &sortOrder); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load plans")
			return
		}
		if models == nil {
			models = []string{}
		}
		plans = append(plans, map[string]any{"id": id, "name": name, "description": description, "price": price, "currency": currency, "billing_period": billing, "credit_amount": credit, "group_name": groupName, "model_whitelist": models, "max_requests_per_period": maxReq, "max_credit_per_period": maxCredit, "overage_policy": overage, "model_quotas": []subscriptionPlanModelQuota{}, "sort_order": sortOrder})
		planIDs = append(planIDs, id)
	}
	if len(planIDs) > 0 {
		quotaRows, err := s.db.Query(r.Context(), `select plan_id,model,max_requests_per_period,max_credit_per_period from subscription_plan_model_quotas where plan_id = any($1) order by plan_id, model`, planIDs)
		if err == nil {
			defer quotaRows.Close()
			quotaMap := map[string][]subscriptionPlanModelQuota{}
			for quotaRows.Next() {
				var q subscriptionPlanModelQuota
				var planID string
				if err = quotaRows.Scan(&planID, &q.Model, &q.MaxRequestsPerRule, &q.MaxCreditPerRule); err == nil {
					quotaMap[planID] = append(quotaMap[planID], q)
				}
			}
			for i := range plans {
				if qs, ok := quotaMap[plans[i]["id"].(string)]; ok {
					plans[i]["model_quotas"] = qs
				}
			}
		}
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
	_, err = s.db.Exec(r.Context(), `insert into subscription_plans(id,name,description,price,currency,billing_period,credit_amount,group_id,model_whitelist,max_requests_per_period,max_credit_per_period,overage_policy,sort_order,enabled) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		id, plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingPeriod, nullableCredit(plan.CreditAmount), nullableGroupRef(plan.GroupID), plan.ModelWhitelist, plan.MaxRequestsPerRule, plan.MaxCreditPerRule, plan.OveragePolicy, plan.SortOrder, plan.Enabled)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "plan name already exists")
		return
	}
	if err = s.syncPlanModelQuotas(r.Context(), id, plan.ModelQuotas); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save model quotas")
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
	result, err := s.db.Exec(r.Context(), `update subscription_plans set name=$1,description=$2,price=$3,currency=$4,billing_period=$5,credit_amount=$6,group_id=$7,model_whitelist=$8,max_requests_per_period=$9,max_credit_per_period=$10,overage_policy=$11,sort_order=$12,enabled=$13,updated_at=now() where id=$14`,
		plan.Name, plan.Description, plan.Price, plan.Currency, plan.BillingPeriod, nullableCredit(plan.CreditAmount), nullableGroupRef(plan.GroupID), plan.ModelWhitelist, plan.MaxRequestsPerRule, plan.MaxCreditPerRule, plan.OveragePolicy, plan.SortOrder, plan.Enabled, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "plan name already exists")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	if err = s.syncPlanModelQuotas(r.Context(), r.PathValue("id"), plan.ModelQuotas); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save model quotas")
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
	s.subscriptionCache.clear()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) syncPlanModelQuotas(ctx context.Context, planID string, quotas []subscriptionPlanModelQuota) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `delete from subscription_plan_model_quotas where plan_id=$1`, planID); err != nil {
		return err
	}
	for _, q := range quotas {
		if _, err = tx.Exec(ctx, `insert into subscription_plan_model_quotas(plan_id,model,max_requests_per_period,max_credit_per_period) values($1,$2,$3,$4) on conflict (plan_id,model) do update set max_requests_per_period=excluded.max_requests_per_period, max_credit_per_period=excluded.max_credit_per_period`, planID, q.Model, q.MaxRequestsPerRule, q.MaxCreditPerRule); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.subscriptionCache.clear()
	return nil
}

func readSubscriptionPlanInput(r *http.Request, s *Service, existingID string) (subscriptionPlan, error) {
	var in struct {
		Name            string                       `json:"name"`
		Description     string                       `json:"description"`
		Price           string                       `json:"price"`
		Currency        string                       `json:"currency"`
		BillingPeriod   string                       `json:"billing_period"`
		CreditAmount    string                       `json:"credit_amount"`
		GroupID         string                       `json:"group_id"`
		ModelWhitelist  []string                     `json:"model_whitelist"`
		MaxRequests     *int64                       `json:"max_requests_per_period"`
		MaxCredit       *float64                     `json:"max_credit_per_period"`
		OveragePolicy   string                       `json:"overage_policy"`
		ModelQuotas     []subscriptionPlanModelQuota `json:"model_quotas"`
		SortOrder       int                          `json:"sort_order"`
		Enabled         *bool                        `json:"enabled"`
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
	switch in.BillingPeriod {
	case "hour", "day", "week", "month", "year":
	default:
		return subscriptionPlan{}, fmt.Errorf("billing_period must be one of hour, day, week, month, year")
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
	creditAmount := strings.TrimSpace(in.CreditAmount)
	credit := ""
	if creditAmount != "" {
		parsed, ok := parseCreditAmount(creditAmount)
		if !ok || parsed < 0 || parsed > maxPlanCreditAmount {
			return subscriptionPlan{}, fmt.Errorf("credit_amount must be a non-negative decimal up to 1000000")
		}
		credit = formatCredit(parsed)
	}
	if in.SortOrder < minPlanSortOrder || in.SortOrder > maxPlanSortOrder {
		return subscriptionPlan{}, fmt.Errorf("sort_order must be between -10000 and 10000")
	}
	if !validQuotaLimit(in.MaxRequests) || !validQuotaCost(in.MaxCredit) {
		return subscriptionPlan{}, fmt.Errorf("period limits must be between 0 and 1e12")
	}
	overagePolicy := strings.ToLower(strings.TrimSpace(in.OveragePolicy))
	if overagePolicy == "" {
		overagePolicy = "allow_wallet"
	}
	if overagePolicy != "allow_wallet" && overagePolicy != "block" {
		return subscriptionPlan{}, fmt.Errorf("overage_policy must be allow_wallet or block")
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
	modelQuotas := []subscriptionPlanModelQuota{}
	seenModels := map[string]bool{}
	for _, q := range in.ModelQuotas {
		modelName := strings.TrimSpace(q.Model)
		if modelName == "" || len(modelName) > 200 {
			return subscriptionPlan{}, fmt.Errorf("invalid model quota model entry")
		}
		if seenModels[modelName] {
			return subscriptionPlan{}, fmt.Errorf("duplicate model quota entry for %s", modelName)
		}
		if !validQuotaLimit(q.MaxRequestsPerRule) || !validQuotaCost(q.MaxCreditPerRule) {
			return subscriptionPlan{}, fmt.Errorf("model quota limits must be between 0 and 1e12")
		}
		if q.MaxRequestsPerRule == nil && q.MaxCreditPerRule == nil {
			return subscriptionPlan{}, fmt.Errorf("model quota for %s must set at least one limit", modelName)
		}
		seenModels[modelName] = true
		modelQuotas = append(modelQuotas, subscriptionPlanModelQuota{
			Model:              modelName,
			MaxRequestsPerRule: q.MaxRequestsPerRule,
			MaxCreditPerRule:   q.MaxCreditPerRule,
		})
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
		CreditAmount:       credit,
		GroupID:            groupRef,
		ModelWhitelist:     models,
		MaxRequestsPerRule: in.MaxRequests,
		MaxCreditPerRule:   in.MaxCredit,
		OveragePolicy:      overagePolicy,
		ModelQuotas:        modelQuotas,
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

func nullableCredit(credit string) any {
	if credit == "" {
		return nil
	}
	return credit
}

func parseCreditAmount(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 7 {
		return 0, false
	}
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 0 || len(fraction) > 2 {
			return 0, false
		}
		for _, ch := range fraction {
			if ch < '0' || ch > '9' {
				return 0, false
			}
		}
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	var cents int64
	for _, ch := range parts[0] + fraction {
		cents = cents*10 + int64(ch-'0')
	}
	return float64(cents) / 100, true
}

func formatAmount(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func formatCredit(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// ---- Account: browse plans & subscriptions ----

func (s *Service) accountSubscriptions(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select us.id,us.user_id,us.plan_id,p.name,us.status,us.current_period_start,us.current_period_end,us.auto_renew,us.cancelled_at,us.created_at,us.updated_at,p.price::text,p.billing_period,coalesce(p.credit_amount::text,''),coalesce(p.group_id::text,''),coalesce(coalesce(g.display_name, g.name),''),p.model_whitelist,p.max_requests_per_period,p.max_credit_per_period,p.overage_policy from user_subscriptions us join subscription_plans p on p.id=us.plan_id left join groups g on g.id=p.group_id where us.user_id=$1 order by us.created_at desc`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	subs := []userSubscription{}
	var planIDs []string
	for rows.Next() {
		var sub userSubscription
		var groupID, groupName string
		var models []string
		var maxReq *int64
		var maxCredit *float64
		var periodStart *time.Time
		if err = rows.Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.PlanName, &sub.Status, &periodStart, &sub.CurrentPeriodEnd, &sub.AutoRenew, &sub.CancelledAt, &sub.CreatedAt, &sub.UpdatedAt, &sub.Price, &sub.BillingPeriod, &sub.CreditAmount, &groupID, &groupName, &models, &maxReq, &maxCredit, &sub.OveragePolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		sub.CurrentPeriodStart = periodStart
		sub.GroupID, sub.GroupName = groupID, groupName
		sub.ModelWhitelist = models
		if sub.ModelWhitelist == nil {
			sub.ModelWhitelist = []string{}
		}
		sub.MaxRequestsPerRule = maxReq
		sub.MaxCreditPerRule = maxCredit
		sub.ModelQuotas = []subscriptionPlanModelQuota{}
		subs = append(subs, sub)
		planIDs = append(planIDs, sub.PlanID)
	}
	if len(planIDs) > 0 {
		quotaRows, err := s.db.Query(r.Context(), `select plan_id,model,max_requests_per_period,max_credit_per_period from subscription_plan_model_quotas where plan_id = any($1) order by plan_id, model`, planIDs)
		if err == nil {
			defer quotaRows.Close()
			quotaMap := map[string][]subscriptionPlanModelQuota{}
			for quotaRows.Next() {
				var q subscriptionPlanModelQuota
				var planID string
				if err = quotaRows.Scan(&planID, &q.Model, &q.MaxRequestsPerRule, &q.MaxCreditPerRule); err == nil {
					quotaMap[planID] = append(quotaMap[planID], q)
				}
			}
			for i := range subs {
				if qs, ok := quotaMap[subs[i].PlanID]; ok {
					subs[i].ModelQuotas = qs
				}
			}
		}
	}
	for i := range subs {
		sub := &subs[i]
		if sub.MaxRequestsPerRule != nil || sub.MaxCreditPerRule != nil {
			var remainingReq *int64
			var remainingCredit *float64
			if err := s.db.QueryRow(r.Context(), `select remaining_requests,remaining_credit from user_subscriptions where id=$1`, sub.ID).Scan(&remainingReq, &remainingCredit); err == nil {
				if sub.MaxRequestsPerRule != nil && remainingReq != nil {
					sub.UsageRequests = max(0, *sub.MaxRequestsPerRule-*remainingReq)
				}
				if sub.MaxCreditPerRule != nil && remainingCredit != nil {
					sub.UsageCredit = max(0, *sub.MaxCreditPerRule-*remainingCredit)
				}
			}
		}
		if len(sub.ModelQuotas) > 0 {
			remaining := map[string][2]any{}
			rows, err := s.db.Query(r.Context(), `select model,remaining_requests,remaining_credit from user_subscription_model_usage where subscription_id=$1`, sub.ID)
			if err == nil {
				for rows.Next() {
					var model string
					var rReq *int64
					var rCredit *float64
					if rows.Scan(&model, &rReq, &rCredit) == nil {
						remaining[model] = [2]any{rReq, rCredit}
					}
				}
				rows.Close()
			}
			for _, q := range sub.ModelQuotas {
				var usedReq int64
				var usedCredit float64
				if rem, ok := remaining[q.Model]; ok {
					if rReq, ok2 := rem[0].(*int64); ok2 && rReq != nil && q.MaxRequestsPerRule != nil {
						usedReq = max(0, *q.MaxRequestsPerRule-*rReq)
					}
					if rCredit, ok2 := rem[1].(*float64); ok2 && rCredit != nil && q.MaxCreditPerRule != nil {
						usedCredit = max(0, *q.MaxCreditPerRule-*rCredit)
					}
				}
				sub.ModelUsage = append(sub.ModelUsage, subscriptionModelUsage{Model: q.Model, Requests: usedReq, Credit: usedCredit})
			}
		}
		if sub.ModelUsage == nil {
			sub.ModelUsage = []subscriptionModelUsage{}
		}
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
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ---- Admin: view all subscriptions ----

func (s *Service) adminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select us.id::text,us.user_id::text,u.email,u.name,us.plan_id::text,p.name,us.status,to_char(us.current_period_start,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.current_period_end,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),us.auto_renew,to_char(us.cancelled_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),to_char(us.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),p.max_requests_per_period,us.remaining_requests,p.max_credit_per_period,us.remaining_credit,(select count(*) from subscription_plan_model_quotas q where q.plan_id=p.id) from user_subscriptions us join users u on u.id=us.user_id join subscription_plans p on p.id=us.plan_id order by us.created_at desc limit 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("query: %v", err))
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, userID, email, name, planID, planName, status string
		var start, end, cancelled, created, updated *string
		var autoRenew bool
		var maxReq, remainingReq *int64
		var maxCredit, remainingCredit *float64
		var modelQuotaCount int64
		if err = rows.Scan(&id, &userID, &email, &name, &planID, &planName, &status, &start, &end, &autoRenew, &cancelled, &created, &updated, &maxReq, &remainingReq, &maxCredit, &remainingCredit, &modelQuotaCount); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("scan: %v", err))
			return
		}
		data = append(data, map[string]any{"id": id, "user_id": userID, "email": email, "user_name": name, "plan_id": planID, "plan_name": planName, "status": status, "current_period_start": start, "current_period_end": end, "auto_renew": autoRenew, "cancelled_at": cancelled, "created_at": created, "updated_at": updated, "max_requests_per_period": maxReq, "max_credit_per_period": maxCredit, "remaining_requests": remainingReq, "remaining_credit": remainingCredit, "model_quota_count": modelQuotaCount})
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
	if in.Status == "" {
		in.Status = "active"
	}
	switch in.Status {
	case "active", "inactive", "all":
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of active, inactive, all")
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
	statusClause := `status='active'`
	if in.Status == "inactive" {
		statusClause = `status in ('pending','expired','cancelled')`
	} else if in.Status == "all" {
		statusClause = `status in ('pending','active','expired','cancelled')`
	}
	ext := `update user_subscriptions set current_period_end = case when current_period_end is null or current_period_end <= now() then now() + $1::interval else current_period_end + $1::interval end, updated_at = now() where ` + statusClause
	var result pgconn.CommandTag
	var err error
	days := fmt.Sprintf("%d days", in.Days)
	if planID != "" {
		result, err = s.db.Exec(r.Context(), ext+` and plan_id=$2`, days, planID)
	} else {
		result, err = s.db.Exec(r.Context(), ext, days)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("could not extend subscriptions: %v", err))
		return
	}
	affected := result.RowsAffected()
	s.audit(r, "subscription.batch_extended", "subscription_plan", planID, map[string]any{"days": in.Days, "status": in.Status, "affected": affected})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"affected": affected})
}

func (s *Service) resetActiveSubscriptionQuotas(w http.ResponseWriter, r *http.Request) {
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quotas")
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `select id::text from user_subscriptions where status='active'`)
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
	s.audit(r, "subscription.active_quotas_reset", "user_subscription", "", map[string]any{"affected": len(ids)})
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusOK, map[string]any{"affected": len(ids)})
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
	var groupID string
	if err = tx.QueryRow(ctx, `select billing_period,coalesce(credit_amount::text,''),coalesce(group_id::text,'') from subscription_plans where id=$1`, planID).Scan(&billing, &creditStr, &groupID); err != nil {
		return false, err
	}
	periodStart := time.Now()
	periodEnd := subscriptionPeriodEnd(periodStart, billing)
	if _, err = tx.Exec(ctx, `update subscription_orders set status='paid',provider_trade_no=$1,paid_at=now(),updated_at=now() where id=$2`, tradeNo, id); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `update user_subscriptions set status='active',current_period_start=$1,current_period_end=$2,auto_renew=true,updated_at=now() where id=$3 and status in ('pending','active')`, periodStart, periodEnd, subID); err != nil {
		return false, err
	}
	if err = s.initSubscriptionCountersTx(ctx, tx, subID); err != nil {
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
	if groupID != "" {
		if _, err = tx.Exec(ctx, `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`, userID, groupID); err != nil {
			return false, err
		}
	}
	return true, nil
}

// initSubscriptionCountersTx resets a subscription's remaining request/credit
// counters to the plan's per-period maxes and mirrors each per-model quota into
// user_subscription_model_usage. Called on activation and renewal so every
// period starts with a full quota. Null counters mean the dimension is uncapped.
func (s *Service) initSubscriptionCountersTx(ctx context.Context, tx pgx.Tx, subID string) error {
	var maxReq *int64
	var maxCredit *float64
	if err := tx.QueryRow(ctx, `select max_requests_per_period,max_credit_per_period from subscription_plans where id=(select plan_id from user_subscriptions where id=$1)`, subID).Scan(&maxReq, &maxCredit); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update user_subscriptions set remaining_requests=$1,remaining_credit=$2,updated_at=now() where id=$3`, maxReq, maxCredit, subID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from user_subscription_model_usage where subscription_id=$1`, subID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `select model,max_requests_per_period,max_credit_per_period from subscription_plan_model_quotas q where q.plan_id=(select plan_id from user_subscriptions where id=$1)`, subID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var model string
		var mReq *int64
		var mCredit *float64
		if err = rows.Scan(&model, &mReq, &mCredit); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `insert into user_subscription_model_usage(subscription_id,model,remaining_requests,remaining_credit) values($1,$2,$3,$4)`, subID, model, mReq, mCredit); err != nil {
			return err
		}
	}
	return rows.Err()
}

func subscriptionPeriodEnd(start time.Time, billing string) time.Time {
	switch billing {
	case "hour":
		return start.Add(time.Hour)
	case "day":
		return start.AddDate(0, 0, 1)
	case "week":
		return start.AddDate(0, 0, 7)
	case "year":
		return start.AddDate(1, 0, 0)
	default:
		return start.AddDate(0, 1, 0)
	}
}

// subscriptionAccess describes the outcome of a subscription coverage check.
type subscriptionAccess struct {
	Covered        bool
	SubscriptionID string
	OveragePolicy  string
}

// subscriptionCoversModel reports whether the user has an active subscription whose plan
// whitelists the requested model (empty whitelist = all models) and whose per-period
// request/credit counters have not run out. Quota is tracked as remaining counters on
// user_subscriptions / user_subscription_model_usage that are reset to the plan max on
// every activation and decremented once per covered request at settlement time, so this
// check is a plain counter read instead of an
// on-the-fly aggregate over request_logs. When a per-model quota override exists for the
// requested model, its counters take precedence over the plan-level counters: a null
// override dimension inherits the plan-level counter for that model's requests. A null
// current_period_end means the subscription has no expiry (e.g. migrated from a system
// that does not track one). It runs on every proxied request, so matching and the
// counter checks are pushed into a single query. The user_id comparison goes through
// ::text so it works whether the column is uuid (pre-027 databases) or bigint. The
// returned SubscriptionID is the subscription whose quota this request consumes, so the
// settle path can decrement exactly that counter. The returned OveragePolicy tells the
// gateway whether to fall through to wallet billing ('allow_wallet') or reject ('block')
// once the quota is used up.
func (s *Service) subscriptionCoversModel(ctx context.Context, userID, model string) subscriptionAccess {
	access, err := s.subscriptionCache.get(ctx, subscriptionRouteKey{userID: userID, model: model}, func(ctx context.Context) (subscriptionAccess, error) {
		return s.loadSubscriptionCoversModel(ctx, userID, model)
	})
	if err != nil {
		log.Printf("subscriptionCoversModel: %v", err)
		return subscriptionAccess{Covered: false, OveragePolicy: "allow_wallet"}
	}
	return access
}

func (s *Service) loadSubscriptionCoversModel(ctx context.Context, userID, model string) (subscriptionAccess, error) {
	var access subscriptionAccess
	var subscriptionID *string
	err := s.db.QueryRow(ctx, `select exists(
		select 1
		from user_subscriptions us
		join subscription_plans p on p.id=us.plan_id
		left join subscription_plan_model_quotas mq on mq.plan_id=p.id and mq.model=$2
		left join user_subscription_model_usage uq on uq.subscription_id=us.id and uq.model=$2
		where us.user_id::text=$1 and us.status='active' and (us.current_period_end is null or us.current_period_end > now())
		  and (coalesce(array_length(p.model_whitelist,1),0)=0 or $2 = any(p.model_whitelist))
		  and (coalesce(mq.max_requests_per_period,p.max_requests_per_period) is null
		    or coalesce(uq.remaining_requests,us.remaining_requests) > 0)
		  and (coalesce(mq.max_credit_per_period,p.max_credit_per_period) is null
		    or coalesce(uq.remaining_credit,us.remaining_credit) > 0)
	) as covered,
	(
		select us2.id
		from user_subscriptions us2
		join subscription_plans p2 on p2.id=us2.plan_id
		left join subscription_plan_model_quotas mq2 on mq2.plan_id=p2.id and mq2.model=$2
		left join user_subscription_model_usage uq2 on uq2.subscription_id=us2.id and uq2.model=$2
		where us2.user_id::text=$1 and us2.status='active' and (us2.current_period_end is null or us2.current_period_end > now())
		  and (coalesce(array_length(p2.model_whitelist,1),0)=0 or $2 = any(p2.model_whitelist))
		  and (coalesce(mq2.max_requests_per_period,p2.max_requests_per_period) is null
		    or coalesce(uq2.remaining_requests,us2.remaining_requests) > 0)
		  and (coalesce(mq2.max_credit_per_period,p2.max_credit_per_period) is null
		    or coalesce(uq2.remaining_credit,us2.remaining_credit) > 0)
		order by us2.created_at desc
		limit 1
	) as subscription_id,
	coalesce((
		select p3.overage_policy
		from user_subscriptions us3
		join subscription_plans p3 on p3.id=us3.plan_id
		where us3.user_id::text=$1 and us3.status='active' and (us3.current_period_end is null or us3.current_period_end > now())
		  and (coalesce(array_length(p3.model_whitelist,1),0)=0 or $2 = any(p3.model_whitelist))
		order by us3.created_at desc
		limit 1
	), 'allow_wallet') as overage_policy`, userID, model).Scan(&access.Covered, &subscriptionID, &access.OveragePolicy)
	if err != nil {
		log.Printf("subscriptionCoversModel: %v", err)
		return subscriptionAccess{Covered: false, OveragePolicy: "allow_wallet"}, err
	}
	if subscriptionID != nil {
		access.SubscriptionID = *subscriptionID
	}
	return access, nil
}
