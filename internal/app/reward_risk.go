package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type rewardRiskDecision struct {
	ID      string
	Action  string
	Reasons []string
	Details map[string]any
}

type rewardRiskPeer struct {
	ID                            string
	Name                          string
	HTTP, Browser, RTC, IPInviter bool
	BurstBrowser, BurstIPInviter  bool
}

func rewardRiskCluster(cfg rewardRiskSettings, name string, peers []rewardRiskPeer) ([]string, map[string]any, bool) {
	normalized := normalizeRiskName(name)
	template := riskNameTemplate(normalized)
	similar, browserSimilar, ipInviterSimilar, rtcSimilar := 1, 1, 1, 1
	browserBurst, ipInviterBurst := 1, 1
	matches := []map[string]any{}
	for _, p := range peers {
		if p.BurstBrowser {
			browserBurst++
		}
		if p.BurstIPInviter {
			ipInviterBurst++
		}
		other := normalizeRiskName(p.Name)
		similarity := riskNameSimilarity(normalized, other)
		templateMatch := template != "" && template == riskNameTemplate(other)
		if similarity < cfg.UsernameSimilarity && !templateMatch {
			continue
		}
		similar++
		if p.BurstBrowser {
			browserSimilar++
		}
		if p.BurstIPInviter {
			ipInviterSimilar++
		}
		if p.RTC {
			rtcSimilar++
		}
		if len(matches) < 30 {
			matches = append(matches, map[string]any{"user_id": p.ID, "similarity": similarity, "template_match": templateMatch, "http_match": p.HTTP, "browser_match": p.Browser, "webrtc_match": p.RTC, "burst_browser": p.BurstBrowser, "burst_ip_inviter": p.BurstIPInviter})
		}
	}
	reasons := []string{}
	if similar >= cfg.SimilarAccounts {
		reasons = append(reasons, "username_cluster")
	}
	if rtcSimilar >= cfg.SimilarAccounts {
		reasons = append(reasons, "webrtc_username_cluster")
	}
	burst := browserBurst >= cfg.BurstAccounts || ipInviterBurst >= cfg.BurstAccounts
	if burst {
		reasons = append(reasons, "account_burst")
	}
	ban := browserBurst >= cfg.BurstAccounts && browserSimilar >= cfg.SimilarAccounts || ipInviterBurst >= cfg.BurstAccounts && ipInviterSimilar >= cfg.SimilarAccounts
	details := map[string]any{"version": cfg.Version, "similar_accounts": similar, "browser_burst_accounts": browserBurst, "ip_inviter_burst_accounts": ipInviterBurst, "matched_accounts": matches}
	return reasons, details, ban
}

func (s *Service) evaluateRewardRiskTx(ctx context.Context, tx pgx.Tx, sig rewardRiskSignals, userID, action, sourceID, inviterID string) (rewardRiskDecision, error) {
	decision := rewardRiskDecision{Action: "allow", Reasons: []string{}, Details: map[string]any{}}
	if sig.RTCHashes == nil {
		sig.RTCHashes = []string{}
	}
	var name string
	var enabled bool
	var reviewedThrough, now time.Time
	if err := tx.QueryRow(ctx, `select name,enabled,risk_reviewed_through,clock_timestamp() from users where id=$1 for update`, userID).Scan(&name, &enabled, &reviewedThrough, &now); err != nil {
		return decision, err
	}
	if !enabled {
		return decision, errRiskAccountRestricted
	}
	var reasons, details []byte
	err := tx.QueryRow(ctx, `select id::text,decision,reasons,details from risk_observations where user_id=$1 and action=$2 and source_id=$3`, userID, action, sourceID).Scan(&decision.ID, &decision.Action, &reasons, &details)
	if err == nil {
		if err = json.Unmarshal(reasons, &decision.Reasons); err != nil {
			return decision, err
		}
		err = json.Unmarshal(details, &decision.Details)
		return decision, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return decision, err
	}
	cfg, err := loadRewardRiskSettings(ctx, tx)
	if err != nil {
		return decision, err
	}
	decision.Details = map[string]any{"version": cfg.Version, "rtc_status": sig.RTCStatus}
	autoBan := false
	if cfg.Enabled {
		window := now.Add(-time.Duration(cfg.WindowHours) * time.Hour)
		burst := now.Add(-time.Duration(cfg.BurstMinutes) * time.Minute)
		if reviewedThrough.After(burst) {
			burst = reviewedThrough
		}
		rows, err := tx.Query(ctx, `select u.id::text,u.name,
   bool_or($3<>'' and o.http_ip_hash=$3),bool_or($4<>'' and o.browser_hash=$4),bool_or(o.rtc_hashes && $5::text[]),
   bool_or($3<>'' and o.http_ip_hash=$3 and coalesce(o.inviter_id=nullif($6,'')::bigint,false)),
   bool_or(o.action in ('register','checkin') and o.created_at>$7 and $4<>'' and o.browser_hash=$4),
   bool_or(o.action in ('register','checkin') and o.created_at>$7 and $3<>'' and o.http_ip_hash=$3 and coalesce(o.inviter_id=nullif($6,'')::bigint,false))
   from risk_observations o join users u on u.id=o.user_id
   where o.user_id<>$1 and o.created_at>$2 and (($3<>'' and o.http_ip_hash=$3) or ($4<>'' and o.browser_hash=$4) or o.rtc_hashes && $5::text[] or coalesce(o.inviter_id=nullif($6,'')::bigint,false))
   group by u.id,u.name order by max(o.created_at) desc,u.id limit 501`, userID, window, sig.HTTPIPHash, sig.BrowserHash, sig.RTCHashes, inviterID, burst)
		if err != nil {
			return decision, err
		}
		peers := []rewardRiskPeer{}
		for rows.Next() {
			var p rewardRiskPeer
			if err = rows.Scan(&p.ID, &p.Name, &p.HTTP, &p.Browser, &p.RTC, &p.IPInviter, &p.BurstBrowser, &p.BurstIPInviter); err != nil {
				rows.Close()
				return decision, err
			}
			peers = append(peers, p)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return decision, err
		}
		if len(peers) > 500 {
			decision.Reasons = append(decision.Reasons, "candidate_overflow")
		} else {
			decision.Reasons, decision.Details, autoBan = rewardRiskCluster(cfg, name, peers)
		}
		decision.Details["rtc_status"] = sig.RTCStatus
		var registrations, checkins, browserCheckins, invites int
		err = tx.QueryRow(ctx, `select
   count(distinct user_id) filter(where action='register' and $2<>'' and http_ip_hash=$2 and created_at>$4),
   count(distinct user_id) filter(where action='checkin' and $2<>'' and http_ip_hash=$2 and created_at>=$5),
   count(distinct user_id) filter(where action='checkin' and $3<>'' and browser_hash=$3 and created_at>=$5)
   from risk_observations where user_id<>$1 and created_at>=least($4,$5) and (($2<>'' and http_ip_hash=$2) or ($3<>'' and browser_hash=$3))`, userID, sig.HTTPIPHash, sig.BrowserHash, window, time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)).Scan(&registrations, &checkins, &browserCheckins)
		if err != nil {
			return decision, err
		}
		if action == "register" && sig.HTTPIPHash != "" && registrations >= cfg.RegistrationsPerIP {
			decision.Reasons = append(decision.Reasons, "registration_ip_limit")
		}
		if action == "checkin" {
			if sig.HTTPIPHash != "" && checkins >= cfg.CheckinsPerIP {
				decision.Reasons = append(decision.Reasons, "checkin_ip_limit")
			}
			if sig.BrowserHash != "" && browserCheckins >= cfg.CheckinsPerBrowser {
				decision.Reasons = append(decision.Reasons, "checkin_browser_limit")
			}
		}
		if action == "register" && inviterID != "" {
			err = tx.QueryRow(ctx, `select count(*) from invitations where inviter_id=$1 and invitee_id<>$2 and created_at >= date_trunc('day',$3::timestamptz at time zone 'UTC') at time zone 'UTC'`, inviterID, userID, now).Scan(&invites)
			if err != nil {
				return decision, err
			}
			if invites >= cfg.InvitationsPerInviter {
				decision.Reasons = append(decision.Reasons, "inviter_daily_limit")
			}
			var shared bool
			err = tx.QueryRow(ctx, `select exists(select 1 from risk_observations where user_id=$1 and browser_hash=$2 and $2<>'' and created_at>$3)`, inviterID, sig.BrowserHash, window).Scan(&shared)
			if err != nil {
				return decision, err
			}
			if shared {
				decision.Reasons = append(decision.Reasons, "shared_browser_invitation")
			}
		}
		decision.Details["registration_ip_accounts"] = registrations
		decision.Details["checkin_ip_accounts"] = checkins
		decision.Details["checkin_browser_accounts"] = browserCheckins
		decision.Details["inviter_daily_invitations"] = invites
		if len(decision.Reasons) > 0 {
			decision.Action = "review"
		}
	}
	decision.ID, err = randomID()
	if err != nil {
		return decision, err
	}
	if err = s.insertRewardObservationTx(ctx, tx, sig, userID, action, sourceID, inviterID, name, decision, now); err != nil {
		return decision, err
	}
	if cfg.Enabled && cfg.AutoBanEnabled && autoBan {
		banned, err := s.autoBanRewardUserTx(ctx, tx, userID, decision.ID, cfg.Version, decision.Details)
		if err != nil {
			return decision, err
		}
		if banned {
			decision.Action = "ban"
			decision.Reasons = append(decision.Reasons, "automatic_ban")
			raw, _ := json.Marshal(decision.Reasons)
			if _, err = tx.Exec(ctx, `update risk_observations set decision='ban',reasons=$2 where id=$1`, decision.ID, raw); err != nil {
				return decision, err
			}
		}
	}
	return decision, nil
}

func (s *Service) insertRewardObservationTx(ctx context.Context, tx pgx.Tx, sig rewardRiskSignals, userID, action, sourceID, inviterID, name string, d rewardRiskDecision, now time.Time) error {
	reasons, err := json.Marshal(d.Reasons)
	if err != nil {
		return err
	}
	details, err := json.Marshal(d.Details)
	if err != nil {
		return err
	}
	if sig.RTCHashes == nil {
		sig.RTCHashes = []string{}
	}
	if sig.RTCStatus == "" {
		sig.RTCStatus = "unknown"
	}
	_, err = tx.Exec(ctx, `insert into risk_observations(id,user_id,action,source_id,http_ip_hash,browser_hash,rtc_hashes,rtc_status,inviter_id,name_snapshot,decision,reasons,details,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,nullif($9,'')::bigint,$10,$11,$12,$13,$14)`, d.ID, userID, action, sourceID, sig.HTTPIPHash, sig.BrowserHash, sig.RTCHashes, sig.RTCStatus, inviterID, name, d.Action, reasons, details, now)
	return err
}

func (s *Service) recordRewardLoginTx(ctx context.Context, tx pgx.Tx, sig rewardRiskSignals, userID string) error {
	var name string
	var now time.Time
	if err := tx.QueryRow(ctx, `select name,clock_timestamp() from users where id=$1 and enabled for update`, userID).Scan(&name, &now); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errRiskAccountRestricted
		}
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	return s.insertRewardObservationTx(ctx, tx, sig, userID, "login", id, "", name, rewardRiskDecision{ID: id, Action: "allow", Reasons: []string{}, Details: map[string]any{}}, now)
}

func (s *Service) recordRewardLogin(w http.ResponseWriter, r *http.Request, userID string) error {
	sig, err := s.rewardSignals(w, r)
	if err != nil {
		return err
	}
	tx, err := s.beginRewardRiskTx(r.Context(), false)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = s.recordRewardLoginTx(r.Context(), tx, sig, userID); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}
