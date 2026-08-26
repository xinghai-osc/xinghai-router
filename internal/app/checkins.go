package app

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) accountCheckin(w http.ResponseWriter, r *http.Request) {
	var in struct {
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
	account := accountFromContext(r)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	date := today.Format("2006-01-02")

	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not start check-in")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `select id from users where id=$1 for update`, account.userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not lock account")
		return
	}
	var existing bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from user_checkins where user_id=$1 and checkin_date=$2)`, account.userID, date).Scan(&existing); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not check check-in status")
		return
	}
	if existing {
		writeJSON(w, http.StatusOK, map[string]any{"checked_in": true, "already_checked_in": true, "checkin_date": date})
		return
	}

	var previousDate *time.Time
	var previousStreak int
	if err = tx.QueryRow(r.Context(), `select checkin_date,streak from user_checkins where user_id=$1 order by checkin_date desc limit 1`, account.userID).Scan(&previousDate, &previousStreak); err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in history")
		return
	}
	streak := 1
	if previousDate != nil && previousDate.UTC().AddDate(0, 0, 1).Format("2006-01-02") == date {
		streak = previousStreak + 1
	}
	var baseReward, streakBonus float64
	var maxBonusDays int
	if err = tx.QueryRow(r.Context(), `select checkin_base_reward,checkin_streak_bonus,checkin_max_bonus_days from site_settings where id=true`).Scan(&baseReward, &streakBonus, &maxBonusDays); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in rewards")
		return
	}
	reward := baseReward + float64(minInt(streak-1, maxBonusDays-1))*streakBonus
	if _, err = tx.Exec(r.Context(), `insert into user_checkins(user_id,checkin_date,streak,reward) values($1,$2,$3,$4)`, account.userID, date, streak, reward); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save check-in")
		return
	}
	checkinID := account.userID + ":" + date
	if err = s.creditWalletTx(r.Context(), tx, account.userID, reward, "checkin", checkinID, "Daily check-in reward"); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not credit check-in reward")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not complete check-in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checked_in": true, "already_checked_in": false, "checkin_date": date, "streak": streak, "reward": reward})
}

func (s *Service) accountCheckinStatus(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select checkin_date,streak,reward,created_at from user_checkins where user_id=$1 order by checkin_date desc limit 30`, account.userID)
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
		if rows.Scan(&date, &streak, &reward, &created) == nil {
			data = append(data, map[string]any{"checkin_date": date, "streak": streak, "reward": reward, "created_at": created})
		}
	}
	today := time.Now().UTC().Format("2006-01-02")
	var checkedIn bool
	if err := s.db.QueryRow(r.Context(), `select exists(select 1 from user_checkins where user_id=$1 and checkin_date=$2)`, account.userID, today).Scan(&checkedIn); err != nil {
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
