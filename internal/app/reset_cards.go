package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	maxResetCardBatchSize = 1000
	maxResetCardNoteLength = 500
)

type resetCard struct {
	ID             string `json:"id"`
	BatchID        string `json:"batch_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	UserEmail      string `json:"user_email"`
	UserName       string `json:"user_name"`
	PlanID         string `json:"plan_id"`
	PlanName       string `json:"plan_name"`
	Enabled        bool   `json:"enabled"`
	ExpiresAt      any    `json:"expires_at"`
	Note           string `json:"note"`
	UsedBy         any    `json:"used_by"`
	UsedAt         any    `json:"used_at"`
	CreatedAt      any    `json:"created_at"`
	UpdatedAt      any    `json:"updated_at"`
}

// ---- Admin: batch-issue reset cards to a subscription ----

func (s *Service) createResetCards(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SubscriptionID string `json:"subscription_id"`
		Quantity       int    `json:"quantity"`
		ExpiresAt      string `json:"expires_at"`
		Note           string `json:"note"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	subID := strings.TrimSpace(in.SubscriptionID)
	if subID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "subscription_id is required")
		return
	}
	quantity := in.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > maxResetCardBatchSize {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("quantity must be between 1 and %d", maxResetCardBatchSize))
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
	if len(note) > maxResetCardNoteLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
		return
	}
	var subExists bool
	if err := s.db.QueryRow(r.Context(), `select exists(select 1 from user_subscriptions where id=$1)`, subID).Scan(&subExists); err != nil || !subExists {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	batchID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	quantityCreated, err := s.insertResetCards(r.Context(), batchID, []string{subID}, quantity, expiresAt, note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	s.audit(r, "reset_cards.created", "user_subscription", subID, map[string]any{"quantity": quantity, "batch_id": batchID})
	writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "quantity": quantityCreated})
}

// ---- Admin: batch-issue reset cards to every subscription of a plan ----

func (s *Service) createResetCardsByPlan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlanID    string `json:"plan_id"`
		Status    string `json:"status"`
		Quantity  int    `json:"quantity"`
		ExpiresAt string `json:"expires_at"`
		Note      string `json:"note"`
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
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "inactive", "all":
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of active, inactive, all")
		return
	}
	quantity := in.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > maxResetCardBatchSize {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("quantity must be between 1 and %d", maxResetCardBatchSize))
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
	if len(note) > maxResetCardNoteLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
		return
	}
	var planExists bool
	if err := s.db.QueryRow(r.Context(), `select exists(select 1 from subscription_plans where id=$1)`, planID).Scan(&planExists); err != nil || !planExists {
		writeError(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	statusClause := "status='active'"
	if status == "inactive" {
		statusClause = "status in ('pending','expired','cancelled')"
	} else if status == "all" {
		statusClause = "status in ('pending','active','expired','cancelled')"
	}
	rows, err := s.db.Query(r.Context(), `select id from user_subscriptions where plan_id=$1 and `+statusClause, planID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	var subIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		subIDs = append(subIDs, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	if len(subIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"batch_id": "", "subscriptions": 0, "quantity": 0})
		return
	}
	batchID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	quantityCreated, err := s.insertResetCards(r.Context(), batchID, subIDs, quantity, expiresAt, note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	s.audit(r, "reset_cards.batch_created", "subscription_plans", planID, map[string]any{"subscriptions": len(subIDs), "per_subscription": quantity, "quantity": quantityCreated, "status": status})
	writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "subscriptions": len(subIDs), "quantity": quantityCreated})
}

// ---- Admin: batch-issue reset cards to every subscription ----

func (s *Service) createResetCardsAll(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status    string `json:"status"`
		Quantity  int    `json:"quantity"`
		ExpiresAt string `json:"expires_at"`
		Note      string `json:"note"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid payload")
		return
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "inactive", "all":
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be one of active, inactive, all")
		return
	}
	quantity := in.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > maxResetCardBatchSize {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("quantity must be between 1 and %d", maxResetCardBatchSize))
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
	if len(note) > maxResetCardNoteLength {
		writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
		return
	}
	statusClause := "status='active'"
	if status == "inactive" {
		statusClause = "status in ('pending','expired','cancelled')"
	} else if status == "all" {
		statusClause = "status in ('pending','active','expired','cancelled')"
	}
	rows, err := s.db.Query(r.Context(), `select id from user_subscriptions where `+statusClause)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	var subIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
			return
		}
		subIDs = append(subIDs, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscriptions")
		return
	}
	if len(subIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"batch_id": "", "subscriptions": 0, "quantity": 0})
		return
	}
	batchID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	quantityCreated, err := s.insertResetCards(r.Context(), batchID, subIDs, quantity, expiresAt, note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create cards")
		return
	}
	s.audit(r, "reset_cards.batch_created_all", "user_subscription", "", map[string]any{"subscriptions": len(subIDs), "per_subscription": quantity, "quantity": quantityCreated, "status": status})
	writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "subscriptions": len(subIDs), "quantity": quantityCreated})
}

// insertResetCards creates quantity cards per subscription in one transaction
// and returns the total number of cards created.
func (s *Service) insertResetCards(ctx context.Context, batchID string, subIDs []string, quantity int, expiresAt *time.Time, note string) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	total := 0
	for _, subID := range subIDs {
		for i := 0; i < quantity; i++ {
			id, err := randomID()
			if err != nil {
				return 0, err
			}
			if _, err = tx.Exec(ctx, `insert into reset_cards(id,batch_id,subscription_id,expires_at,note) values($1,$2,$3,$4,$5)`,
				id, batchID, subID, expiresAt, note); err != nil {
				return 0, err
			}
			total++
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return total, nil
}

// ---- Admin: list reset cards ----

func (s *Service) listResetCards(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	var total int
	_ = s.db.QueryRow(r.Context(), `select count(*) from reset_cards`).Scan(&total)
	rows, err := s.db.Query(r.Context(), `select rc.id,rc.batch_id::text,rc.subscription_id::text,us.user_id::text,coalesce(u.email,''),coalesce(u.name,''),us.plan_id::text,coalesce(p.name,''),rc.enabled,rc.expires_at,rc.note,rc.used_by::text,rc.used_at,rc.created_at,rc.updated_at from reset_cards rc join user_subscriptions us on us.id=rc.subscription_id join subscription_plans p on p.id=us.plan_id left join users u on u.id=us.user_id order by rc.created_at desc limit $1 offset $2`, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load cards")
		return
	}
	defer rows.Close()
	data := []resetCard{}
	for rows.Next() {
		var c resetCard
		if err = rows.Scan(&c.ID, &c.BatchID, &c.SubscriptionID, &c.UserID, &c.UserEmail, &c.UserName, &c.PlanID, &c.PlanName, &c.Enabled, &c.ExpiresAt, &c.Note, &c.UsedBy, &c.UsedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load cards")
			return
		}
		data = append(data, c)
	}
	writePaged(w, data, total, page, pageSize)
}

// ---- Admin: update a single card ----

func (s *Service) updateResetCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Enabled   *bool   `json:"enabled"`
		ExpiresAt *string `json:"expires_at"`
		Note      *string `json:"note"`
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
			sets = append(sets, "expires_at=null")
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
		if len(note) > maxResetCardNoteLength {
			writeError(w, http.StatusBadRequest, "invalid_request", "note must be at most 500 characters")
			return
		}
		sets = append(sets, fmt.Sprintf("note=$%d", argIdx))
		args = append(args, note)
		argIdx++
	}
	if len(sets) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "no fields to update")
		return
	}
	sets = append(sets, "updated_at=now()")
	args = append(args, id)
	query := fmt.Sprintf("update reset_cards set %s where id=$%d", strings.Join(sets, ", "), argIdx)
	result, err := s.db.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update card")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "card not found")
		return
	}
	s.audit(r, "reset_card.updated", "reset_card", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: delete a card ----

func (s *Service) deleteResetCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.Exec(r.Context(), `delete from reset_cards where id=$1 and used_at is null`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete card")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, http.StatusConflict, "conflict", "card not found or already used")
		return
	}
	s.audit(r, "reset_card.deleted", "reset_card", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Account: consume a reset card for one of my subscriptions ----

func (s *Service) useResetCard(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	subID := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not use reset card")
		return
	}
	defer tx.Rollback(r.Context())
	var subOwner string
	var subStatus string
	err = tx.QueryRow(r.Context(), `select user_id,status from user_subscriptions where id=$1 for update`, subID).Scan(&subOwner, &subStatus)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load subscription")
		return
	}
	if subOwner != account.userID {
		writeError(w, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	if subStatus != "active" {
		writeError(w, http.StatusConflict, "subscription_not_active", "subscription is not active")
		return
	}
	var cardID string
	err = tx.QueryRow(r.Context(), `select id from reset_cards where subscription_id=$1 and enabled and used_at is null and (expires_at is null or expires_at>now()) order by created_at asc limit 1 for update`, subID).Scan(&cardID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusConflict, "no_reset_card", "no available reset card for this subscription")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load reset card")
		return
	}
	if err = s.initSubscriptionCountersTx(r.Context(), tx, subID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset subscription quota")
		return
	}
	if _, err = tx.Exec(r.Context(), `update reset_cards set used_by=$1,used_at=now(),updated_at=now() where id=$2`, account.userID, cardID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not mark card used")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not use reset card")
		return
	}
	s.subscriptionCache.clear()
	s.audit(r, "reset_card.used", "reset_card", cardID, map[string]any{"subscription_id": subID})
	writeJSON(w, http.StatusOK, map[string]any{"used": true, "card_id": cardID})
}

// ---- Account: reset-card availability for my subscriptions ----

func (s *Service) accountResetCards(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select rc.id,rc.subscription_id::text,rc.enabled,rc.expires_at,rc.note,rc.used_at,rc.created_at from reset_cards rc join user_subscriptions us on us.id=rc.subscription_id where us.user_id=$1 order by rc.created_at desc limit 200`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load reset cards")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, subID, note string
		var enabled bool
		var expires, used, created any
		if rows.Scan(&id, &subID, &enabled, &expires, &note, &used, &created) == nil {
			available := enabled && used == nil && (expires == nil || expiresAfterNow(expires))
			data = append(data, map[string]any{"id": id, "subscription_id": subID, "enabled": enabled, "expires_at": expires, "note": note, "used_at": used, "created_at": created, "available": available})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func expiresAfterNow(v any) bool {
	t, ok := v.(time.Time)
	return ok && t.After(time.Now())
}

// ---- Admin: direct user quota reset ----

func (s *Service) adminResetUserQuota(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset quota")
		return
	}
	defer tx.Rollback(r.Context())
	var exists bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, userID).Scan(&exists); err != nil || !exists {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	resetCount, err := s.resetUserQuotaTx(r.Context(), tx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset quota")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not reset quota")
		return
	}
	s.audit(r, "user.quotas_reset", "user", userID, map[string]any{"reset_subscriptions": resetCount})
	s.subscriptionCache.clear()
	s.invalidateQuotaAbsence()
	writeJSON(w, http.StatusOK, map[string]any{"reset_subscriptions": resetCount})
}

// resetUserQuotaTx refills the remaining request/credit counters of every
// active subscription the user owns within the supplied transaction, mirroring
// the admin per-subscription reset action. Returns the number of subscriptions
// whose counters were reset.
func (s *Service) resetUserQuotaTx(ctx context.Context, tx pgx.Tx, userID string) (int, error) {
	rows, err := tx.Query(ctx, `select id from user_subscriptions where user_id=$1 and status='active'`, userID)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err = s.initSubscriptionCountersTx(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}
