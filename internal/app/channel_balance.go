package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type upstreamUsageWindow struct {
	Window    string   `json:"window"`
	Used      *float64 `json:"used"`
	Limit     *float64 `json:"limit"`
	Remaining *float64 `json:"remaining"`
	Percent   *float64 `json:"percent"`
	ResetAt   string   `json:"reset_at,omitempty"`
	Unit      string   `json:"unit,omitempty"`
}

type upstreamBalance struct {
	Balance        *float64
	Used           *float64
	Total          *float64
	Currency       string
	UsageWindows   []upstreamUsageWindow
	UsageSupported bool
}

func balanceURL(base, path string) string {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return ""
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}

func number(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case json.Number:
		f, err := x.Float64()
		if err == nil {
			return &f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err == nil {
			return &f
		}
	}
	return nil
}

func firstNumber(m map[string]any, names ...string) *float64 {
	for _, name := range names {
		if v, ok := m[name]; ok {
			if n := number(v); n != nil {
				return n
			}
		}
	}
	return nil
}

func balanceFields(raw map[string]any) (upstreamBalance, bool) {
	// New API/One API returns quota counters under data. Quota is intentionally
	// kept as the currency because it is not guaranteed to be USD.
	if nested, ok := raw["data"].(map[string]any); ok {
		if b, found := balanceFields(nested); found {
			return b, true
		}
	}
	// DeepSeek reports available balance in data.balance_infos[].total_balance,
	// matching the NewAPI DeepSeek adaptor's /user/balance implementation.
	if infos, ok := raw["balance_infos"].([]any); ok {
		for _, item := range infos {
			info, ok := item.(map[string]any)
			if !ok {
				continue
			}
			b := upstreamBalance{Currency: "CNY"}
			if currency, ok := info["currency"].(string); ok && strings.TrimSpace(currency) != "" {
				b.Currency = strings.TrimSpace(currency)
			}
			b.Balance = firstNumber(info, "total_balance")
			if b.Balance != nil {
				return b, true
			}
		}
	}
	b := upstreamBalance{Currency: "USD"}
	b.Balance = firstNumber(raw, "total_available", "available", "balance", "remaining", "remain_quota")
	b.Used = firstNumber(raw, "total_used", "used", "total_usage", "usage", "used_quota")
	b.Total = firstNumber(raw, "total_granted", "total", "hard_limit_usd", "credit_limit", "quota")
	if firstNumber(raw, "remain_quota", "used_quota", "quota") != nil {
		b.Currency = "quota"
	}
	if currency, ok := raw["currency"].(string); ok && strings.TrimSpace(currency) != "" {
		b.Currency = currency
	}
	if b.Balance == nil && b.Used != nil && b.Total != nil {
		v := *b.Total - *b.Used
		b.Balance = &v
	}
	if b.Total == nil && b.Balance != nil && b.Used != nil {
		v := *b.Balance + *b.Used
		b.Total = &v
	}
	return b, b.Balance != nil || b.Used != nil || b.Total != nil
}

func parseOpenCodeGoUsageWindow(name string, raw json.RawMessage) (upstreamUsageWindow, bool) {
	var item struct {
		Status   string  `json:"status"`
		Percent  float64 `json:"percent"`
		ResetsAt string  `json:"resetsAt"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return upstreamUsageWindow{}, false
	}
	if !math.IsNaN(item.Percent) && !math.IsInf(item.Percent, 0) && (item.Percent < 0 || item.Percent > 100) {
		return upstreamUsageWindow{}, false
	}
	window := upstreamUsageWindow{Window: name, Percent: &item.Percent, ResetAt: strings.TrimSpace(item.ResetsAt), Unit: "%"}
	if item.Status != "" {
		window.Unit = "%"
	}
	if window.ResetAt != "" {
		if parsed, err := time.Parse(time.RFC3339, window.ResetAt); err == nil {
			window.ResetAt = parsed.UTC().Format(time.RFC3339)
		}
	}
	return window, true
}

func parseOpenCodeGoUsage(body []byte) ([]upstreamUsageWindow, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid OpenCode Go usage response")
	}
	if wrapped, ok := raw["usage"]; ok {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(wrapped, &nested); err != nil {
			return nil, fmt.Errorf("invalid OpenCode Go usage response")
		}
		raw = nested
	}
	windows := make([]upstreamUsageWindow, 0, 3)
	for _, item := range []struct{ key, label string }{{"rolling", "rolling"}, {"weekly", "weekly"}, {"monthly", "monthly"}} {
		value, ok := raw[item.key]
		if !ok {
			continue
		}
		window, valid := parseOpenCodeGoUsageWindow(item.label, value)
		if valid {
			windows = append(windows, window)
		}
	}
	if len(windows) == 0 {
		return nil, fmt.Errorf("OpenCode Go usage response has no usage windows")
	}
	return windows, nil
}

func (s *Service) queryOpenCodeGoUsage(ctx context.Context, baseURL, apiKey string) (upstreamBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, balanceURL(baseURL, "/usage"), nil)
	if err != nil {
		return upstreamBalance{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return upstreamBalance{}, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if readErr != nil {
		return upstreamBalance{}, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamBalance{}, fmt.Errorf("OpenCode Go usage returned status %d", resp.StatusCode)
	}
	windows, err := parseOpenCodeGoUsage(body)
	if err != nil {
		return upstreamBalance{}, err
	}
	return upstreamBalance{Currency: "usage", UsageWindows: windows, UsageSupported: true}, nil
}

func (s *Service) queryUpstreamBalance(ctx context.Context, baseURL, apiKey, provider string) (upstreamBalance, error) {
	if provider == "anthropic" || provider == "ollama" || provider == "commandcode" {
		return upstreamBalance{}, fmt.Errorf("balance query is not supported for %s", provider)
	}
	if provider == "opencode_go" {
		return s.queryOpenCodeGoUsage(ctx, baseURL, apiKey)
	}
	// New API/One API and OpenCode-compatible deployments commonly expose one
	// of these account endpoints in addition to the OpenAI billing endpoints.
	paths := []string{"/api/user/self", "/api/user/credits", "/api/usage", "/v1/credits", "/v1/dashboard/billing/credit_grants", "/dashboard/billing/credit_grants", "/v1/dashboard/billing/subscription", "/dashboard/billing/subscription"}
	if provider == "deepseek" {
		// DeepSeek's official balance endpoint, as used by the NewAPI DeepSeek adaptor.
		paths = append([]string{"/user/balance"}, paths...)
	}
	var lastStatus int
	for _, path := range paths {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, balanceURL(baseURL, path), nil)
		if err != nil {
			return upstreamBalance{}, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return upstreamBalance{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return upstreamBalance{}, readErr
		}
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return upstreamBalance{}, fmt.Errorf("upstream balance returned status %d", resp.StatusCode)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			return upstreamBalance{}, fmt.Errorf("invalid upstream balance response")
		}
		b, found := balanceFields(raw)
		if !found {
			return upstreamBalance{}, fmt.Errorf("upstream balance response has no balance fields")
		}
		return b, nil
	}
	return upstreamBalance{}, fmt.Errorf("upstream balance endpoint unavailable (status %d)", lastStatus)
}

func (s *Service) refreshChannelBalance(ctx context.Context, channelID string, keyID *string) (upstreamBalance, error) {
	var baseURL, provider, encrypted string
	var selectedKey *string
	query := `select c.base_url,c.provider,k.key_encrypted,k.id::text from channels c join channel_api_keys k on k.channel_id=c.id and k.enabled where c.id=$1 order by k.priority desc nulls last,k.created_at limit 1`
	args := []any{channelID}
	if keyID != nil && strings.TrimSpace(*keyID) != "" {
		query = `select c.base_url,c.provider,k.key_encrypted,k.id::text from channels c join channel_api_keys k on k.channel_id=c.id where c.id=$1 and k.id=$2`
		args = append(args, *keyID)
	}
	if err := s.db.QueryRow(ctx, query, args...).Scan(&baseURL, &provider, &encrypted, &selectedKey); err != nil {
		return upstreamBalance{}, fmt.Errorf("channel key not found")
	}
	apiKey, err := channelKeyValue(s.cfg.EncryptionKey, encrypted)
	if err != nil {
		return upstreamBalance{}, err
	}
	b, err := s.queryUpstreamBalance(ctx, baseURL, apiKey, provider)
	if err != nil {
		_, _ = s.db.Exec(ctx, `insert into channel_balances(channel_id,key_id,supported,error,fetched_at) values($1,$2,false,$3,now()) on conflict (channel_id,key_id) do update set supported=false,error=excluded.error,fetched_at=excluded.fetched_at`, channelID, selectedKey, err.Error())
		return b, err
	}
	usageJSON, _ := json.Marshal(b.UsageWindows)
	_, _ = s.db.Exec(ctx, `insert into channel_balances(channel_id,key_id,balance,used,total,currency,usage,supported,error,fetched_at) values($1,$2,$3,$4,$5,$6,$7,true,'',now()) on conflict (channel_id,key_id) do update set balance=excluded.balance,used=excluded.used,total=excluded.total,currency=excluded.currency,usage=excluded.usage,supported=true,error='',fetched_at=excluded.fetched_at`, channelID, selectedKey, b.Balance, b.Used, b.Total, b.Currency, usageJSON)
	return b, nil
}

func (s *Service) channelBalanceHandler(w http.ResponseWriter, r *http.Request) {
	keyID := strings.TrimSpace(r.URL.Query().Get("key_id"))
	var keyPtr *string
	if keyID != "" {
		keyPtr = &keyID
	}
	b, err := s.refreshChannelBalance(r.Context(), r.PathValue("id"), keyPtr)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"balance": b.Balance, "used": b.Used, "total": b.Total, "currency": b.Currency, "usage_windows": b.UsageWindows, "supported": true, "fetched_at": time.Now().UTC()})
}

func (s *Service) startChannelBalanceScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refreshChannelBalances(ctx)
			}
		}
	}()
}

func (s *Service) refreshChannelBalances(ctx context.Context) {
	rows, err := s.db.Query(ctx, `select id::text from channels where enabled and not auto_disabled`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			_, _ = s.refreshChannelBalance(ctx, id, nil)
		}
	}
}
