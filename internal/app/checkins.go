package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) accountCheckin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RiskContextID string `json:"risk_context_id"`
		geetestPayload
		corptchaPayload
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid check-in request")
		return
	}
	if err := s.verifyCaptcha(r.Context(), in.geetestPayload, in.corptchaPayload, captchaPurposeCheckin); err != nil {
		writeError(w, http.StatusForbidden, "captcha_failed", err.Error())
		return
	}
	signals, err := s.rewardSignals(w, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not check account risk")
		return
	}
	account := accountFromContext(r)
	tx, err := s.beginRewardRiskTx(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not start check-in")
		return
	}
	defer tx.Rollback(r.Context())
	var enabled bool
	err = tx.QueryRow(r.Context(), `select enabled from users where id=$1 for update`, account.userID).Scan(&enabled)
	if err == pgx.ErrNoRows || (err == nil && !enabled) {
		writeError(w, http.StatusForbidden, "account_restricted", "account is restricted")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not lock account")
		return
	}
	var date string
	if err = tx.QueryRow(r.Context(), `select (clock_timestamp() at time zone 'UTC')::date::text`).Scan(&date); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not check check-in date")
		return
	}
	var existingStatus, existingReward string
	var existingStreak int
	err = tx.QueryRow(r.Context(), `select streak,reward::text,reward_status from user_checkins where user_id=$1 and checkin_date=$2`, account.userID, date).Scan(&existingStreak, &existingReward, &existingStatus)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"checked_in": true, "already_checked_in": true, "checkin_date": date, "streak": existingStreak, "reward": 0, "reward_status": existingStatus, "scheduled_reward": json.Number(existingReward)})
		return
	}
	if err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not check check-in status")
		return
	}
	var previousDate *time.Time
	var previousStreak int
	if err = tx.QueryRow(r.Context(), `select checkin_date,streak from user_checkins where user_id=$1 and reward_status<>'withdrawn' order by checkin_date desc limit 1`, account.userID).Scan(&previousDate, &previousStreak); err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in history")
		return
	}
	streak := 1
	if previousDate != nil && previousDate.UTC().AddDate(0, 0, 1).Format("2006-01-02") == date {
		streak = previousStreak + 1
	}
	var reward string
	if err = tx.QueryRow(r.Context(), `select (checkin_base_reward+least($1::integer-1,checkin_max_bonus_days-1)*checkin_streak_bonus)::numeric(20,8)::text from site_settings where id=true`, streak).Scan(&reward); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in rewards")
		return
	}
	sourceID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save check-in")
		return
	}
	signals, err = s.consumeRewardContextTx(r.Context(), tx, strings.TrimSpace(in.RiskContextID), "checkin", signals, account.userID)
	if err != nil {
		if errors.Is(err, errRiskContext) {
			writeError(w, http.StatusBadRequest, "invalid_risk_context", "invalid reward risk context")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not check account risk")
		}
		return
	}
	var inviterID string
	err = tx.QueryRow(r.Context(), `select inviter_id::text from invitations where invitee_id=$1`, account.userID).Scan(&inviterID)
	if err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not check invitation history")
		return
	}
	decision, err := s.evaluateRewardRiskTx(r.Context(), tx, signals, account.userID, "checkin", sourceID, inviterID)
	if err != nil {
		if errors.Is(err, errRiskAccountRestricted) {
			writeError(w, http.StatusForbidden, "account_restricted", "account is restricted")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not check account risk")
		}
		return
	}
	status, err := s.createRewardClaimTx(r.Context(), tx, decision, account.userID, account.userID, "checkin", sourceID, reward)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create check-in reward")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into user_checkins(user_id,checkin_date,streak,reward,reward_source_id,reward_status,created_at) values($1,$2,$3,$4,$5,$6,clock_timestamp())`, account.userID, date, streak, reward, sourceID, status); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save check-in")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not complete check-in")
		return
	}
	if decision.Action == "ban" {
		writeError(w, http.StatusForbidden, "account_restricted", "account is restricted")
		return
	}
	credited := json.Number("0")
	if status == "credited" {
		credited = json.Number(reward)
	}
	writeJSON(w, http.StatusOK, map[string]any{"checked_in": true, "already_checked_in": false, "checkin_date": date, "streak": streak, "reward": credited, "reward_status": status, "scheduled_reward": json.Number(reward)})
}

func (s *Service) accountCheckinStatus(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select checkin_date,streak,reward,created_at,reward_status from user_checkins where user_id=$1 order by checkin_date desc limit 30`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in history")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var date, created any
		var streak int
		var reward any
		var status string
		if rows.Scan(&date, &streak, &reward, &created, &status) == nil {
			data = append(data, map[string]any{"checkin_date": date, "streak": streak, "reward": reward, "created_at": created, "reward_status": status})
		}
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in history")
		return
	}
	var checkedIn bool
	if err := s.db.QueryRow(r.Context(), `select exists(select 1 from user_checkins where user_id=$1 and checkin_date=(now() at time zone 'UTC')::date)`, account.userID).Scan(&checkedIn); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checked_in": checkedIn, "data": data})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
