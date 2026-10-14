package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	redemptionCodeLength    = 16
	redemptionCodePrefix    = "XH"
	maxRedemptionBatchSize  = 1000
	maxRedemptionAmount     = 1_000_000.0
	maxRedemptionPeriodDays = 3650
	maxRedemptionNoteLength = 500
)

type redemptionCode struct {
	ID         string `json:"id"`
	BatchID    string `json:"batch_id"`
	Code       string `json:"code"`
	RewardType string `json:"reward_type"`
	Amount        string `json:"amount"`
	TierID        string `json:"tier_id"`
	TierName      string `json:"tier_name"`
	BillingPeriod string `json:"billing_period"`
	PeriodDays    *int   `json:"period_days"`
	MaxUses    int    `json:"max_uses"`
	UsedCount  int    `json:"used_count"`
	ExpiresAt  any    `json:"expires_at"`
	Enabled    bool   `json:"enabled"`
	Note       string `json:"note"`
	RedeemedBy any    `json:"redeemed_by"`
	RedeemedAt any    `json:"redeemed_at"`
	CreatedAt  any    `json:"created_at"`
	UpdatedAt  any    `json:"updated_at"`
}

type redemptionCodeRedemption struct {
	ID             string `json:"id"`
	CodeID         string `json:"code_id"`
	Code           string `json:"code"`
	UserID         string `json:"user_id"`
	UserEmail      string `json:"user_email"`
	UserName       string `json:"user_name"`
	Amount         string `json:"amount"`
	TierID         string `json:"tier_id"`
	TierName       string `json:"tier_name"`
	SubscriptionID string `json:"subscription_id"`
	CreatedAt      any    `json:"created_at"`
}

// ---- Admin: batch-create redemption codes ----

func (s *Service) createRedemptionCodes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RewardType    string `json:"reward_type"`
		Amount        string `json:"amount"`
		TierID        string `json:"tier_id"`
		PlanID        string `json:"plan_id"`
		BillingPeriod string `json:"billing_period"`
		PeriodDays    *int   `json:"period_days"`
		MaxUses    int    `json:"max_uses"`
		Quantity   int    `json:"quantity"`
		ExpiresAt  string `json:"expires_at"`
		Note       string `json:"note"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	rewardType := strings.ToLower(strings.TrimSpace(in.RewardType))
	if rewardType != "balance" && rewardType != "subscription" {
		writeError(w, http.StatusBadRequest, "invalid_request", "reward_type must be balance or subscription")
		return
	}
	var amount float64
	if rewardType == "balance" {
		amt, _, ok := parsePaymentAmount(in.Amount)
		if !ok || amt <= 0 || amt > int64(maxRedemptionAmount*100) {
			writeError(w, http.StatusBadRequest, "invalid_request", "amount must be a positive decimal up to 1000000.00")
			return
		}
		amount = float64(amt) / 100
	}
	var tierID, tierName, billing string
	if rewardType == "subscription" {
		tierID = strings.TrimSpace(in.TierID)
		if tierID == "" {
			tierID = strings.TrimSpace(in.PlanID)
		}
		if tierID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "tier_id is required for subscription codes")
			return
		}
		billing = strings.ToLower(strings.TrimSpace(in.BillingPeriod))
		requested := billing
		var productName, rawTierName string
		err := s.db.QueryRow(r.Context(), `select t.id::text,s.name,coalesce(t.name,''),coalesce((select p.billing_period from subscription_tier_prices p where p.tier_id=t.id and ($2='' or p.billing_period=$2) and p.enabled order by p.sort_order,p.billing_period limit 1),'month') from subscription_tiers t join subscriptions s on s.id=t.subscription_id where t.id=$1 and t.enabled and s.enabled`, tierID, requested).Scan(&tierID, &productName, &rawTierName, &billing)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "subscription tier not found or disabled")
			return
		}
		if requested != "" && requested != billing {
			writeError(w, http.StatusBadRequest, "invalid_request", "this subscription tier does not offer that billing period")
			return
		}
		tierName = subscriptionDisplayName(productName, rawTierName)
	}
	if in.PeriodDays != nil {
		if *in.PeriodDays <= 0 || *in.PeriodDays > maxRedemptionPeriodDays {
			writeError(w, http.StatusBadRequest, "invalid_request", "period_days must be between 1 and 3650")
			return
		}
	}
	maxUses := in.MaxUses
	if maxUses < 1 {
		maxUses = 1
	}
	quantity := in.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > maxRedemptionBatchSize {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("quantity must be between 1 and %d", maxRedemptionBatchSize))
		return
	}
	var expiresAt *time.Time
	if in.ExpiresAt != "" {
		t, err := parseRedemptionTimestamp(in.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be a valid timestamp")
			return
		}
		expiresAt = &t
	}
	note := strings.TrimSpace(in.Note)
	if len(note) > maxRedemptionNoteLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
		return
	}
	batchID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
		return
	}
	defer tx.Rollback(r.Context())
	codes := make([]string, 0, quantity)
	seen := make(map[string]bool, quantity)
	for i := 0; i < quantity; i++ {
		code, err := generateRedemptionCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
			return
		}
		for seen[code] {
			code, err = generateRedemptionCode()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
				return
			}
		}
		seen[code] = true
		id, err := randomID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
			return
		}
		var tierRef any
		if tierID != "" {
			tierRef = tierID
		}
		if _, err = tx.Exec(r.Context(), `insert into redemption_codes(id,batch_id,code,reward_type,amount,tier_id,tier_name,billing_period,period_days,max_uses,expires_at,note) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			id, batchID, code, rewardType, amount, tierRef, tierName, billing, in.PeriodDays, maxUses, expiresAt, note); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
			return
		}
		codes = append(codes, code)
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create codes")
		return
	}
	s.audit(r, "redemption_codes.created", "redemption_code", batchID, map[string]any{"reward_type": rewardType, "amount": amount, "tier_id": tierID, "quantity": quantity, "max_uses": maxUses})
	writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "codes": codes})
}

// ---- Admin: list redemption codes ----

func (s *Service) listRedemptionCodes(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	var total int
	_ = s.db.QueryRow(r.Context(), `select count(*) from redemption_codes`).Scan(&total)
	rows, err := s.db.Query(r.Context(), `select rc.id,rc.batch_id::text,rc.code,rc.reward_type,rc.amount::text,coalesce(rc.tier_id::text,''),coalesce(rc.tier_name,''),coalesce(rc.billing_period,'month'),rc.period_days,rc.max_uses,rc.used_count,rc.expires_at,rc.enabled,rc.note,rc.redeemed_by::text,rc.redeemed_at,rc.created_at,rc.updated_at from redemption_codes rc order by rc.created_at desc limit $1 offset $2`, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load codes")
		return
	}
	defer rows.Close()
	data := []redemptionCode{}
	for rows.Next() {
		var c redemptionCode
		var amountStr string
		if err = rows.Scan(&c.ID, &c.BatchID, &c.Code, &c.RewardType, &amountStr, &c.TierID, &c.TierName, &c.BillingPeriod, &c.PeriodDays, &c.MaxUses, &c.UsedCount, &c.ExpiresAt, &c.Enabled, &c.Note, &c.RedeemedBy, &c.RedeemedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load codes")
			return
		}
		c.Amount = amountStr
		data = append(data, c)
	}
	writePaged(w, data, total, page, pageSize)
}

// ---- Admin: update a single code ----

func (s *Service) updateRedemptionCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Enabled   *bool   `json:"enabled"`
		ExpiresAt *string `json:"expires_at"`
		Note      *string `json:"note"`
		MaxUses   *int    `json:"max_uses"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	sets := []string{}
	args := []any{}
	argIdx := 1
	if in.Enabled != nil {
		sets = append(sets, fmt.Sprintf("enabled=$%d", argIdx))
		args = append(args, *in.Enabled)
		argIdx++
	}
	if in.ExpiresAt != nil {
		if *in.ExpiresAt == "" {
			sets = append(sets, fmt.Sprintf("expires_at=null"))
		} else {
			t, err := parseRedemptionTimestamp(*in.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be a valid timestamp")
				return
			}
			sets = append(sets, fmt.Sprintf("expires_at=$%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if in.Note != nil {
		note := strings.TrimSpace(*in.Note)
		if len(note) > maxRedemptionNoteLength {
			writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
			return
		}
		sets = append(sets, fmt.Sprintf("note=$%d", argIdx))
		args = append(args, note)
		argIdx++
	}
	if in.MaxUses != nil {
		if *in.MaxUses < 1 {
			writeError(w, http.StatusBadRequest, "invalid_request", "max_uses must be at least 1")
			return
		}
		sets = append(sets, fmt.Sprintf("max_uses=$%d", argIdx))
		args = append(args, *in.MaxUses)
		argIdx++
	}
	if len(sets) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "no fields to update")
		return
	}
	sets = append(sets, "updated_at=now()")
	args = append(args, id)
	query := fmt.Sprintf("update redemption_codes set %s where id=$%d", strings.Join(sets, ", "), argIdx)
	result, err := s.db.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update code")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "code not found")
		return
	}
	s.audit(r, "redemption_code.updated", "redemption_code", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: delete a code ----

func (s *Service) deleteRedemptionCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.Exec(r.Context(), `delete from redemption_codes where id=$1 and used_count=0`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete code")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusConflict, "conflict", "code not found or already redeemed")
		return
	}
	s.audit(r, "redemption_code.deleted", "redemption_code", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: list redemptions for a code ----

func (s *Service) listCodeRedemptions(w http.ResponseWriter, r *http.Request) {
	codeID := r.PathValue("id")
	page, pageSize, offset := listPage(r)
	var total int
	_ = s.db.QueryRow(r.Context(), `select count(*) from redemption_code_redemptions where code_id=$1`, codeID).Scan(&total)
	rows, err := s.db.Query(r.Context(), `select rcr.id,rcr.code_id::text,rc.code,rcr.user_id::text,coalesce(u.email,''),coalesce(u.name,''),rcr.amount::text,coalesce(rcr.tier_id::text,''),coalesce(rc.tier_name,''),coalesce(rcr.subscription_id::text,''),rcr.created_at from redemption_code_redemptions rcr join redemption_codes rc on rc.id=rcr.code_id left join users u on u.id=rcr.user_id where rcr.code_id=$1 order by rcr.created_at desc limit $2 offset $3`, codeID, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load redemptions")
		return
	}
	defer rows.Close()
	data := []redemptionCodeRedemption{}
	for rows.Next() {
		var rc redemptionCodeRedemption
		var amountStr string
		if err = rows.Scan(&rc.ID, &rc.CodeID, &rc.Code, &rc.UserID, &rc.UserEmail, &rc.UserName, &amountStr, &rc.TierID, &rc.TierName, &rc.SubscriptionID, &rc.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load redemptions")
			return
		}
		rc.Amount = amountStr
		data = append(data, rc)
	}
	writePaged(w, data, total, page, pageSize)
}

// ---- Account: redeem a code ----

func (s *Service) redeemCode(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	var in struct {
		Code string `json:"code"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	code := strings.TrimSpace(in.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "code is required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not redeem code")
		return
	}
	defer tx.Rollback(r.Context())
	var id, rewardType, tierID, amountStr, billingPeriod string
	var maxUses, usedCount int
	var enabled bool
	var expiresAt *time.Time
	var periodDays *int
	err = tx.QueryRow(r.Context(), `select id,reward_type,coalesce(tier_id::text,''),amount::text,max_uses,used_count,enabled,expires_at,period_days,coalesce(billing_period,'month') from redemption_codes where code=$1 for update`, code).Scan(&id, &rewardType, &tierID, &amountStr, &maxUses, &usedCount, &enabled, &expiresAt, &periodDays, &billingPeriod)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "code not found")
		return
	}
	if !enabled {
		writeError(w, http.StatusConflict, "code_disabled", "code is disabled")
		return
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		writeError(w, http.StatusConflict, "code_expired", "code has expired")
		return
	}
	if usedCount >= maxUses {
		writeError(w, http.StatusConflict, "code_used", "code has been fully redeemed")
		return
	}
	var alreadyRedeemed bool
	_ = tx.QueryRow(r.Context(), `select exists(select 1 from redemption_code_redemptions where code_id=$1 and user_id=$2)`, id, account.userID).Scan(&alreadyRedeemed)
	if alreadyRedeemed {
		writeError(w, http.StatusConflict, "already_redeemed", "you have already redeemed this code")
		return
	}
	result := map[string]any{"reward_type": rewardType}
	if rewardType == "balance" {
		cents, _, ok := parsePaymentAmount(amountStr)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal_error", "invalid code amount")
			return
		}
		amount := float64(cents) / 100
		if err = s.creditWalletTx(r.Context(), tx, account.userID, amount, "redemption", id, "Redemption code"); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not credit wallet")
			return
		}
		result["amount"] = formatAmount(cents)
	} else {
		purchase, err := s.loadTierPurchase(r.Context(), tx, tierID, billingPeriod, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription tier")
			return
		}
		subID, _, err := s.activateSubscriptionTx(r.Context(), tx, account.userID, purchase, subscriptionActivation{PeriodDays: periodDays})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not grant subscription")
			return
		}
		result["subscription_id"] = subID
		result["tier_id"] = tierID
		result["tier_name"] = purchase.DisplayName
	}
	redemptionID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not record redemption")
		return
	}
	var subRef any
	if v, ok := result["subscription_id"]; ok {
		subRef = v
	}
	var tierRef any
	if tierID != "" {
		tierRef = tierID
	}
	if _, err = tx.Exec(r.Context(), `insert into redemption_code_redemptions(id,code_id,user_id,amount,tier_id,subscription_id) values($1,$2,$3,$4,$5,$6)`, redemptionID, id, account.userID, amountStr, tierRef, subRef); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not record redemption")
		return
	}
	now := time.Now()
	if usedCount+1 >= maxUses {
		if _, err = tx.Exec(r.Context(), `update redemption_codes set used_count=used_count+1,redeemed_by=$1,redeemed_at=$2,updated_at=now() where id=$3`, account.userID, now, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not update code")
			return
		}
	} else {
		if _, err = tx.Exec(r.Context(), `update redemption_codes set used_count=used_count+1,updated_at=now() where id=$1`, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not update code")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not redeem code")
		return
	}
	s.subscriptionCache.clear()
	s.audit(r, "redemption_code.redeemed", "redemption_code", id, map[string]any{"reward_type": rewardType, "amount": amountStr, "tier_id": tierID})
	writeJSON(w, http.StatusOK, result)
}

// ---- Account: list own redemptions ----

func (s *Service) accountRedemptions(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select rcr.id,rcr.code_id::text,rc.code,rcr.amount::text,coalesce(rcr.tier_id::text,''),coalesce(rc.tier_name,''),coalesce(rcr.subscription_id::text,''),rcr.created_at from redemption_code_redemptions rcr join redemption_codes rc on rc.id=rcr.code_id where rcr.user_id=$1 order by rcr.created_at desc limit 50`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load redemptions")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, codeID, code, amount, planID, planName, subID, created any
		if rows.Scan(&id, &codeID, &code, &amount, &planID, &planName, &subID, &created) == nil {
			data = append(data, map[string]any{"id": id, "code_id": codeID, "code": code, "amount": amount, "tier_id": planID, "tier_name": planName, "subscription_id": subID, "created_at": created})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// creditWalletTx credits a user's wallet within a transaction and records a
// ledger entry with the given kind.
func (s *Service) creditWalletTx(ctx context.Context, tx pgx.Tx, userID string, amount float64, kind, requestID, note string) error {
	if _, err := tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, userID); err != nil {
		return err
	}
	var balanceStr string
	if err := tx.QueryRow(ctx, `update user_wallets set balance=balance+$1::numeric,updated_at=now() where user_id=$2 returning balance::text`, amount, userID).Scan(&balanceStr); err != nil {
		return err
	}
	ledgerID, err := randomID()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,$5,$6,$7)`, ledgerID, userID, amount, balanceStr, kind, requestID, note); err != nil {
		return err
	}
	return nil
}

// parseRedemptionTimestamp accepts full RFC3339 timestamps (with seconds) and
// the "YYYY-MM-DDTHH:MM" format emitted by HTML datetime-local inputs (without
// seconds), so the admin console can submit either without client-side padding.
func parseRedemptionTimestamp(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02T15:04", value)
}

// generateRedemptionCode produces a human-friendly code: a fixed prefix, a
// random base62 body, and a dash separator for readability. Digits and letters
// are drawn from an unambiguous alphabet (no 0/O/1/I/l) so codes survive being
// read over the phone or retyped.
func generateRedemptionCode() (string, error) {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	body := make([]byte, redemptionCodeLength)
	buf := make([]byte, redemptionCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range body {
		body[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return redemptionCodePrefix + string(body[:4]) + "-" + string(body[4:10]) + "-" + string(body[10:16]), nil
}
