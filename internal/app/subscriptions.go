package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A subscription is a product ("订阅") that owns tiers ("等级"); a tier carries
// the benefit identity (group, model whitelist, overage policy) plus default
// per-period amounts, and each tier owns one price per billing period. A price
// may override the amounts for its own period, which is how the same
// subscription sold monthly, quarterly and yearly stays one tier.
type subscriptionProduct struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	SortOrder   int                `json:"sort_order"`
	Enabled     bool               `json:"enabled"`
	Tiers       []subscriptionTier `json:"tiers"`
	CreatedAt   any                `json:"created_at"`
	UpdatedAt   any                `json:"updated_at"`
}

type subscriptionTier struct {
	ID                 string                       `json:"id"`
	SubscriptionID     string                       `json:"subscription_id"`
	Level              int                          `json:"level"`
	Name               string                       `json:"name"`
	Description        string                       `json:"description"`
	CreditAmount       string                       `json:"credit_amount"`
	GroupID            string                       `json:"group_id"`
	GroupName          string                       `json:"group_name"`
	ModelWhitelist     []string                     `json:"model_whitelist"`
	MaxRequestsPerRule *int64                       `json:"max_requests_per_period"`
	MaxCreditPerRule   *float64                     `json:"max_credit_per_period"`
	OveragePolicy      string                       `json:"overage_policy"`
	ModelQuotas        []subscriptionPlanModelQuota `json:"model_quotas"`
	Prices             []subscriptionTierPrice      `json:"prices"`
	SortOrder          int                          `json:"sort_order"`
	Enabled            bool                         `json:"enabled"`
	CreatedAt          any                          `json:"created_at"`
	UpdatedAt          any                          `json:"updated_at"`
}

// subscriptionTierPrice is one billing-period option of a tier. The three amount
// fields are per-period overrides; an empty value inherits the tier default.
type subscriptionTierPrice struct {
	ID                 string   `json:"id"`
	BillingPeriod      string   `json:"billing_period"`
	Price              string   `json:"price"`
	Currency           string   `json:"currency"`
	CreditAmount       string   `json:"credit_amount"`
	MaxRequestsPerRule *int64   `json:"max_requests_per_period"`
	MaxCreditPerRule   *float64 `json:"max_credit_per_period"`
	Enabled            bool     `json:"enabled"`
	SortOrder          int      `json:"sort_order"`
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
	ID                 string                       `json:"id"`
	UserID             string                       `json:"user_id"`
	SubscriptionID     string                       `json:"subscription_id"`
	ProductName        string                       `json:"product_name"`
	TierID             string                       `json:"tier_id"`
	TierName           string                       `json:"tier_name"`
	TierLevel          int                          `json:"tier_level"`
	DisplayName        string                       `json:"display_name"`
	Status             string                       `json:"status"`
	CurrentPeriodStart any                          `json:"current_period_start"`
	CurrentPeriodEnd   any                          `json:"current_period_end"`
	AutoRenew          bool                         `json:"auto_renew"`
	CancelledAt        any                          `json:"cancelled_at"`
	CreatedAt          any                          `json:"created_at"`
	UpdatedAt          any                          `json:"updated_at"`
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
	UsageRequests      int64                        `json:"usage_requests"`
	UsageCredit        float64                      `json:"usage_credit"`
	ModelUsage         []subscriptionModelUsage     `json:"model_usage"`
	ResetCardCount     int64                        `json:"reset_card_count"`
}

type subscriptionOrder struct {
	ID             string `json:"id"`
	OrderNo        string `json:"order_no"`
	SubscriptionID string `json:"subscription_id"`
	TierID         string `json:"tier_id"`
	PlanName       string `json:"plan_name"`
	Provider       string `json:"provider"`
	PaymentType    string `json:"payment_type"`
	Amount         string `json:"amount"`
	BillingPeriod  string `json:"billing_period"`
	Status         string `json:"status"`
	ProviderTrade  string `json:"provider_trade_no,omitempty"`
	PeriodKind     string `json:"period_kind"`
	PaidAt         any    `json:"paid_at"`
	CreatedAt      any    `json:"created_at"`
}

// subscriptionDisplayName renders the catalogue name of one tier: a product with
// a single tier shows the product name alone, a multi-level product prefixes it.
func subscriptionDisplayName(productName, tierName string) string {
	tierName = strings.TrimSpace(tierName)
	if tierName == "" {
		return productName
	}
	if strings.EqualFold(productName, tierName) {
		return productName
	}
	return productName + " · " + tierName
}

// ---- Catalogue loading ----

func (s *Service) loadSubscriptionCatalog(ctx context.Context, includeDisabled bool) ([]subscriptionProduct, error) {
	productRows, err := s.db.Query(ctx, `select id::text,name,description,sort_order,enabled,created_at,updated_at from subscriptions where ($1 or enabled) order by sort_order,name`, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer productRows.Close()
	products := []subscriptionProduct{}
	order := []string{}
	byID := map[string]*subscriptionProduct{}
	for productRows.Next() {
		var p subscriptionProduct
		if err = productRows.Scan(&p.ID, &p.Name, &p.Description, &p.SortOrder, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Tiers = []subscriptionTier{}
		products = append(products, p)
		order = append(order, p.ID)
	}
	if err = productRows.Err(); err != nil {
		return nil, err
	}
	for i := range products {
		byID[products[i].ID] = &products[i]
	}
	if len(order) == 0 {
		return products, nil
	}

	tierRows, err := s.db.Query(ctx, `select t.id::text,t.subscription_id::text,t.level,t.name,t.description,coalesce(t.credit_amount::text,''),coalesce(t.group_id::text,''),coalesce(coalesce(g.display_name,g.name),''),t.model_whitelist,t.max_requests_per_period,t.max_credit_per_period,t.overage_policy,t.sort_order,t.enabled,t.created_at,t.updated_at from subscription_tiers t left join groups g on g.id=t.group_id where t.subscription_id = any($1) and ($2 or t.enabled) order by t.subscription_id,t.level`, order, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	tierIndex := map[string]*subscriptionTier{}
	tierOrder := []string{}
	for tierRows.Next() {
		var tier subscriptionTier
		var models []string
		if err = tierRows.Scan(&tier.ID, &tier.SubscriptionID, &tier.Level, &tier.Name, &tier.Description, &tier.CreditAmount, &tier.GroupID, &tier.GroupName, &models, &tier.MaxRequestsPerRule, &tier.MaxCreditPerRule, &tier.OveragePolicy, &tier.SortOrder, &tier.Enabled, &tier.CreatedAt, &tier.UpdatedAt); err != nil {
			return nil, err
		}
		tier.ModelWhitelist = models
		if tier.ModelWhitelist == nil {
			tier.ModelWhitelist = []string{}
		}
		tier.ModelQuotas = []subscriptionPlanModelQuota{}
		tier.Prices = []subscriptionTierPrice{}
		product := byID[tier.SubscriptionID]
		if product == nil {
			continue
		}
		product.Tiers = append(product.Tiers, tier)
		tierOrder = append(tierOrder, tier.ID)
	}
	if err = tierRows.Err(); err != nil {
		return nil, err
	}
	if len(tierOrder) == 0 {
		if includeDisabled {
			return products, nil
		}
		visible := products[:0]
		for _, p := range products {
			if len(p.Tiers) > 0 {
				visible = append(visible, p)
			}
		}
		return visible, nil
	}
	for i := range products {
		for j := range products[i].Tiers {
			tierIndex[products[i].Tiers[j].ID] = &products[i].Tiers[j]
		}
	}

	priceRows, err := s.db.Query(ctx, `select id::text,tier_id::text,billing_period,price::text,currency,coalesce(credit_amount::text,''),max_requests_per_period,max_credit_per_period,enabled,sort_order from subscription_tier_prices where tier_id = any($1) and ($2 or enabled) order by tier_id,sort_order,billing_period`, tierOrder, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer priceRows.Close()
	for priceRows.Next() {
		var price subscriptionTierPrice
		var tierID string
		if err = priceRows.Scan(&price.ID, &tierID, &price.BillingPeriod, &price.Price, &price.Currency, &price.CreditAmount, &price.MaxRequestsPerRule, &price.MaxCreditPerRule, &price.Enabled, &price.SortOrder); err != nil {
			return nil, err
		}
		if tier := tierIndex[tierID]; tier != nil {
			tier.Prices = append(tier.Prices, price)
		}
	}
	if err = priceRows.Err(); err != nil {
		return nil, err
	}

	quotaRows, err := s.db.Query(ctx, `select tier_id::text,model,max_requests_per_period,max_credit_per_period from subscription_tier_model_quotas where tier_id = any($1) order by tier_id,model`, tierOrder)
	if err != nil {
		return nil, err
	}
	defer quotaRows.Close()
	for quotaRows.Next() {
		var tierID string
		var q subscriptionPlanModelQuota
		if err = quotaRows.Scan(&tierID, &q.Model, &q.MaxRequestsPerRule, &q.MaxCreditPerRule); err != nil {
			return nil, err
		}
		if tier := tierIndex[tierID]; tier != nil {
			tier.ModelQuotas = append(tier.ModelQuotas, q)
		}
	}
	if err = quotaRows.Err(); err != nil {
		return nil, err
	}
	if includeDisabled {
		return products, nil
	}
	visible := products[:0]
	for _, p := range products {
		if len(p.Tiers) > 0 {
			visible = append(visible, p)
		}
	}
	return visible, nil
}

// ---- Admin: catalogue management ----

func (s *Service) listSubscriptionProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.loadSubscriptionCatalog(r.Context(), true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription products")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": products})
}

func (s *Service) publicSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	products, err := s.loadSubscriptionCatalog(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription products")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": products})
}

func (s *Service) createSubscriptionProduct(w http.ResponseWriter, r *http.Request) {
	in, err := readSubscriptionProductInput(r, s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `insert into subscriptions(id,name,description,sort_order,enabled) values($1,$2,$3,$4,$5)`, id, in.Name, in.Description, in.SortOrder, in.Enabled); err != nil {
		writeError(w, http.StatusConflict, "conflict", "subscription name already exists")
		return
	}
	if err = s.syncSubscriptionTiersTx(r.Context(), tx, id, in.Tiers); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	s.audit(r, "subscription_product.created", "subscription", id, map[string]any{"name": in.Name, "tiers": len(in.Tiers)})
	s.subscriptionCache.clear()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) updateSubscriptionProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	in, err := readSubscriptionProductInput(r, s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `update subscriptions set name=$1,description=$2,sort_order=$3,enabled=$4,updated_at=now() where id=$5`, in.Name, in.Description, in.SortOrder, in.Enabled, id)
	if err != nil {
		writeError(w, http.StatusConflict, "conflict", "subscription name already exists")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	if err = s.syncSubscriptionTiersTx(r.Context(), tx, id, in.Tiers); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update subscription")
		return
	}
	s.audit(r, "subscription_product.updated", "subscription", id, map[string]any{"name": in.Name, "enabled": in.Enabled})
	s.subscriptionCache.clear()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) deleteSubscriptionProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true"
	var subscribers int64
	if err := s.db.QueryRow(r.Context(), `select count(*) from user_subscriptions us join subscription_tiers t on t.id=us.tier_id where t.subscription_id=$1`, id).Scan(&subscribers); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete subscription")
		return
	}
	if subscribers > 0 && !force {
		writeError(w, http.StatusConflict, "subscription_in_use", "subscription still has user subscriptions; disable it instead or pass force=1")
		return
	}
	result, err := s.db.Exec(r.Context(), `delete from subscriptions where id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete subscription")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	s.audit(r, "subscription_product.deleted", "subscription", id, map[string]any{"forced": force})
	s.subscriptionCache.clear()
	w.WriteHeader(http.StatusNoContent)
}

type subscriptionTierInput struct {
	ID             string
	Level          int
	Name           string
	Description    string
	CreditAmount   string
	GroupRef       string
	ModelWhitelist []string
	MaxRequests    *int64
	MaxCredit      *float64
	OveragePolicy  string
	ModelQuotas    []subscriptionPlanModelQuota
	SortOrder      int
	Enabled        bool
	Prices         []subscriptionTierPriceInput
}

type subscriptionTierPriceInput struct {
	ID            string
	BillingPeriod string
	Price         string
	Currency      string
	CreditAmount  string
	MaxRequests   *int64
	MaxCredit     *float64
	Enabled       bool
	SortOrder     int
}

type subscriptionProductInput struct {
	Name        string
	Description string
	SortOrder   int
	Enabled     bool
	Tiers       []subscriptionTierInput
}

func readSubscriptionProductInput(r *http.Request, s *Service) (subscriptionProductInput, error) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *bool  `json:"enabled"`
		Tiers       []struct {
			ID             string                       `json:"id"`
			Level          int                          `json:"level"`
			Name           string                       `json:"name"`
			Description    string                       `json:"description"`
			CreditAmount   string                       `json:"credit_amount"`
			GroupID        string                       `json:"group_id"`
			ModelWhitelist []string                     `json:"model_whitelist"`
			MaxRequests    *int64                       `json:"max_requests_per_period"`
			MaxCredit      *float64                     `json:"max_credit_per_period"`
			OveragePolicy  string                       `json:"overage_policy"`
			ModelQuotas    []subscriptionPlanModelQuota `json:"model_quotas"`
			SortOrder      int                          `json:"sort_order"`
			Enabled        *bool                        `json:"enabled"`
			Prices         []struct {
				ID            string   `json:"id"`
				BillingPeriod string   `json:"billing_period"`
				Price         string   `json:"price"`
				Currency      string   `json:"currency"`
				CreditAmount  string   `json:"credit_amount"`
				MaxRequests   *int64   `json:"max_requests_per_period"`
				MaxCredit     *float64 `json:"max_credit_per_period"`
				Enabled       *bool    `json:"enabled"`
				SortOrder     int      `json:"sort_order"`
			} `json:"prices"`
		} `json:"tiers"`
	}
	if decode(r, &in) != nil {
		return subscriptionProductInput{}, fmt.Errorf("invalid subscription payload")
	}
	out := subscriptionProductInput{SortOrder: in.SortOrder, Enabled: true}
	out.Name = strings.TrimSpace(in.Name)
	if out.Name == "" || len(out.Name) > 100 {
		return subscriptionProductInput{}, fmt.Errorf("name must be 1-100 characters")
	}
	out.Description = strings.TrimSpace(in.Description)
	if len(out.Description) > maxPlanDescriptionLength {
		return subscriptionProductInput{}, fmt.Errorf("description must be at most 2000 characters")
	}
	if in.SortOrder < minPlanSortOrder || in.SortOrder > maxPlanSortOrder {
		return subscriptionProductInput{}, fmt.Errorf("sort_order must be between -10000 and 10000")
	}
	if in.Enabled != nil {
		out.Enabled = *in.Enabled
	}
	if len(in.Tiers) > maxSubscriptionTiers {
		return subscriptionProductInput{}, fmt.Errorf("a subscription supports at most %d tiers", maxSubscriptionTiers)
	}
	seenLevels := map[int]bool{}
	seenNames := map[string]bool{}
	for _, t := range in.Tiers {
		tier, err := readSubscriptionTierInput(r, s, t.ID, t.Level, t.Name, t.Description, t.CreditAmount, t.GroupID, t.ModelWhitelist, t.MaxRequests, t.MaxCredit, t.OveragePolicy, t.ModelQuotas, t.SortOrder, t.Enabled)
		if err != nil {
			return subscriptionProductInput{}, err
		}
		if seenLevels[tier.Level] {
			return subscriptionProductInput{}, fmt.Errorf("duplicate tier level %d", tier.Level)
		}
		seenLevels[tier.Level] = true
		if tier.Name != "" {
			key := strings.ToLower(tier.Name)
			if seenNames[key] {
				return subscriptionProductInput{}, fmt.Errorf("duplicate tier name %s", tier.Name)
			}
			seenNames[key] = true
		}
		for _, p := range t.Prices {
			period := strings.ToLower(strings.TrimSpace(p.BillingPeriod))
			switch period {
			case "hour", "day", "week", "month", "year":
			default:
				return subscriptionProductInput{}, fmt.Errorf("billing_period must be one of hour, day, week, month, year")
			}
			priceCents, _, ok := parsePaymentAmount(p.Price)
			if !ok || priceCents < 0 || priceCents > maxPlanPriceCents {
				return subscriptionProductInput{}, fmt.Errorf("price must be a non-negative decimal up to 100000.00")
			}
			currency := strings.ToUpper(strings.TrimSpace(p.Currency))
			if currency == "" {
				currency = "CNY"
			}
			if len(currency) > 8 {
				return subscriptionProductInput{}, fmt.Errorf("currency code too long")
			}
			credit := ""
			if raw := strings.TrimSpace(p.CreditAmount); raw != "" {
				parsed, ok := parseCreditAmount(raw)
				if !ok || parsed < 0 || parsed > maxPlanCreditAmount {
					return subscriptionProductInput{}, fmt.Errorf("credit_amount must be a non-negative decimal up to 1000000")
				}
				credit = formatCredit(parsed)
			}
			if !validQuotaLimit(p.MaxRequests) || !validQuotaCost(p.MaxCredit) {
				return subscriptionProductInput{}, fmt.Errorf("period limits must be between 0 and 1e12")
			}
			enabled := true
			if p.Enabled != nil {
				enabled = *p.Enabled
			}
			if p.SortOrder < minPlanSortOrder || p.SortOrder > maxPlanSortOrder {
				return subscriptionProductInput{}, fmt.Errorf("sort_order must be between -10000 and 10000")
			}
			for _, existing := range tier.Prices {
				if existing.BillingPeriod == period {
					return subscriptionProductInput{}, fmt.Errorf("duplicate billing period %s", period)
				}
			}
			tier.Prices = append(tier.Prices, subscriptionTierPriceInput{
				ID:            strings.TrimSpace(p.ID),
				BillingPeriod: period,
				Price:         formatAmount(priceCents),
				Currency:      currency,
				CreditAmount:  credit,
				MaxRequests:   p.MaxRequests,
				MaxCredit:     p.MaxCredit,
				Enabled:       enabled,
				SortOrder:     p.SortOrder,
			})
		}
		out.Tiers = append(out.Tiers, tier)
	}
	if len(out.Tiers) > 1 {
		for _, tier := range out.Tiers {
			if tier.Name == "" {
				return subscriptionProductInput{}, fmt.Errorf("every tier needs a name when a subscription has more than one tier")
			}
		}
	}
	return out, nil
}

func readSubscriptionTierInput(r *http.Request, s *Service, id string, level int, name, description, creditAmount, groupRef string, models []string, maxRequests *int64, maxCredit *float64, overagePolicy string, quotas []subscriptionPlanModelQuota, sortOrder int, enabled *bool) (subscriptionTierInput, error) {
	tier := subscriptionTierInput{
		ID:          strings.TrimSpace(id),
		Level:       level,
		SortOrder:   sortOrder,
		Enabled:     true,
		MaxRequests: maxRequests,
		MaxCredit:   maxCredit,
	}
	tier.Name = strings.TrimSpace(name)
	if len(tier.Name) > 100 {
		return subscriptionTierInput{}, fmt.Errorf("tier name must be at most 100 characters")
	}
	if level < 1 || level > maxSubscriptionTierLevel {
		return subscriptionTierInput{}, fmt.Errorf("tier level must be between 1 and %d", maxSubscriptionTierLevel)
	}
	tier.Description = strings.TrimSpace(description)
	if len(tier.Description) > maxPlanDescriptionLength {
		return subscriptionTierInput{}, fmt.Errorf("description must be at most 2000 characters")
	}
	if raw := strings.TrimSpace(creditAmount); raw != "" {
		parsed, ok := parseCreditAmount(raw)
		if !ok || parsed < 0 || parsed > maxPlanCreditAmount {
			return subscriptionTierInput{}, fmt.Errorf("credit_amount must be a non-negative decimal up to 1000000")
		}
		tier.CreditAmount = formatCredit(parsed)
	}
	if !validQuotaLimit(maxRequests) || !validQuotaCost(maxCredit) {
		return subscriptionTierInput{}, fmt.Errorf("period limits must be between 0 and 1e12")
	}
	policy := strings.ToLower(strings.TrimSpace(overagePolicy))
	if policy == "" {
		policy = "allow_wallet"
	}
	if policy != "allow_wallet" && policy != "block" {
		return subscriptionTierInput{}, fmt.Errorf("overage_policy must be allow_wallet or block")
	}
	tier.OveragePolicy = policy
	if sortOrder < minPlanSortOrder || sortOrder > maxPlanSortOrder {
		return subscriptionTierInput{}, fmt.Errorf("sort_order must be between -10000 and 10000")
	}
	if enabled != nil {
		tier.Enabled = *enabled
	}
	ref := strings.TrimSpace(groupRef)
	if ref != "" {
		var groupID string
		if err := s.db.QueryRow(r.Context(), `select id::text from groups where id::text=$1 or name=$2 or display_name=$2 limit 1`, ref, ref).Scan(&groupID); err != nil {
			return subscriptionTierInput{}, fmt.Errorf("group does not exist")
		}
		ref = groupID
	}
	tier.GroupRef = ref
	tier.ModelWhitelist = models
	if tier.ModelWhitelist == nil {
		tier.ModelWhitelist = []string{}
	}
	for i, m := range tier.ModelWhitelist {
		tier.ModelWhitelist[i] = strings.TrimSpace(m)
		if tier.ModelWhitelist[i] == "" || len(tier.ModelWhitelist[i]) > 200 {
			return subscriptionTierInput{}, fmt.Errorf("invalid model whitelist entry")
		}
	}
	tier.ModelQuotas = []subscriptionPlanModelQuota{}
	seenModels := map[string]bool{}
	for _, q := range quotas {
		modelName := strings.TrimSpace(q.Model)
		if modelName == "" || len(modelName) > 200 {
			return subscriptionTierInput{}, fmt.Errorf("invalid model quota model entry")
		}
		if seenModels[modelName] {
			return subscriptionTierInput{}, fmt.Errorf("duplicate model quota entry for %s", modelName)
		}
		if !validQuotaLimit(q.MaxRequestsPerRule) || !validQuotaCost(q.MaxCreditPerRule) {
			return subscriptionTierInput{}, fmt.Errorf("model quota limits must be between 0 and 1e12")
		}
		if q.MaxRequestsPerRule == nil && q.MaxCreditPerRule == nil {
			return subscriptionTierInput{}, fmt.Errorf("model quota for %s must set at least one limit", modelName)
		}
		seenModels[modelName] = true
		tier.ModelQuotas = append(tier.ModelQuotas, subscriptionPlanModelQuota{Model: modelName, MaxRequestsPerRule: q.MaxRequestsPerRule, MaxCreditPerRule: q.MaxCreditPerRule})
	}
	return tier, nil
}

// syncSubscriptionTiersTx writes the tiers of one product and their prices and
// model quotas, and reconciles tiers that the payload dropped: a tier that still
// has subscribers is disabled (its user_subscriptions must keep working), one
// without subscribers is deleted. A payload tier that currently belongs to
// another product is moved here, which is how two products that describe the
// same subscription are consolidated.
func (s *Service) syncSubscriptionTiersTx(ctx context.Context, tx pgx.Tx, productID string, tiers []subscriptionTierInput) error {
	existingRows, err := tx.Query(ctx, `select id::text from subscription_tiers where subscription_id=$1`, productID)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for existingRows.Next() {
		var id string
		if err = existingRows.Scan(&id); err != nil {
			existingRows.Close()
			return err
		}
		existing[id] = true
	}
	existingRows.Close()
	if err = existingRows.Err(); err != nil {
		return err
	}

	// Tiers are first parked under temporary levels and names so that moving a
	// tier between products, or renaming two tiers at once, cannot trip the
	// unique (subscription_id, level) / (subscription_id, name) indexes. The
	// parking band starts above every level already in use so a tier retired by an
	// earlier edit cannot collide with it.
	var parkBase int
	if err = tx.QueryRow(ctx, `select coalesce(max(level),0) from subscription_tiers where subscription_id=$1`, productID).Scan(&parkBase); err != nil {
		return err
	}
	parkBase++
	kept := map[string]bool{}
	for i := range tiers {
		if tiers[i].ID == "" {
			id, err := randomID()
			if err != nil {
				return err
			}
			tiers[i].ID = id
			if _, err = tx.Exec(ctx, `insert into subscription_tiers(id,subscription_id,level,name) values($1,$2,$3,$4)`,
				id, productID, parkBase+i, tempTierName(i)); err != nil {
				return err
			}
		} else {
			if _, err = tx.Exec(ctx, `update subscription_tiers set subscription_id=$1,level=$2,name=$3 where id=$4`, productID, parkBase+i, tempTierName(i), tiers[i].ID); err != nil {
				return err
			}
		}
		kept[tiers[i].ID] = true
	}

	// Dropped tiers are reconciled before the kept tiers take their real levels:
	// a dropped tier must not sit on a level or name the payload wants. One that
	// still has subscribers is disabled (its subscriptions have to keep working)
	// and parked above the payload range; one without subscribers is deleted
	// outright.
	removed := make([]string, 0, len(existing))
	for id := range existing {
		if !kept[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(removed)
	wanted := make([]string, 0, len(tiers))
	for i := range tiers {
		wanted = append(wanted, tiers[i].Name)
	}
	retired := 0
	for _, id := range removed {
		var subscribers int64
		if err = tx.QueryRow(ctx, `select count(*) from user_subscriptions where tier_id=$1`, id).Scan(&subscribers); err != nil {
			return err
		}
		if subscribers > 0 {
			// The retired tier keeps its name so support can still find it, unless
			// the payload gives that name to a live tier, in which case the retired
			// one yields it.
			if _, err = tx.Exec(ctx, `update subscription_tiers set enabled=false,level=$1,name=case when name = any($2::text[]) then name || ' (retired)' else name end,updated_at=now() where id=$3`, parkBase+len(tiers)+retired, wanted, id); err != nil {
				return err
			}
			retired++
			continue
		}
		if _, err = tx.Exec(ctx, `delete from subscription_tiers where id=$1`, id); err != nil {
			return err
		}
	}

	for i := range tiers {
		tier := tiers[i]
		if _, err = tx.Exec(ctx, `update subscription_tiers set level=$1,name=$2,description=$3,credit_amount=$4,group_id=$5,model_whitelist=$6,max_requests_per_period=$7,max_credit_per_period=$8,overage_policy=$9,sort_order=$10,enabled=$11,updated_at=now() where id=$12`,
			tier.Level, tier.Name, tier.Description, nullableCredit(tier.CreditAmount), nullableGroupRef(tier.GroupRef), tier.ModelWhitelist, tier.MaxRequests, tier.MaxCredit, tier.OveragePolicy, tier.SortOrder, tier.Enabled, tier.ID); err != nil {
			return err
		}
		if err = s.syncTierPricesTx(ctx, tx, tier.ID, tier.Prices); err != nil {
			return err
		}
		if err = s.syncTierModelQuotasTx(ctx, tx, tier.ID, tier.ModelQuotas); err != nil {
			return err
		}
	}
	return nil
}

func tempTierName(index int) string {
	return fmt.Sprintf("__pending_tier_%d", index)
}

func (s *Service) syncTierPricesTx(ctx context.Context, tx pgx.Tx, tierID string, prices []subscriptionTierPriceInput) error {
	keep := []string{}
	for _, price := range prices {
		if price.ID == "" {
			id, err := randomID()
			if err != nil {
				return err
			}
			price.ID = id
			if _, err = tx.Exec(ctx, `insert into subscription_tier_prices(id,tier_id,billing_period,price,currency,credit_amount,max_requests_per_period,max_credit_per_period,enabled,sort_order) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				id, tierID, price.BillingPeriod, price.Price, price.Currency, nullableCredit(price.CreditAmount), price.MaxRequests, price.MaxCredit, price.Enabled, price.SortOrder); err != nil {
				return err
			}
		} else {
			result, err := tx.Exec(ctx, `update subscription_tier_prices set billing_period=$1,price=$2,currency=$3,credit_amount=$4,max_requests_per_period=$5,max_credit_per_period=$6,enabled=$7,sort_order=$8,updated_at=now() where id=$9 and tier_id=$10`,
				price.BillingPeriod, price.Price, price.Currency, nullableCredit(price.CreditAmount), price.MaxRequests, price.MaxCredit, price.Enabled, price.SortOrder, price.ID, tierID)
			if err != nil {
				return err
			}
			if result.RowsAffected() == 0 {
				if _, err = tx.Exec(ctx, `insert into subscription_tier_prices(id,tier_id,billing_period,price,currency,credit_amount,max_requests_per_period,max_credit_per_period,enabled,sort_order) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
					price.ID, tierID, price.BillingPeriod, price.Price, price.Currency, nullableCredit(price.CreditAmount), price.MaxRequests, price.MaxCredit, price.Enabled, price.SortOrder); err != nil {
					return err
				}
			}
		}
		keep = append(keep, price.ID)
	}
	if _, err := tx.Exec(ctx, `delete from subscription_tier_prices where tier_id=$1 and not (id = any($2))`, tierID, keep); err != nil {
		return err
	}
	return nil
}

func (s *Service) syncTierModelQuotasTx(ctx context.Context, tx pgx.Tx, tierID string, quotas []subscriptionPlanModelQuota) error {
	if _, err := tx.Exec(ctx, `delete from subscription_tier_model_quotas where tier_id=$1`, tierID); err != nil {
		return err
	}
	for _, q := range quotas {
		if _, err := tx.Exec(ctx, `insert into subscription_tier_model_quotas(tier_id,model,max_requests_per_period,max_credit_per_period) values($1,$2,$3,$4) on conflict (tier_id,model) do update set max_requests_per_period=excluded.max_requests_per_period, max_credit_per_period=excluded.max_credit_per_period`, tierID, q.Model, q.MaxRequestsPerRule, q.MaxCreditPerRule); err != nil {
			return err
		}
	}
	return nil
}

const (
	maxPlanDescriptionLength = 2000
	maxPlanPriceCents        = maxPaymentCents
	maxPlanCreditAmount      = 1_000_000.0
	minPlanSortOrder         = -10000
	maxPlanSortOrder         = 10000
	maxSubscriptionTiers     = 50
	maxSubscriptionTierLevel = 1000
	// maxSubscriptionPeriodDays bounds an explicit period override on an admin
	// grant or a redemption code.
	maxSubscriptionPeriodDays = 3650
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

// ---- Account: browse own subscriptions ----

func (s *Service) accountSubscriptions(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select us.id::text,us.user_id::text,t.subscription_id::text,s.name,t.id::text,t.level,t.name,us.status,us.current_period_start,us.current_period_end,us.auto_renew,us.cancelled_at,us.created_at,us.updated_at,coalesce(us.price::text,''),us.currency,coalesce(us.billing_period,'month'),coalesce(coalesce(pr.credit_amount,t.credit_amount)::text,''),coalesce(t.group_id::text,''),coalesce(coalesce(g.display_name,g.name),''),t.model_whitelist,coalesce(pr.max_requests_per_period,t.max_requests_per_period),coalesce(pr.max_credit_per_period,t.max_credit_per_period),t.overage_policy from user_subscriptions us join subscription_tiers t on t.id=us.tier_id join subscriptions s on s.id=t.subscription_id left join groups g on g.id=t.group_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period where us.user_id=$1 order by s.sort_order,s.name,t.level,us.created_at desc`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	defer rows.Close()
	subs := []userSubscription{}
	tierIDs := []string{}
	subIDs := []string{}
	for rows.Next() {
		var sub userSubscription
		var productName, tierName string
		var models []string
		var periodStart *time.Time
		if err = rows.Scan(&sub.ID, &sub.UserID, &sub.SubscriptionID, &productName, &sub.TierID, &sub.TierLevel, &tierName, &sub.Status, &periodStart, &sub.CurrentPeriodEnd, &sub.AutoRenew, &sub.CancelledAt, &sub.CreatedAt, &sub.UpdatedAt, &sub.Price, &sub.Currency, &sub.BillingPeriod, &sub.CreditAmount, &sub.GroupID, &sub.GroupName, &models, &sub.MaxRequestsPerRule, &sub.MaxCreditPerRule, &sub.OveragePolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		sub.CurrentPeriodStart = periodStart
		sub.ProductName = productName
		sub.TierName = tierName
		sub.DisplayName = subscriptionDisplayName(productName, tierName)
		sub.ModelWhitelist = models
		if sub.ModelWhitelist == nil {
			sub.ModelWhitelist = []string{}
		}
		sub.ModelQuotas = []subscriptionPlanModelQuota{}
		subs = append(subs, sub)
		tierIDs = append(tierIDs, sub.TierID)
		subIDs = append(subIDs, sub.ID)
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	if len(tierIDs) > 0 {
		quotaRows, err := s.db.Query(r.Context(), `select tier_id::text,model,max_requests_per_period,max_credit_per_period from subscription_tier_model_quotas where tier_id = any($1) order by tier_id,model`, tierIDs)
		if err == nil {
			quotaMap := map[string][]subscriptionPlanModelQuota{}
			for quotaRows.Next() {
				var q subscriptionPlanModelQuota
				var tierID string
				if err = quotaRows.Scan(&tierID, &q.Model, &q.MaxRequestsPerRule, &q.MaxCreditPerRule); err == nil {
					quotaMap[tierID] = append(quotaMap[tierID], q)
				}
			}
			quotaRows.Close()
			for i := range subs {
				if qs, ok := quotaMap[subs[i].TierID]; ok {
					subs[i].ModelQuotas = qs
				}
			}
		}
	}
	for i := range subs {
		sub := &subs[i]
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
		var resetCardCount int64
		if err := s.db.QueryRow(r.Context(), `select count(*) from reset_cards where subscription_id=$1 and enabled and used_at is null and (expires_at is null or expires_at>now())`, sub.ID).Scan(&resetCardCount); err == nil {
			sub.ResetCardCount = resetCardCount
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": subs})
}

func (s *Service) accountSubscriptionOrders(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select id::text,order_no,subscription_id::text,coalesce(tier_id::text,''),plan_name,provider,payment_type,amount::text,coalesce(billing_period,'month'),status,coalesce(provider_trade_no,''),period_kind,paid_at,created_at from subscription_orders where user_id=$1 order by created_at desc limit 50`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	defer rows.Close()
	orders := []subscriptionOrder{}
	for rows.Next() {
		var order subscriptionOrder
		if err = rows.Scan(&order.ID, &order.OrderNo, &order.SubscriptionID, &order.TierID, &order.PlanName, &order.Provider, &order.PaymentType, &order.Amount, &order.BillingPeriod, &order.Status, &order.ProviderTrade, &order.PeriodKind, &order.PaidAt, &order.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
			return
		}
		orders = append(orders, order)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": orders})
}

// tierPurchase describes one purchasable (tier, billing period) pair.
type tierPurchase struct {
	TierID        string
	TierLevel     int
	TierName      string
	ProductID     string
	ProductName   string
	DisplayName   string
	BillingPeriod string
	PriceID       string
	Price         string
	Currency      string
	GroupID       string
	OveragePolicy string
}

func (s *Service) loadTierPurchase(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tierID, billingPeriod string, enabledOnly bool) (tierPurchase, error) {
	var p tierPurchase
	err := q.QueryRow(ctx, `select t.id::text,t.level,t.name,s.id::text,s.name,coalesce(t.group_id::text,''),t.overage_policy,coalesce(pr.id::text,''),coalesce(pr.billing_period,$2),coalesce(pr.price::text,'0.00'),coalesce(pr.currency,'CNY') from subscription_tiers t join subscriptions s on s.id=t.subscription_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=$2 where t.id=$1 and (not $3 or (t.enabled and s.enabled and pr.enabled))`, tierID, billingPeriod, enabledOnly).
		Scan(&p.TierID, &p.TierLevel, &p.TierName, &p.ProductID, &p.ProductName, &p.GroupID, &p.OveragePolicy, &p.PriceID, &p.BillingPeriod, &p.Price, &p.Currency)
	if err != nil {
		return tierPurchase{}, err
	}
	p.DisplayName = subscriptionDisplayName(p.ProductName, p.TierName)
	return p, nil
}

func (s *Service) createSubscription(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	settings, err := s.loadEpaySettings(r)
	if err != nil || !settings.ready() {
		writeError(w, http.StatusServiceUnavailable, "payment_unavailable", "online payment is not configured")
		return
	}
	var in struct {
		TierID        string `json:"tier_id"`
		BillingPeriod string `json:"billing_period"`
		PaymentType   string `json:"payment_type"`
		AutoRenew     bool   `json:"auto_renew"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.TierID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "tier_id is required")
		return
	}
	paymentType := strings.ToLower(strings.TrimSpace(in.PaymentType))
	if !methodEnabled(settings.Methods, paymentType) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payment type")
		return
	}
	billingPeriod := strings.ToLower(strings.TrimSpace(in.BillingPeriod))
	if billingPeriod == "" {
		billingPeriod = "month"
	}
	purchase, err := s.loadTierPurchase(r.Context(), s.db, strings.TrimSpace(in.TierID), billingPeriod, true)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "subscription tier or billing period not found")
		return
	}
	if priceCents, _, ok := parsePaymentAmount(purchase.Price); !ok || priceCents <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "this subscription period requires a positive price")
		return
	}
	orderID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	orderNo := fmt.Sprintf("xhsub%d%s", time.Now().UnixMilli(), strings.ReplaceAll(orderID, "-", "")[:12])
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	defer tx.Rollback(r.Context())
	subID, renewal, err := s.ensureSubscriptionRowTx(r.Context(), tx, account.userID, purchase, in.AutoRenew)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	periodKind := "new"
	if renewal {
		periodKind = "renewal"
	}
	if _, err = tx.Exec(r.Context(), `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period,period_kind,auto_renew) values($1,$2,$3,$4,$5,$6,'epay',$7,$8,$9,$10,$11)`,
		orderID, orderNo, subID, account.userID, purchase.TierID, purchase.DisplayName, paymentType, purchase.Price, purchase.BillingPeriod, periodKind, in.AutoRenew); err != nil {
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
		"name":         {"Xinghai subscription: " + purchase.DisplayName},
		"money":        {purchase.Price},
	}
	params.Set("sign", epaySign(params, settings.MerchantKey))
	params.Set("sign_type", "MD5")
	writeJSON(w, http.StatusCreated, map[string]any{"order_no": orderNo, "amount": purchase.Price, "status": "pending", "pay_url": settings.BaseURL + "/submit.php?" + params.Encode()})
}

func (s *Service) createBalanceSubscription(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		TierID        string `json:"tier_id"`
		BillingPeriod string `json:"billing_period"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.TierID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "tier_id is required")
		return
	}
	billingPeriod := strings.ToLower(strings.TrimSpace(in.BillingPeriod))
	if billingPeriod == "" {
		billingPeriod = "month"
	}
	purchase, err := s.loadTierPurchase(r.Context(), s.db, strings.TrimSpace(in.TierID), billingPeriod, true)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "subscription tier or billing period not found")
		return
	}
	if priceCents, _, ok := parsePaymentAmount(purchase.Price); !ok || priceCents <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "this subscription period requires a positive price")
		return
	}
	orderID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	ledgerID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	orderNo := fmt.Sprintf("xhsub%d%s", time.Now().UnixMilli(), strings.ReplaceAll(orderID, "-", "")[:12])
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict do nothing`, account.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	var balanceAfter string
	err = tx.QueryRow(r.Context(), `update user_wallets set balance=balance-$1::numeric,updated_at=now() where user_id=$2 and balance-reserved >= $1::numeric returning balance::text`, purchase.Price, account.userID).Scan(&balanceAfter)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusConflict, "insufficient_balance", "insufficient available balance")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	subID, renewal, err := s.activateSubscriptionTx(r.Context(), tx, account.userID, purchase, subscriptionActivation{RenewAutoRenew: boolPtr(false)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	periodKind := "new"
	if renewal {
		periodKind = "renewal"
	}
	if _, err = tx.Exec(r.Context(), `insert into subscription_orders(id,order_no,subscription_id,user_id,tier_id,plan_name,provider,payment_type,amount,billing_period,status,period_kind,paid_at) values($1,$2,$3,$4,$5,$6,'wallet','balance',$7,$8,'paid',$9,now())`,
		orderID, orderNo, subID, account.userID, purchase.TierID, purchase.DisplayName, purchase.Price, purchase.BillingPeriod, periodKind); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,-$3::numeric,$4,'subscription_purchase',$5,$6)`, ledgerID, account.userID, purchase.Price, balanceAfter, orderNo, "Subscription purchase"); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create subscription")
		return
	}
	s.audit(r, "subscription.balance_purchased", "user_subscription", subID, map[string]any{"order_no": orderNo, "tier_id": purchase.TierID, "billing_period": purchase.BillingPeriod, "amount": purchase.Price})
	s.notifyAdminsOfPurchase(r.Context(), "subscription", orderNo, account.userID, purchase.Price, purchase.DisplayName)
	s.subscriptionCache.clear()
	writeJSON(w, http.StatusCreated, map[string]any{"order_no": orderNo, "subscription_id": subID, "amount": purchase.Price, "status": "paid", "balance": balanceAfter})
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

func (s *Service) accountSubscriptionOrder(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var order subscriptionOrder
	err := s.db.QueryRow(r.Context(), `select id::text,order_no,subscription_id::text,coalesce(tier_id::text,''),plan_name,provider,payment_type,amount::text,coalesce(billing_period,'month'),status,coalesce(provider_trade_no,''),period_kind,paid_at,created_at from subscription_orders where order_no=$1 and user_id=$2`, r.PathValue("order_no"), account.userID).Scan(&order.ID, &order.OrderNo, &order.SubscriptionID, &order.TierID, &order.PlanName, &order.Provider, &order.PaymentType, &order.Amount, &order.BillingPeriod, &order.Status, &order.ProviderTrade, &order.PeriodKind, &order.PaidAt, &order.CreatedAt)
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

// ---- Subscription instance activation ----

type subscriptionActivation struct {
	PeriodDays     *int
	StartAt        *time.Time
	EndAt          *time.Time
	RenewAutoRenew *bool
	InstanceID     string
	ForceNew       bool
}

func boolPtr(v bool) *bool { return &v }

// ensureSubscriptionRowTx returns the instance selected for a normal purchase
// (active first, then furthest expiry), creating a pending instance when none
// exists. Admin grants may explicitly create additional instances. The billing
// period snapshot is only written for a new instance; an active instance keeps
// the period it is currently serving until payment activates the new one.
func (s *Service) ensureSubscriptionRowTx(ctx context.Context, tx pgx.Tx, userID string, purchase tierPurchase, autoRenew bool) (string, bool, error) {
	var id string
	err := tx.QueryRow(ctx, `select id::text from user_subscriptions where user_id=$1 and tier_id=$2 order by (status='active' and (current_period_end is null or current_period_end>now())) desc,current_period_end desc nulls last,created_at desc limit 1 for update`, userID, purchase.TierID).Scan(&id)
	if err == nil {
		// The instance may already be active; the auto-renew choice of this
		// checkout lives on the order until it is paid.
		return id, true, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, err
	}
	newID, err := randomID()
	if err != nil {
		return "", false, err
	}
	if _, err = tx.Exec(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,billing_period,price,currency,auto_renew) values($1,$2,$3,'pending',$4,$5,$6,$7)`,
		newID, userID, purchase.TierID, purchase.BillingPeriod, purchase.Price, purchase.Currency, autoRenew); err != nil {
		return "", false, err
	}
	return newID, false, nil
}

// activateSubscriptionTx credits a purchase or grant to the selected instance.
// A normal renewal extends the chosen instance from its current end and resets
// its quota; ForceNew admin grants can create a sibling instance for the same tier.
func (s *Service) activateSubscriptionTx(ctx context.Context, tx pgx.Tx, userID string, purchase tierPurchase, opts subscriptionActivation) (string, bool, error) {
	var tierProductID, tierName, tierCredit, groupID string
	var tierLevel int
	if err := tx.QueryRow(ctx, `select subscription_id::text,level,name,coalesce(credit_amount::text,''),coalesce(group_id::text,'') from subscription_tiers where id=$1`, purchase.TierID).Scan(&tierProductID, &tierLevel, &tierName, &tierCredit, &groupID); err != nil {
		return "", false, err
	}
	if purchase.ProductName == "" {
		_ = tx.QueryRow(ctx, `select name from subscriptions where id=$1`, tierProductID).Scan(&purchase.ProductName)
		purchase.TierName = tierName
		purchase.TierLevel = tierLevel
		purchase.DisplayName = subscriptionDisplayName(purchase.ProductName, tierName)
	}
	if purchase.GroupID == "" {
		purchase.GroupID = groupID
	}
	var priceCredit string
	var override string
	if err := tx.QueryRow(ctx, `select coalesce(credit_amount::text,'') from subscription_tier_prices where tier_id=$1 and billing_period=$2`, purchase.TierID, purchase.BillingPeriod).Scan(&override); err == nil && override != "" {
		priceCredit = override
	}
	billingPeriod := purchase.BillingPeriod
	if billingPeriod == "" {
		billingPeriod = "month"
	}

	var existingID string
	var existingStart, existingEnd *time.Time
	var existingStatus string
	var err error
	renewal := false
	if opts.InstanceID != "" {
		err = tx.QueryRow(ctx, `select id::text,current_period_start,current_period_end,status from user_subscriptions where id=$1 and user_id=$2 and tier_id=$3 for update`, opts.InstanceID, userID, purchase.TierID).Scan(&existingID, &existingStart, &existingEnd, &existingStatus)
		renewal = err == nil
	} else if !opts.ForceNew {
		err = tx.QueryRow(ctx, `select id::text,current_period_start,current_period_end,status from user_subscriptions where user_id=$1 and tier_id=$2 order by (status='active' and (current_period_end is null or current_period_end>now())) desc,current_period_end desc nulls last,created_at desc limit 1 for update`, userID, purchase.TierID).Scan(&existingID, &existingStart, &existingEnd, &existingStatus)
		renewal = err == nil
	} else {
		err = pgx.ErrNoRows
	}
	if err != nil && err != pgx.ErrNoRows {
		return "", false, err
	}
	if opts.InstanceID != "" && err == pgx.ErrNoRows {
		return "", false, pgx.ErrNoRows
	}

	now := time.Now()
	var start, end time.Time
	switch {
	case opts.StartAt != nil:
		start = *opts.StartAt
		if opts.EndAt != nil {
			end = *opts.EndAt
		} else {
			end = subscriptionPeriodEnd(start, billingPeriod)
		}
	case opts.PeriodDays != nil:
		end = now.AddDate(0, 0, *opts.PeriodDays)
		start = now
	case renewal && existingStatus == "active" && existingEnd != nil && existingEnd.After(now):
		start = now
		if existingStart != nil {
			start = *existingStart
		}
		end = subscriptionPeriodEnd(*existingEnd, billingPeriod)
	default:
		start = now
		end = subscriptionPeriodEnd(now, billingPeriod)
	}
	if opts.EndAt != nil && opts.PeriodDays != nil {
		end = *opts.EndAt
	}

	if renewal {
		if _, err = tx.Exec(ctx, `update user_subscriptions set status='active',current_period_start=$1,current_period_end=$2,billing_period=$3,price=$4,currency=$5,auto_renew=coalesce($6::boolean,auto_renew),updated_at=now() where id=$7`,
			start, end, billingPeriod, purchase.Price, purchase.Currency, opts.RenewAutoRenew, existingID); err != nil {
			return "", false, err
		}
	} else {
		newID, err := randomID()
		if err != nil {
			return "", false, err
		}
		autoRenew := opts.RenewAutoRenew != nil && *opts.RenewAutoRenew
		if _, err = tx.Exec(ctx, `insert into user_subscriptions(id,user_id,tier_id,status,current_period_start,current_period_end,billing_period,price,currency,auto_renew) values($1,$2,$3,'active',$4,$5,$6,$7,$8,$9)`,
			newID, userID, purchase.TierID, start, end, billingPeriod, purchase.Price, purchase.Currency, autoRenew); err != nil {
			return "", false, err
		}
		existingID = newID
	}
	if err = s.initSubscriptionCountersTx(ctx, tx, existingID); err != nil {
		return "", false, err
	}
	credit := ""
	if priceCredit != "" {
		credit = priceCredit
	} else {
		credit = tierCredit
	}
	if amount, ok := parseCreditAmount(credit); ok && amount > 0 {
		if err = s.creditWalletTx(ctx, tx, userID, amount, "subscription_topup", existingID, "Subscription credit"); err != nil {
			return "", false, err
		}
	}
	if purchase.GroupID != "" {
		if _, err = tx.Exec(ctx, `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`, userID, purchase.GroupID); err != nil {
			return "", false, err
		}
	}
	return existingID, renewal, nil
}

// activateSubscriptionOrderTx activates the subscription order identified by
// orderNo within the supplied transaction. It is a no-op if the order does not
// exist, belongs to a different provider, or is already paid. The tradeNo is
// recorded for reconciliation. Returns true when the order was activated by this
// call.
func (s *Service) activateSubscriptionOrderTx(ctx context.Context, tx pgx.Tx, orderNo, tradeNo, notifiedAmount string) (bool, error) {
	var id, subID, userID, planName, status, amountStr string
	var tierID *string
	var billingPeriod *string
	var autoRenew bool
	err := tx.QueryRow(ctx, `select id::text,subscription_id::text,user_id::text,coalesce(tier_id::text,''),coalesce(plan_name,''),status,amount::text,coalesce(billing_period,'month'),auto_renew from subscription_orders where order_no=$1 and provider='epay' for update`, orderNo).Scan(&id, &subID, &userID, &tierID, &planName, &status, &amountStr, &billingPeriod, &autoRenew)
	if err != nil {
		return false, nil
	}
	if amountStr != notifiedAmount {
		return false, fmt.Errorf("subscription order amount mismatch")
	}
	if status == "paid" {
		return true, nil
	}
	if status != "pending" {
		return false, fmt.Errorf("subscription order not pending: %s", status)
	}
	if tierID == nil || *tierID == "" {
		return false, fmt.Errorf("subscription order has no tier")
	}
	purchase, err := s.loadTierPurchase(ctx, tx, *tierID, *billingPeriod, false)
	if err != nil {
		// The tier or its price row may have been removed after checkout; fall
		// back to the order snapshot so a paid order is never lost.
		purchase = tierPurchase{TierID: *tierID, BillingPeriod: *billingPeriod, DisplayName: planName}
	}
	if _, err = tx.Exec(ctx, `update subscription_orders set status='paid',provider_trade_no=$1,paid_at=now(),updated_at=now() where id=$2`, tradeNo, id); err != nil {
		return false, err
	}
	if _, _, err = s.activateSubscriptionTx(ctx, tx, userID, purchase, subscriptionActivation{RenewAutoRenew: &autoRenew, InstanceID: subID}); err != nil {
		return false, err
	}
	return true, nil
}

// initSubscriptionCountersTx resets a subscription's remaining request/credit
// counters to the quota of the billing period it is currently serving (the price
// override when the period defines one, else the tier default) and mirrors each
// per-model quota into user_subscription_model_usage. Called on activation and
// renewal so every period starts with a full quota. Null counters mean the
// dimension is uncapped.
func (s *Service) initSubscriptionCountersTx(ctx context.Context, tx pgx.Tx, subID string) error {
	var maxReq *int64
	var maxCredit *float64
	if err := tx.QueryRow(ctx, `select coalesce(pr.max_requests_per_period,t.max_requests_per_period),coalesce(pr.max_credit_per_period,t.max_credit_per_period) from user_subscriptions us join subscription_tiers t on t.id=us.tier_id left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period where us.id=$1`, subID).Scan(&maxReq, &maxCredit); err != nil {
		return err
	}
	var tierID string
	if err := tx.QueryRow(ctx, `select tier_id::text from user_subscriptions where id=$1`, subID).Scan(&tierID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update user_subscriptions set remaining_requests=$1,remaining_credit=$2,updated_at=now() where id=$3`, maxReq, maxCredit, subID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from user_subscription_model_usage where subscription_id=$1`, subID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `select model,max_requests_per_period,max_credit_per_period from subscription_tier_model_quotas where tier_id=$1`, tierID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type modelQuotaRow struct {
		model   string
		mReq    *int64
		mCredit *float64
	}
	var modelQuotas []modelQuotaRow
	for rows.Next() {
		var q modelQuotaRow
		if err = rows.Scan(&q.model, &q.mReq, &q.mCredit); err != nil {
			return err
		}
		modelQuotas = append(modelQuotas, q)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, q := range modelQuotas {
		if _, err = tx.Exec(ctx, `insert into user_subscription_model_usage(subscription_id,model,remaining_requests,remaining_credit) values($1,$2,$3,$4)`, subID, q.model, q.mReq, q.mCredit); err != nil {
			return err
		}
	}
	return nil
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

// subscriptionCoversModel reports whether the user's request is covered by an
// active subscription instance whose tier whitelists the requested model (empty
// whitelist = all models) and whose per-period request/credit counters have not
// run out. Quota is tracked as remaining counters on user_subscriptions /
// user_subscription_model_usage that are reset to the effective period quota on
// every activation and decremented once per covered request at settlement time,
// so this check is a plain counter read instead of an on-the-fly aggregate over
// request_logs. Coverage is scoped by the key's group: a tier bound to a group is
// usable only by keys in that group (an ungrouped key may additionally use tiers
// bound to any of the user's groups via user_groups); a tier with no group is
// usable by ungrouped keys and by keys whose group is among the user's groups.
// Keys in different groups therefore never draw on each other's bound-tier
// quota. When a per-model quota override exists for the requested model, its
// counters take precedence over the tier/period counters: a null override
// dimension inherits the period-level counter for that model's requests. A null
// current_period_end means the subscription has no expiry. Among several covering
// subscriptions the highest tier level wins, then the earliest product. The
// returned SubscriptionID is the subscription whose quota this request consumes,
// so the settle path can decrement exactly that counter. The returned
// OveragePolicy tells the gateway whether to fall through to wallet billing
// ('allow_wallet') or reject ('block') once the quota is used up.
func (s *Service) subscriptionCoversModel(ctx context.Context, key keyContext, model string) subscriptionAccess {
	access, err := s.loadSubscriptionCoversModel(ctx, key, model)
	if err != nil {
		log.Printf("subscriptionCoversModel: %v", err)
		return subscriptionAccess{Covered: false, OveragePolicy: "allow_wallet"}
	}
	return access
}

func (s *Service) loadSubscriptionCoversModel(ctx context.Context, key keyContext, model string) (subscriptionAccess, error) {
	var access subscriptionAccess
	var subscriptionID *string
	err := s.db.QueryRow(ctx, `with candidates as (
		select us.id,
		       t.overage_policy,
		       t.level,
		       s.sort_order as product_sort,
		       us.created_at,
		       (coalesce(mq.max_requests_per_period,pr.max_requests_per_period,t.max_requests_per_period) is null
		         or coalesce(uq.remaining_requests,us.remaining_requests) > 0) as req_ok,
		       (coalesce(mq.max_credit_per_period,pr.max_credit_per_period,t.max_credit_per_period) is null
		         or coalesce(uq.remaining_credit,us.remaining_credit) > 0) as credit_ok
		from user_subscriptions us
		join subscription_tiers t on t.id=us.tier_id
		join subscriptions s on s.id=t.subscription_id
		left join subscription_tier_prices pr on pr.tier_id=t.id and pr.billing_period=us.billing_period
		left join subscription_tier_model_quotas mq on mq.tier_id=t.id and mq.model=$2
		left join user_subscription_model_usage uq on uq.subscription_id=us.id and uq.model=$2
		where us.user_id::text=$1 and us.status='active' and (us.current_period_end is null or us.current_period_end > now())
		  and ($4='' or us.id::text=$4)
		  and (t.group_id::text=$3
		    or (t.group_id is null and ($3='' or exists(select 1 from user_groups ug where ug.user_id::text=$1 and ug.group_id::text=$3)))
		    or ($3='' and exists(select 1 from user_groups ug where ug.user_id::text=$1 and ug.group_id=t.group_id)))
		  and (coalesce(array_length(t.model_whitelist,1),0)=0 or $2 = any(t.model_whitelist))
	)
	select coalesce((select bool_or(req_ok and credit_ok) from candidates), false),
	       (select id from candidates order by (req_ok and credit_ok) desc, level desc, product_sort, created_at desc limit 1),
	       coalesce((select overage_policy from candidates order by (req_ok and credit_ok) desc, level desc, product_sort, created_at desc limit 1), 'allow_wallet')`, key.userID, model, key.groupID, key.subscriptionID).Scan(&access.Covered, &subscriptionID, &access.OveragePolicy)
	if err != nil {
		return subscriptionAccess{Covered: false, OveragePolicy: "allow_wallet"}, err
	}
	if subscriptionID != nil {
		access.SubscriptionID = *subscriptionID
	}
	return access, nil
}

// subscriptionStatusClause maps an admin status filter to the SQL predicate the
// batch actions use.
func subscriptionStatusClause(status string) (string, bool) {
	switch status {
	case "", "active":
		return "status='active'", true
	case "inactive":
		return "status in ('pending','expired','cancelled')", true
	case "all":
		return "status in ('pending','active','expired','cancelled')", true
	default:
		return "", false
	}
}
