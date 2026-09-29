package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

func rewardRiskListWhere(r *http.Request, kind string) (string, []any, error) {
	where := " where true"
	args := []any{}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 200 {
		return "", nil, errors.New("search is too long")
	}
	if q != "" {
		args = append(args, "%"+q+"%", q)
		where += " and (u.name ilike $1 or u.email ilike $1 or u.id::text=$2)"
	}
	appendFilter := func(column, value string) {
		args = append(args, value)
		where += fmt.Sprintf(" and %s=$%d", column, len(args))
	}
	status := r.URL.Query().Get("status")
	switch kind {
	case "claims":
		if status != "" {
			switch status {
			case "pending", "credited", "rejected", "withdrawn":
				appendFilter("c.status", status)
			default:
				return "", nil, errors.New("invalid reward status")
			}
		}
		if source := r.URL.Query().Get("source"); source != "" {
			if source != "invitation" && source != "checkin" {
				return "", nil, errors.New("invalid reward source")
			}
			appendFilter("c.source", source)
		}
	case "events":
		if v := r.URL.Query().Get("decision"); v != "" {
			switch v {
			case "allow", "review", "ban":
				appendFilter("c.decision", v)
			default:
				return "", nil, errors.New("invalid decision")
			}
		}
	case "bans":
		switch status {
		case "", "all":
		case "active":
			where += " and c.released_at is null"
		case "released":
			where += " and c.released_at is not null"
		default:
			return "", nil, errors.New("invalid ban status")
		}
	}
	return where, args, nil
}

func (s *Service) listRewardRiskClaims(w http.ResponseWriter, r *http.Request) {
	s.listRewardRiskRecords(w, r, "claims")
}
func (s *Service) listRewardRiskEvents(w http.ResponseWriter, r *http.Request) {
	s.listRewardRiskRecords(w, r, "events")
}
func (s *Service) listRewardRiskBans(w http.ResponseWriter, r *http.Request) {
	s.listRewardRiskRecords(w, r, "bans")
}

func (s *Service) listRewardRiskRecords(w http.ResponseWriter, r *http.Request, kind string) {
	where, args, err := rewardRiskListWhere(r, kind)
	if err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	page, size, offset := listPage(r)
	table, columns, order := "reward_claims", `c.id::text,c.user_id::text,u.name,u.email,c.origin_user_id::text,c.source,c.source_id,c.amount::text,c.status,coalesce(o.reasons,'[]'::jsonb),c.created_at,c.updated_at`, "c.created_at"
	join := " left join risk_observations o on o.id=c.risk_event_id"
	switch kind {
	case "events":
		table = "risk_observations"
		columns = `c.id::text,c.user_id::text,u.name,u.email,c.action,c.decision,c.reasons,c.details,c.created_at`
		join = ""
	case "bans":
		table = "risk_bans"
		columns = `c.id::text,c.user_id::text,u.name,u.email,c.rule_version,c.details,c.observation_id::text,c.banned_at,c.released_at,c.released_by::text,c.release_reason`
		order = "c.banned_at"
		join = ""
	}
	from := " from " + table + " c join users u on u.id=c.user_id"
	var total int
	if err = s.db.QueryRow(r.Context(), "select count(*)"+from+where, args...).Scan(&total); err != nil {
		writeError(w, 500, "internal_error", "could not load risk records")
		return
	}
	args = append(args, size, offset)
	rows, err := s.db.Query(r.Context(), "select "+columns+from+join+where+" order by "+order+" desc,c.id desc"+fmt.Sprintf(" limit $%d offset $%d", len(args)-1, len(args)), args...)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load risk records")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, userID, name, email string
		var created, updated any
		var reasons, details json.RawMessage
		record := map[string]any{}
		switch kind {
		case "claims":
			var origin, source, sourceID, amount, status string
			err = rows.Scan(&id, &userID, &name, &email, &origin, &source, &sourceID, &amount, &status, &reasons, &created, &updated)
			record = map[string]any{"origin_user_id": origin, "source": source, "source_id": sourceID, "amount": amount, "status": status, "reasons": reasons, "created_at": created, "updated_at": updated}
		case "events":
			var action, decision string
			err = rows.Scan(&id, &userID, &name, &email, &action, &decision, &reasons, &details, &created)
			record = map[string]any{"action": action, "decision": decision, "reasons": reasons, "details": details, "created_at": created}
		case "bans":
			var version int
			var observationID, releasedBy *string
			var releaseReason string
			err = rows.Scan(&id, &userID, &name, &email, &version, &details, &observationID, &created, &updated, &releasedBy, &releaseReason)
			record = map[string]any{"rule_version": version, "details": details, "observation_id": observationID, "banned_at": created, "released_at": updated, "released_by": releasedBy, "release_reason": releaseReason}
		}
		if err != nil {
			writeError(w, 500, "internal_error", "could not read risk record")
			return
		}
		record["id"] = id
		record["user_id"] = userID
		record["user_name"] = name
		record["email"] = email
		data = append(data, record)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal_error", "could not load risk records")
		return
	}
	writeJSON(w, 200, map[string]any{"data": data, "total": total, "page": page, "page_size": size})
}

func (s *Service) approveRewardRiskClaim(w http.ResponseWriter, r *http.Request) {
	s.reviewRewardRiskClaim(w, r, "credited")
}
func (s *Service) rejectRewardRiskClaim(w http.ResponseWriter, r *http.Request) {
	s.reviewRewardRiskClaim(w, r, "rejected")
}

func (s *Service) reviewRewardRiskClaim(w http.ResponseWriter, r *http.Request, target string) {
	id := r.PathValue("id")
	var in struct {
		Reason string `json:"reason"`
	}
	if !validRiskUUID(id) || decode(r, &in) != nil || len(in.Reason) > 500 {
		writeError(w, 400, "invalid_request", "invalid reward review")
		return
	}
	tx, err := s.beginRewardRiskTx(r.Context(), false)
	if err != nil {
		writeError(w, 500, "internal_error", "could not review reward")
		return
	}
	defer tx.Rollback(r.Context())
	var userID, originID, status, source, sourceID string
	err = tx.QueryRow(r.Context(), `select user_id::text,origin_user_id::text,status,source,source_id from reward_claims where id=$1`, id).Scan(&userID, &originID, &status, &source, &sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "reward not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal_error", "could not load reward")
		return
	}
	actorID := accountFromContext(r).userID
	if _, _, err = lockUserMutation(r.Context(), tx, actorID, userID, userMutation{balance: true}, false); err != nil {
		writeUserMutationError(w, err)
		return
	}
	if _, err = lockUserAccess(r.Context(), tx, originID); err != nil {
		writeError(w, 500, "internal_error", "could not load reward origin")
		return
	}
	if err = tx.QueryRow(r.Context(), `select status from reward_claims where id=$1 for update`, id).Scan(&status); err != nil {
		writeError(w, 500, "internal_error", "could not lock reward")
		return
	}
	if status == target {
		writeJSON(w, 200, map[string]string{"id": id, "status": status})
		return
	}
	if status != "pending" {
		writeError(w, 409, "reward_already_processed", "reward has already been processed")
		return
	}
	if target == "credited" {
		err = s.creditRewardClaimTx(r.Context(), tx, id)
	} else {
		_, err = tx.Exec(r.Context(), `update reward_claims set status='rejected',updated_at=clock_timestamp() where id=$1`, id)
		if err == nil && source == "checkin" {
			_, err = tx.Exec(r.Context(), `update user_checkins set reward_status='rejected' where reward_source_id=$1::uuid and user_id=$2`, sourceID, userID)
		}
	}
	if errors.Is(err, errRiskAccountRestricted) {
		writeError(w, 409, "account_restricted", "reward account is restricted; review the account first")
		return
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `update reward_claims set reviewed_by=$2,review_reason=$3,updated_at=clock_timestamp() where id=$1`, id, actorID, strings.TrimSpace(in.Reason))
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeError(w, 500, "internal_error", "could not review reward")
		return
	}
	s.audit(r, "reward_risk.claim_"+target, "reward_claim", id, map[string]any{"user_id": userID, "reason": in.Reason})
	writeJSON(w, 200, map[string]string{"id": id, "status": target})
}
