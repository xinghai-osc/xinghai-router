package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type rewardRiskSettings struct {
	Enabled               bool     `json:"enabled"`
	AutoBanEnabled        bool     `json:"auto_ban_enabled"`
	WebRTCEnabled         bool     `json:"webrtc_enabled"`
	STUNURLs              []string `json:"stun_urls"`
	WindowHours           int      `json:"window_hours"`
	UsernameSimilarity    float64  `json:"username_similarity"`
	SimilarAccounts       int      `json:"similar_accounts"`
	BurstMinutes          int      `json:"burst_minutes"`
	BurstAccounts         int      `json:"burst_accounts"`
	RegistrationsPerIP    int      `json:"registrations_per_ip"`
	CheckinsPerIP         int      `json:"checkins_per_ip"`
	CheckinsPerBrowser    int      `json:"checkins_per_browser"`
	InvitationsPerInviter int      `json:"invitations_per_inviter"`
	Version               int      `json:"version"`
}

type riskQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func defaultRewardRiskSettings() rewardRiskSettings {
	return rewardRiskSettings{Enabled: true, AutoBanEnabled: true, WebRTCEnabled: true, STUNURLs: []string{"stun:stun.cloudflare.com:3478"}, WindowHours: 24, UsernameSimilarity: 0.85, SimilarAccounts: 3, BurstMinutes: 10, BurstAccounts: 5, RegistrationsPerIP: 3, CheckinsPerIP: 3, CheckinsPerBrowser: 1, InvitationsPerInviter: 10, Version: 1}
}

func loadRewardRiskSettings(ctx context.Context, q riskQueryer) (rewardRiskSettings, error) {
	cfg := defaultRewardRiskSettings()
	var raw []byte
	if err := q.QueryRow(ctx, `select reward_risk_settings from site_settings where id=true`).Scan(&raw); err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("decode reward risk settings: %w", err)
	}
	return cfg, validateRewardRiskSettings(cfg)
}

func validateRewardRiskSettings(c rewardRiskSettings) error {
	if c.WindowHours < 1 || c.WindowHours > 720 || c.BurstMinutes < 1 || c.BurstMinutes > 1440 || c.BurstMinutes > c.WindowHours*60 {
		return errors.New("risk windows must be positive; burst window must fit inside observation window")
	}
	if math.IsNaN(c.UsernameSimilarity) || math.IsInf(c.UsernameSimilarity, 0) || c.UsernameSimilarity < 0.7 || c.UsernameSimilarity > 1 {
		return errors.New("username_similarity must be between 0.70 and 1.00")
	}
	if c.SimilarAccounts < 3 || c.SimilarAccounts > 500 || c.BurstAccounts < c.SimilarAccounts || c.BurstAccounts > 500 {
		return errors.New("similar_accounts must be 3–500; burst_accounts must be at least similar_accounts and at most 500")
	}
	for _, n := range []int{c.RegistrationsPerIP, c.CheckinsPerIP, c.CheckinsPerBrowser, c.InvitationsPerInviter} {
		if n < 1 || n > 10000 {
			return errors.New("reward account limits must be between 1 and 10000")
		}
	}
	if len(c.STUNURLs) > 2 {
		return errors.New("at most two STUN URLs are allowed")
	}
	for _, v := range c.STUNURLs {
		if !strings.HasPrefix(v, "stun:") {
			return errors.New("STUN URLs must use stun:host:port without credentials")
		}
		host, port, err := net.SplitHostPort(strings.TrimPrefix(v, "stun:"))
		n, e := strconv.Atoi(port)
		if err != nil || e != nil || n < 1 || n > 65535 || host == "" || strings.ContainsAny(host, "/?#@ \t\r\n") {
			return errors.New("invalid STUN URL")
		}
	}
	return nil
}

func (s *Service) getRewardRiskSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := loadRewardRiskSettings(r.Context(), s.db)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load reward risk settings")
		return
	}
	writeJSON(w, 200, cfg)
}

func (s *Service) updateRewardRiskSettings(w http.ResponseWriter, r *http.Request) {
	var in rewardRiskSettings
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid reward risk settings")
		return
	}
	if err := validateRewardRiskSettings(in); err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	tx, err := s.beginRewardRiskTx(r.Context(), false)
	if err != nil {
		writeError(w, 500, "internal_error", "could not save reward risk settings")
		return
	}
	defer tx.Rollback(r.Context())
	actor, err := lockUserAccess(r.Context(), tx, accountFromContext(r).userID)
	if err != nil || !actor.enabled || !accountHasPermission(actor.accountContext, "system.manage") {
		writeError(w, 403, "forbidden", "missing permission: system.manage")
		return
	}
	if err = validateUserMutationSession(r.Context(), tx, actor.userID); err != nil {
		writeUserMutationError(w, err)
		return
	}
	var old rewardRiskSettings
	old, err = loadRewardRiskSettings(r.Context(), tx)
	if err != nil {
		writeError(w, 500, "internal_error", "could not load reward risk settings")
		return
	}
	in.Version = old.Version + 1
	if in.STUNURLs == nil {
		in.STUNURLs = []string{}
	}
	raw, err := json.Marshal(in)
	if err == nil {
		_, err = tx.Exec(r.Context(), `update site_settings set reward_risk_settings=$1,updated_at=now() where id=true`, raw)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeError(w, 500, "internal_error", "could not save reward risk settings")
		return
	}
	s.audit(r, "reward_risk.settings_updated", "site_settings", "site", map[string]any{"version": in.Version, "enabled": in.Enabled, "auto_ban_enabled": in.AutoBanEnabled})
	writeJSON(w, 200, in)
}

func (s *Service) beginRewardRiskTx(ctx context.Context, creating bool) (pgx.Tx, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(458111)`); err == nil && creating {
		_, err = tx.Exec(ctx, `select pg_advisory_xact_lock(458110)`)
	}
	if err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}
