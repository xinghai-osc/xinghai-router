package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const minimaxBalanceURL = "https://www.minimax.cn/backend/account/token_plan_credit"

type minimaxBalanceConfiguration struct {
	Configured bool   `json:"configured"`
	GroupID    string `json:"group_id"`
}

func validMiniMaxBalanceGroupID(groupID string) bool {
	if len(groupID) == 0 || len(groupID) > 64 {
		return false
	}
	for _, digit := range groupID {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func validMiniMaxBalanceCookie(cookie string) bool {
	if len(cookie) == 0 || len(cookie) > 16384 || strings.ContainsAny(cookie, "\r\n") {
		return false
	}
	foundToken := false
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok || (&http.Cookie{Name: name, Value: value}).Valid() != nil {
			return false
		}
		if name == "_token" {
			if foundToken || value == "" {
				return false
			}
			foundToken = true
		}
	}
	return foundToken
}

func (s *Service) getMiniMaxBalanceCredentials(w http.ResponseWriter, r *http.Request) {
	var provider string
	var config minimaxBalanceConfiguration
	err := s.db.QueryRow(r.Context(), `select c.provider,m.channel_id is not null,coalesce(m.group_id,'') from channels c left join minimax_balance_credentials m on m.channel_id=c.id where c.id=$1`, r.PathValue("id")).Scan(&provider, &config.Configured, &config.GroupID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load MiniMax balance configuration")
		return
	}
	if provider != "minimax" {
		writeError(w, http.StatusBadRequest, "invalid_request", "balance credentials require a MiniMax channel")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Service) updateMiniMaxBalanceCredentials(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Cookie  string `json:"cookie"`
		GroupID string `json:"group_id"`
	}
	if decode(r, &input) != nil || !validMiniMaxBalanceGroupID(input.GroupID) || (input.Cookie != "" && !validMiniMaxBalanceCookie(input.Cookie)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "group_id must contain 1-64 digits; cookie must contain _token and be at most 16384 bytes without line breaks")
		return
	}
	encrypted := ""
	var err error
	if input.Cookie != "" {
		encrypted, err = crypt(s.cfg.EncryptionKey, input.Cookie, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not encrypt MiniMax balance credentials")
			return
		}
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save MiniMax balance configuration")
		return
	}
	defer tx.Rollback(r.Context())
	if !miniMaxBalanceChannel(w, r, tx) {
		return
	}
	if encrypted == "" {
		result, updateErr := tx.Exec(r.Context(), `update minimax_balance_credentials set group_id=$2,updated_at=now() where channel_id=$1`, r.PathValue("id"), input.GroupID)
		err = updateErr
		if err == nil && result.RowsAffected() == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "cookie is required for the first balance configuration")
			return
		}
	} else {
		_, err = tx.Exec(r.Context(), `insert into minimax_balance_credentials(channel_id,cookie_encrypted,group_id) values($1,$2,$3) on conflict(channel_id) do update set cookie_encrypted=excluded.cookie_encrypted,group_id=excluded.group_id,updated_at=now()`, r.PathValue("id"), encrypted, input.GroupID)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `delete from channel_balances where channel_id=$1`, r.PathValue("id"))
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save MiniMax balance configuration")
		return
	}
	s.audit(r, "channel.minimax_balance_credentials_updated", "channel", r.PathValue("id"), nil)
	writeJSON(w, http.StatusOK, minimaxBalanceConfiguration{Configured: true, GroupID: input.GroupID})
}

func (s *Service) deleteMiniMaxBalanceCredentials(w http.ResponseWriter, r *http.Request) {
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete MiniMax balance configuration")
		return
	}
	defer tx.Rollback(r.Context())
	if !miniMaxBalanceChannel(w, r, tx) {
		return
	}
	_, err = tx.Exec(r.Context(), `delete from minimax_balance_credentials where channel_id=$1`, r.PathValue("id"))
	if err == nil {
		_, err = tx.Exec(r.Context(), `delete from channel_balances where channel_id=$1`, r.PathValue("id"))
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete MiniMax balance configuration")
		return
	}
	s.audit(r, "channel.minimax_balance_credentials_deleted", "channel", r.PathValue("id"), nil)
	writeJSON(w, http.StatusOK, minimaxBalanceConfiguration{})
}

func miniMaxBalanceChannel(w http.ResponseWriter, r *http.Request, tx pgx.Tx) bool {
	var provider string
	err := tx.QueryRow(r.Context(), `select provider from channels where id=$1 for update`, r.PathValue("id")).Scan(&provider)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "channel not found")
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load channel")
		return false
	}
	if provider != "minimax" {
		writeError(w, http.StatusBadRequest, "invalid_request", "balance credentials require a MiniMax channel")
		return false
	}
	return true
}

func parseMiniMaxBalance(body []byte) (upstreamBalance, error) {
	var response struct {
		Total     *float64 `json:"total_credits"`
		Used      *float64 `json:"used_credits"`
		Remaining *float64 `json:"remaining_credits"`
		BaseResp  *struct {
			StatusCode *int `json:"status_code"`
		} `json:"base_resp"`
	}
	if json.Unmarshal(body, &response) != nil || response.BaseResp == nil || response.BaseResp.StatusCode == nil {
		return upstreamBalance{}, fmt.Errorf("invalid MiniMax balance response")
	}
	if code := *response.BaseResp.StatusCode; code != 0 {
		return upstreamBalance{}, fmt.Errorf("MiniMax balance query failed (code %d); check the login cookie and group ID", code)
	}
	for _, value := range []*float64{response.Total, response.Used, response.Remaining} {
		if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
			return upstreamBalance{}, fmt.Errorf("MiniMax balance response has invalid credit values")
		}
	}
	return upstreamBalance{Balance: response.Remaining, Used: response.Used, Total: response.Total, Currency: "credits"}, nil
}

func (s *Service) queryMiniMaxBalance(ctx context.Context, cookie, groupID string) (upstreamBalance, error) {
	if !validMiniMaxBalanceCookie(cookie) || !validMiniMaxBalanceGroupID(groupID) {
		return upstreamBalance{}, fmt.Errorf("invalid MiniMax balance credentials")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, minimaxBalanceURL, nil)
	if err != nil {
		return upstreamBalance{}, fmt.Errorf("could not create MiniMax balance request")
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-Group-Id", groupID)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://platform.minimax.cn")
	req.Header.Set("Referer", "https://platform.minimax.cn/")
	client := clientWithTimeout(s.httpClient, 15*time.Second)
	client.Jar = nil
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return upstreamBalance{}, fmt.Errorf("MiniMax balance request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamBalance{}, fmt.Errorf("MiniMax balance returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		return upstreamBalance{}, fmt.Errorf("could not read MiniMax balance response")
	}
	return parseMiniMaxBalance(body)
}

func (s *Service) queryMiniMaxChannelBalance(ctx context.Context, channelID string) (upstreamBalance, error) {
	var encrypted, groupID string
	err := s.db.QueryRow(ctx, `select cookie_encrypted,group_id from minimax_balance_credentials where channel_id=$1`, channelID).Scan(&encrypted, &groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return upstreamBalance{}, fmt.Errorf("configure a MiniMax login cookie and group ID before querying credits")
	}
	if err != nil {
		return upstreamBalance{}, fmt.Errorf("could not load MiniMax balance credentials")
	}
	cookie, err := crypt(s.cfg.EncryptionKey, encrypted, true)
	if err != nil {
		return upstreamBalance{}, fmt.Errorf("could not decrypt MiniMax balance credentials")
	}
	return s.queryMiniMaxBalance(ctx, cookie, groupID)
}
