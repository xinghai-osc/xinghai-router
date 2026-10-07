package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) listAdminCheckins(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	where := ""
	args := []any{}
	if q != "" {
		where = " where u.email ilike $1 or u.name ilike $1 or u.id::text=$2"
		args = append(args, "%"+q+"%", q)
	}
	var total int
	if err := s.db.QueryRow(r.Context(), "select count(*) from user_checkins c join users u on u.id=c.user_id"+where, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in count")
		return
	}
	args = append(args, pageSize, offset)
	rows, err := s.db.Query(r.Context(), `select c.user_id::text,u.email,u.name,c.checkin_date,c.streak,c.reward::text,c.created_at,c.reward_status
		from user_checkins c join users u on u.id=c.user_id`+where+` order by c.checkin_date desc,c.created_at desc limit $`+strconv.Itoa(len(args)-1)+` offset $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-ins")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var userID, email, name, date, reward, status string
		var streak int
		var created any
		if rows.Scan(&userID, &email, &name, &date, &streak, &reward, &created, &status) == nil {
			data = append(data, map[string]any{"user_id": userID, "email": email, "user_name": name, "checkin_date": date, "streak": streak, "reward": reward, "created_at": created, "reward_status": status})
		}
	}
	if err = rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-ins")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func (s *Service) withdrawAdminCheckin(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("user_id"))
	date := strings.TrimSpace(r.PathValue("date"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid check-in date")
		return
	}
	tx, err := s.beginRewardRiskTx(r.Context(), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not withdraw check-in")
		return
	}
	defer tx.Rollback(r.Context())
	if _, _, err = lockUserMutation(r.Context(), tx, accountFromContext(r).userID, userID, userMutation{balance: true}, false); err != nil {
		writeUserMutationError(w, err)
		return
	}
	var reward, status, sourceID string
	err = tx.QueryRow(r.Context(), `select reward::text,reward_status,coalesce(reward_source_id::text,'') from user_checkins where user_id=$1 and checkin_date=$2 for update`, userID, date).Scan(&reward, &status, &sourceID)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "check-in record not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in")
		return
	}
	if status == "withdrawn" {
		writeJSON(w, http.StatusOK, map[string]any{"withdrawn": true, "reward": 0})
		return
	}
	var claimID string
	if sourceID != "" {
		err = tx.QueryRow(r.Context(), `select id::text,status,amount::text from reward_claims where source='checkin' and source_id=$1 and user_id=$2 for update`, sourceID, userID).Scan(&claimID, &status, &reward)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in reward")
			return
		}
	}
	withdrawnReward := json.Number("0")
	if status == "credited" {
		var positive bool
		if err = tx.QueryRow(r.Context(), `select $1::numeric>0`, reward).Scan(&positive); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load check-in reward")
			return
		}
		if positive {
			if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict(user_id) do nothing`, userID); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not load wallet")
				return
			}
			var balanceAfter string
			err = tx.QueryRow(r.Context(), `update user_wallets set balance=balance-$1::numeric,updated_at=clock_timestamp() where user_id=$2 and balance-reserved >= $1::numeric returning balance::text`, reward, userID).Scan(&balanceAfter)
			if err == pgx.ErrNoRows {
				writeError(w, http.StatusBadRequest, "invalid_request", "insufficient available balance to withdraw check-in reward")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not withdraw reward")
				return
			}
			ledgerID, err := randomID()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not record withdrawal")
				return
			}
			if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,-$3::numeric,$4::numeric,'adjustment',$5,$6)`, ledgerID, userID, reward, balanceAfter, userID+":"+date, "Admin withdrew check-in reward"); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "could not record withdrawal")
				return
			}
			withdrawnReward = json.Number(reward)
		}
	}
	if claimID != "" {
		if _, err = tx.Exec(r.Context(), `update reward_claims set status='withdrawn',updated_at=clock_timestamp() where id=$1`, claimID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not withdraw check-in reward")
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `update user_checkins set reward_status='withdrawn' where user_id=$1 and checkin_date=$2`, userID, date); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not withdraw check-in")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not withdraw check-in")
		return
	}
	s.audit(r, "checkin.withdrawn", "user", userID, map[string]any{"checkin_date": date, "reward": withdrawnReward})
	writeJSON(w, http.StatusOK, map[string]any{"withdrawn": true, "reward": withdrawnReward})
}
