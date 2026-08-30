package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Service) fetchChannelModels(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "base_url and api_key are required")
		return
	}
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)
	if !validChannelAPIKey(in.APIKey) {
		writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
		return
	}
	if !validChannelBaseURL(in.BaseURL) {
		writeError(w, 400, "invalid_request", "base_url must be 1-2048 characters and use HTTP or HTTPS")
		return
	}
	baseURL, err := url.Parse(strings.TrimRight(in.BaseURL, "/"))
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid base_url")
		return
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/v1/models"
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, baseURL.String(), nil)
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid base_url")
		return
	}
	request.Header.Set("Authorization", "Bearer "+in.APIKey)
	response, err := s.httpClient.Do(request)
	if err != nil {
		writeError(w, 502, "upstream_error", "could not fetch models")
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		writeError(w, 502, "upstream_error", "could not read upstream models response")
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		w.Header().Set("Content-Type", contentType(response.Header.Get("Content-Type")))
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(body)
		return
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil {
		writeError(w, 502, "upstream_error", "invalid models response")
		return
	}
	models := make([]string, 0, len(result.Data))
	seen := map[string]bool{}
	for _, item := range result.Data {
		model := strings.TrimSpace(item.ID)
		if model != "" && !seen[model] {
			seen[model] = true
			models = append(models, model)
		}
	}
	writeJSON(w, 200, map[string]any{"models": models})
}

func (s *Service) listPricing(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,model,input_per_million,cached_input_per_million,output_per_million,multiplier,enabled,updated_at from pricing_rules order by model`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, model string
		var input, cached, output, multiplier any
		var enabled bool
		var updated any
		if rows.Scan(&id, &model, &input, &cached, &output, &multiplier, &enabled, &updated) != nil {
			continue
		}
		data = append(data, map[string]any{"id": id, "model": model, "input_per_million": input, "cached_input_per_million": cached, "output_per_million": output, "multiplier": multiplier, "enabled": enabled, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) upsertPricing(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Model      string  `json:"model"`
		Input      float64 `json:"input_per_million"`
		Cached     float64 `json:"cached_input_per_million"`
		Output     float64 `json:"output_per_million"`
		Multiplier float64 `json:"multiplier"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid pricing rule")
		return
	}
	in.Model = strings.TrimSpace(in.Model)
	if !validPricingModel(in.Model) || !validPricingRate(in.Input) || !validPricingRate(in.Cached) || !validPricingRate(in.Output) {
		writeError(w, 400, "invalid_request", "invalid pricing rule")
		return
	}
	if in.Multiplier == 0 {
		in.Multiplier = 1
	}
	if !validPricingMultiplier(in.Multiplier) {
		writeError(w, 400, "invalid_request", "multiplier must be between 0 exclusive and 1000")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier) values($1,$2,$3,$4,$5,$6) on conflict(model) do update set input_per_million=excluded.input_per_million,cached_input_per_million=excluded.cached_input_per_million,output_per_million=excluded.output_per_million,multiplier=excluded.multiplier,updated_at=now()`, id, in.Model, in.Input, in.Cached, in.Output, in.Multiplier)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save pricing rule")
		return
	}
	s.pricingCache.invalidate(in.Model)
	s.audit(r, "pricing.updated", "pricing", in.Model, map[string]any{"input": in.Input, "cached": in.Cached, "output": in.Output})
	writeJSON(w, 200, map[string]any{"model": in.Model})
}

type newAPIPricing struct {
	ModelName       string   `json:"model_name"`
	QuotaType       int      `json:"quota_type"`
	ModelRatio      float64  `json:"model_ratio"`
	CompletionRatio float64  `json:"completion_ratio"`
	CacheRatio      *float64 `json:"cache_ratio"`
}

func newAPIPricePerMillion(modelRatio, pricePerQuotaUnit, quotaPerUnit float64) float64 {
	return modelRatio * 1000000 * pricePerQuotaUnit / quotaPerUnit
}

func (s *Service) syncNewAPIPricing(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL           string  `json:"base_url"`
		APIKey            string  `json:"api_key"`
		PricePerQuotaUnit float64 `json:"price_per_quota_unit"`
	}
	if decode(r, &in) != nil || !validPricePerQuotaUnit(in.PricePerQuotaUnit) {
		writeError(w, 400, "invalid_request", "invalid NewAPI pricing source")
		return
	}
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.APIKey = strings.TrimSpace(in.APIKey)
	if !validChannelBaseURL(in.BaseURL) {
		writeError(w, 400, "invalid_request", "base_url must be 1-2048 characters and use HTTP or HTTPS")
		return
	}
	if in.APIKey != "" && !validChannelAPIKey(in.APIKey) {
		writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
		return
	}
	baseURL, err := url.Parse(strings.TrimRight(in.BaseURL, "/"))
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid base_url")
		return
	}
	fetch := func(path string, out any) error {
		target := *baseURL
		target.Path = strings.TrimRight(target.Path, "/") + path
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
		if err != nil {
			return err
		}
		if in.APIKey != "" {
			request.Header.Set("Authorization", "Bearer "+in.APIKey)
		}
		response, err := s.httpClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return io.ErrUnexpectedEOF
		}
		return json.Unmarshal(body, out)
	}
	var status struct {
		Success bool `json:"success"`
		Data    struct {
			QuotaPerUnit float64 `json:"quota_per_unit"`
		} `json:"data"`
	}
	var pricing struct {
		Success bool            `json:"success"`
		Data    []newAPIPricing `json:"data"`
	}
	if err := fetch("/api/status", &status); err != nil || !status.Success || !validPositiveFinite(status.Data.QuotaPerUnit) {
		writeError(w, 502, "upstream_error", "could not read NewAPI quota configuration")
		return
	}
	if err := fetch("/api/pricing", &pricing); err != nil || !pricing.Success {
		writeError(w, 502, "upstream_error", "could not read NewAPI pricing")
		return
	}
	if len(pricing.Data) > maxNewAPIPricingModels {
		writeError(w, 400, "invalid_request", "too many models to sync")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not save pricing rules")
		return
	}
	defer tx.Rollback(r.Context())
	synced := 0
	for _, item := range pricing.Data {
		model := strings.TrimSpace(item.ModelName)
		if !validPricingModel(model) || item.QuotaType != 0 || !validNonNegativeFinite(item.ModelRatio) || !validNonNegativeFinite(item.CompletionRatio) {
			continue
		}
		if item.CacheRatio != nil && !validNonNegativeFinite(*item.CacheRatio) {
			continue
		}
		input := newAPIPricePerMillion(item.ModelRatio, in.PricePerQuotaUnit, status.Data.QuotaPerUnit)
		output := input * item.CompletionRatio
		cached := 0.0
		if item.CacheRatio != nil {
			cached = input * *item.CacheRatio
		}
		if !validPricingRate(input) || !validPricingRate(cached) || !validPricingRate(output) {
			continue
		}
		id, _ := randomID()
		if _, err = tx.Exec(r.Context(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier) values($1,$2,$3,$4,$5,1) on conflict(model) do update set input_per_million=excluded.input_per_million,cached_input_per_million=excluded.cached_input_per_million,output_per_million=excluded.output_per_million,updated_at=now()`, id, model, input, cached, output); err != nil {
			writeError(w, 500, "internal_error", "could not save pricing rules")
			return
		}
		synced++
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not save pricing rules")
		return
	}
	s.pricingCache.clear()
	s.audit(r, "pricing.newapi_synced", "pricing", "newapi", map[string]any{"count": synced, "quota_per_unit": status.Data.QuotaPerUnit})
	writeJSON(w, 200, map[string]any{"synced": synced, "skipped": len(pricing.Data) - synced})
}

// --- Tiered pricing CRUD ---

func (s *Service) listPricingTiers(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if !validPricingModel(model) {
		writeError(w, 400, "invalid_request", "invalid model")
		return
	}
	rows, err := s.db.Query(r.Context(), `select id,model,from_tokens,input_per_million,cached_input_per_million,output_per_million,created_at from pricing_tiers where model=$1 order by from_tokens`, model)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, m string
		var fromTokens int64
		var input, cached, output any
		var created any
		if rows.Scan(&id, &m, &fromTokens, &input, &cached, &output, &created) != nil {
			continue
		}
		data = append(data, map[string]any{"id": id, "model": m, "from_tokens": fromTokens, "input_per_million": input, "cached_input_per_million": cached, "output_per_million": output, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) savePricingTier(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID     string  `json:"id"`
		Model  string  `json:"model"`
		From   int64   `json:"from_tokens"`
		Input  float64 `json:"input_per_million"`
		Cached float64 `json:"cached_input_per_million"`
		Output float64 `json:"output_per_million"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid pricing tier")
		return
	}
	in.Model = strings.TrimSpace(in.Model)
	if !validPricingModel(in.Model) || in.From < 0 || !validPricingRate(in.Input) || !validPricingRate(in.Cached) || !validPricingRate(in.Output) {
		writeError(w, 400, "invalid_request", "invalid pricing tier")
		return
	}
	// Count existing tiers to prevent unbounded growth.
	var count int
	s.db.QueryRow(r.Context(), `select count(*) from pricing_tiers where model=$1`, in.Model).Scan(&count)
	if in.ID == "" && count >= maxPricingTiers {
		writeError(w, 400, "invalid_request", "too many tiers")
		return
	}
	if in.ID == "" {
		id, _ := randomID()
		_, err := s.db.Exec(r.Context(), `insert into pricing_tiers(id,model,from_tokens,input_per_million,cached_input_per_million,output_per_million) values($1,$2,$3,$4,$5,$6)`, id, in.Model, in.From, in.Input, in.Cached, in.Output)
		if err != nil {
			writeError(w, 400, "invalid_request", "could not save pricing tier")
			return
		}
		in.ID = id
	} else {
		_, err := s.db.Exec(r.Context(), `update pricing_tiers set from_tokens=$1,input_per_million=$2,cached_input_per_million=$3,output_per_million=$4 where id=$5 and model=$6`, in.From, in.Input, in.Cached, in.Output, in.ID, in.Model)
		if err != nil {
			writeError(w, 400, "invalid_request", "could not save pricing tier")
			return
		}
	}
	s.pricingCache.invalidate(in.Model)
	s.audit(r, "pricing.tier_saved", "pricing_tier", in.Model, map[string]any{"from_tokens": in.From})
	writeJSON(w, 200, map[string]any{"id": in.ID})
}

func (s *Service) deletePricingTier(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	model := r.URL.Query().Get("model")
	if id == "" || !validPricingModel(model) {
		writeError(w, 400, "invalid_request", "invalid request")
		return
	}
	_, err := s.db.Exec(r.Context(), `delete from pricing_tiers where id=$1 and model=$2`, id, model)
	if err != nil {
		writeError(w, 500, "internal_error", "delete failed")
		return
	}
	s.pricingCache.invalidate(model)
	s.audit(r, "pricing.tier_deleted", "pricing_tier", model, map[string]any{"id": id})
	writeJSON(w, 200, map[string]any{"ok": true})
}

// --- Time-based pricing CRUD ---

func (s *Service) listPricingTimeRules(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if !validPricingModel(model) {
		writeError(w, 400, "invalid_request", "invalid model")
		return
	}
	rows, err := s.db.Query(r.Context(), `select id,model,name,start_minute,end_minute,weekdays,input_per_million,cached_input_per_million,output_per_million,enabled,created_at from pricing_time_rules where model=$1 order by created_at`, model)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, m, name, weekdays string
		var startMinute, endMinute int
		var input, cached, output any
		var enabled bool
		var created any
		if rows.Scan(&id, &m, &name, &startMinute, &endMinute, &weekdays, &input, &cached, &output, &enabled, &created) != nil {
			continue
		}
		data = append(data, map[string]any{"id": id, "model": m, "name": name, "start_minute": startMinute, "end_minute": endMinute, "weekdays": weekdays, "input_per_million": input, "cached_input_per_million": cached, "output_per_million": output, "enabled": enabled, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) savePricingTimeRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID       string  `json:"id"`
		Model    string  `json:"model"`
		Name     string  `json:"name"`
		Start    int     `json:"start_minute"`
		End      int     `json:"end_minute"`
		Weekdays string  `json:"weekdays"`
		Input    float64 `json:"input_per_million"`
		Cached   float64 `json:"cached_input_per_million"`
		Output   float64 `json:"output_per_million"`
		Enabled  *bool   `json:"enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid time rule")
		return
	}
	in.Model = strings.TrimSpace(in.Model)
	in.Name = strings.TrimSpace(in.Name)
	in.Weekdays = strings.TrimSpace(in.Weekdays)
	if in.Weekdays == "" {
		in.Weekdays = "1111111"
	}
	if !validPricingModel(in.Model) || len(in.Name) > maxTimeRuleNameLength || !validTimeWindow(in.Start, in.End) || !validWeekdays(in.Weekdays) {
		writeError(w, 400, "invalid_request", "invalid time rule")
		return
	}
	if !validPricingRate(in.Input) || !validPricingRate(in.Cached) || !validPricingRate(in.Output) {
		writeError(w, 400, "invalid_request", "invalid time rule prices")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	var count int
	s.db.QueryRow(r.Context(), `select count(*) from pricing_time_rules where model=$1`, in.Model).Scan(&count)
	if in.ID == "" && count >= maxPricingTimeRules {
		writeError(w, 400, "invalid_request", "too many time rules")
		return
	}
	if in.ID == "" {
		id, _ := randomID()
		_, err := s.db.Exec(r.Context(), `insert into pricing_time_rules(id,model,name,start_minute,end_minute,weekdays,input_per_million,cached_input_per_million,output_per_million,enabled) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, in.Model, in.Name, in.Start, in.End, in.Weekdays, in.Input, in.Cached, in.Output, enabled)
		if err != nil {
			writeError(w, 400, "invalid_request", "could not save time rule")
			return
		}
		in.ID = id
	} else {
		_, err := s.db.Exec(r.Context(), `update pricing_time_rules set name=$1,start_minute=$2,end_minute=$3,weekdays=$4,input_per_million=$5,cached_input_per_million=$6,output_per_million=$7,enabled=$8 where id=$9 and model=$10`, in.Name, in.Start, in.End, in.Weekdays, in.Input, in.Cached, in.Output, enabled, in.ID, in.Model)
		if err != nil {
			writeError(w, 400, "invalid_request", "could not save time rule")
			return
		}
	}
	s.pricingCache.invalidate(in.Model)
	s.audit(r, "pricing.time_rule_saved", "pricing_time_rule", in.Model, map[string]any{"name": in.Name})
	writeJSON(w, 200, map[string]any{"id": in.ID})
}

func (s *Service) deletePricingTimeRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	model := r.URL.Query().Get("model")
	if id == "" || !validPricingModel(model) {
		writeError(w, 400, "invalid_request", "invalid request")
		return
	}
	_, err := s.db.Exec(r.Context(), `delete from pricing_time_rules where id=$1 and model=$2`, id, model)
	if err != nil {
		writeError(w, 500, "internal_error", "delete failed")
		return
	}
	s.pricingCache.invalidate(model)
	s.audit(r, "pricing.time_rule_deleted", "pricing_time_rule", model, map[string]any{"id": id})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Service) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,action,actor,entity_type,entity_id,details,client_ip,forwarded_for,user_agent,browser,browser_version,operating_system,operating_system_version,device_type,is_bot,request_method,request_path,request_id,created_at from audit_logs order by created_at desc limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, action, actor, typ, entity, clientIP, forwardedFor, userAgent, browser, browserVersion, operatingSystem, operatingSystemVersion, deviceType, method, path, requestID string
		var isBot bool
		var details []byte
		var created any
		if rows.Scan(&id, &action, &actor, &typ, &entity, &details, &clientIP, &forwardedFor, &userAgent, &browser, &browserVersion, &operatingSystem, &operatingSystemVersion, &deviceType, &isBot, &method, &path, &requestID, &created) == nil {
			data = append(data, map[string]any{"id": id, "action": action, "actor": actor, "entity_type": typ, "entity_id": entity, "details": json.RawMessage(details), "client_ip": clientIP, "forwarded_for": forwardedFor, "user_agent": userAgent, "browser": browser, "browser_version": browserVersion, "operating_system": operatingSystem, "operating_system_version": operatingSystemVersion, "device_type": deviceType, "is_bot": isBot, "request_method": method, "request_path": path, "request_id": requestID, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) audit(r *http.Request, action, entityType, entityID string, details map[string]any) {
	s.auditActor(r, accountFromContext(r).userID, action, entityType, entityID, details)
}

func (s *Service) auditActor(r *http.Request, actor, action, entityType, entityID string, details map[string]any) {
	raw, _ := json.Marshal(details)
	id, _ := randomID()
	meta := requestMetadata(r)
	_, _ = s.db.Exec(r.Context(), `insert into audit_logs(id,action,actor,entity_type,entity_id,details,client_ip,forwarded_for,user_agent,browser,browser_version,operating_system,operating_system_version,device_type,is_bot,request_method,request_path,request_id) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, id, action, actor, entityType, entityID, raw, meta.clientIP, meta.forwardedFor, meta.userAgent, meta.browser, meta.browserVersion, meta.operatingSystem, meta.operatingSystemVersion, meta.deviceType, meta.isBot, r.Method, r.URL.Path, requestID(r.Context()))
}

func (s *Service) createUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email       string   `json:"email"`
		Name        string   `json:"name"`
		Password    string   `json:"password"`
		Role        string   `json:"role"`
		Enabled     *bool    `json:"enabled"`
		Permissions []string `json:"permissions"`
		Groups      []string `json:"groups"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid user")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	in.Role = strings.TrimSpace(in.Role)
	if in.Role == "" {
		in.Role = "user"
	}
	if !validAccountInput(in.Email, in.Name, in.Password) || (in.Role != "user" && in.Role != "operator" && in.Role != "admin") {
		writeError(w, 400, "invalid_request", "a valid email, name, password, and role are required")
		return
	}
	seen := map[string]bool{}
	for _, permission := range in.Permissions {
		if !availablePermissions[permission] || seen[permission] {
			writeError(w, 400, "invalid_request", "invalid permissions")
			return
		}
		seen[permission] = true
	}
	passwordHash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, 500, "internal_error", "could not secure password")
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, 500, "internal_error", "could not create user")
		return
	}
	defer tx.Rollback(ctx)
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	var id string
	if err = tx.QueryRow(ctx, `insert into users(email,name,role,password_hash,enabled) values($1,$2,$3,$4,$5) returning id`, in.Email, in.Name, in.Role, passwordHash, enabled).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, 409, "conflict", "email or name already exists")
		} else {
			writeError(w, 500, "internal_error", "could not create user")
		}
		return
	}
	if _, err = tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict do nothing`, id); err != nil {
		writeError(w, 500, "internal_error", "could not create user")
		return
	}
	for _, permission := range in.Permissions {
		if _, err = tx.Exec(ctx, `insert into user_permissions(user_id,permission) values($1,$2)`, id, permission); err != nil {
			writeError(w, 500, "internal_error", "could not create user")
			return
		}
	}
	for _, group := range in.Groups {
		var groupID string
		if err = tx.QueryRow(ctx, `select id from groups where id::text=$1 or name=$1`, strings.TrimSpace(group)).Scan(&groupID); err != nil {
			writeError(w, 400, "invalid_request", "unknown group")
			return
		}
		if _, err = tx.Exec(ctx, `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`, id, groupID); err != nil {
			writeError(w, 500, "internal_error", "could not create user")
			return
		}
	}
	if err = tx.Commit(ctx); err != nil {
		writeError(w, 500, "internal_error", "could not create user")
		return
	}
	s.audit(r, "user.created", "user", id, map[string]any{"email": in.Email, "name": in.Name, "role": in.Role})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Service) listUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	where := ` where u.email not like 'deleted-abuse-%@invalid.local'`
	args := []any{}
	if term != "" {
		where += ` and (u.email ilike $1 or u.name ilike $1)`
		args = append(args, "%"+term+"%")
	}
	countRow := `select count(*) from users u` + where
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	var total int
	if err := s.db.QueryRow(r.Context(), countRow, countArgs...).Scan(&total); err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	args = append(args, pageSize, offset)
	query := `select u.id,u.email,u.name,u.role,u.enabled,u.leaderboard_opt_in,u.leaderboard_mask_name,u.data_usage_enabled,u.max_concurrency,u.created_at,coalesce(w.balance,0),coalesce(w.reserved,0),coalesce(array_agg(p.permission) filter (where p.permission is not null), '{}'),coalesce((select array_agg(ug.group_id order by ug.group_id) from user_groups ug where ug.user_id=u.id), '{}'),i.inviter_id,iu.email,iu.name from users u left join user_permissions p on p.user_id=u.id left join user_wallets w on w.user_id=u.id left join invitations i on i.invitee_id=u.id left join users iu on iu.id=i.inviter_id` + where + ` group by u.id,w.balance,w.reserved,i.inviter_id,iu.email,iu.name order by u.created_at desc limit $` + strconv.Itoa(len(args)-1) + ` offset $` + strconv.Itoa(len(args))
	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, email, name, role string
		var enabled bool
		var leaderboardOptIn, leaderboardMaskName, dataUsageEnabled bool
		var maxConcurrency, created any
		var balance, reserved any
		var permissions []string
		var groups []string
		var inviterID, inviterEmail, inviterName *string
		if rows.Scan(&id, &email, &name, &role, &enabled, &leaderboardOptIn, &leaderboardMaskName, &dataUsageEnabled, &maxConcurrency, &created, &balance, &reserved, &permissions, &groups, &inviterID, &inviterEmail, &inviterName) != nil {
			continue
		}
		out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "enabled": enabled, "leaderboard_opt_in": leaderboardOptIn, "leaderboard_mask_name": leaderboardMaskName, "data_usage_enabled": dataUsageEnabled, "max_concurrency": maxConcurrency, "balance": balance, "reserved": reserved, "permissions": permissions, "groups": groups, "inviter_id": inviterID, "inviter_email": inviterEmail, "inviter_name": inviterName, "created_at": created})
	}
	writePaged(w, out, total, page, pageSize)
}

func (s *Service) updateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID                  *int64          `json:"id"`
		Email               *string         `json:"email"`
		Name                *string         `json:"name"`
		Password            *string         `json:"password"`
		Role                *string         `json:"role"`
		Enabled             *bool           `json:"enabled"`
		Permissions         *[]string       `json:"permissions"`
		Groups              *[]string       `json:"groups"`
		Balance             *float64        `json:"balance"`
		Note                *string         `json:"note"`
		LeaderboardOptIn    *bool           `json:"leaderboard_opt_in"`
		LeaderboardMaskName *bool           `json:"leaderboard_mask_name"`
		DataUsageEnabled    *bool           `json:"data_usage_enabled"`
		MaxConcurrency      json.RawMessage `json:"max_concurrency"`
		InviterID           json.RawMessage `json:"inviter_id"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid user update")
		return
	}
	if in.ID == nil && in.Email == nil && in.Name == nil && in.Password == nil && in.Role == nil && in.Enabled == nil && in.Permissions == nil && in.Groups == nil && in.Balance == nil && in.LeaderboardOptIn == nil && in.LeaderboardMaskName == nil && in.DataUsageEnabled == nil && len(in.MaxConcurrency) == 0 && in.InviterID == nil {
		writeError(w, 400, "invalid_request", "at least one user field is required")
		return
	}
	if in.ID != nil && (*in.ID <= 0 || *in.ID > maxEditableUserID) {
		writeError(w, 400, "invalid_request", "user id must be a positive integer up to 9007199254740991")
		return
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		writeError(w, 400, "invalid_request", "name is required")
		return
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		parsed, err := mail.ParseAddress(email)
		if err != nil || parsed.Address != email {
			writeError(w, 400, "invalid_request", "email is invalid")
			return
		}
	}
	if in.Role != nil && *in.Role != "user" && *in.Role != "operator" && *in.Role != "admin" {
		writeError(w, 400, "invalid_request", "role must be user, operator, or admin")
		return
	}
	if in.Password != nil && !validPasswordLength(*in.Password) {
		writeError(w, 400, "invalid_request", "password must be between 8 and 72 characters")
		return
	}
	if in.Permissions != nil {
		seen := map[string]bool{}
		for _, permission := range *in.Permissions {
			if !availablePermissions[permission] || seen[permission] {
				writeError(w, 400, "invalid_request", "invalid permissions")
				return
			}
			seen[permission] = true
		}
	}
	if in.Balance != nil && !validUserBalance(*in.Balance) {
		writeError(w, 400, "invalid_request", "balance must be a non-negative number up to 999999999999")
		return
	}
	if in.Note != nil {
		note := strings.TrimSpace(*in.Note)
		if in.Balance == nil {
			writeError(w, 400, "invalid_request", "note can only be provided with balance")
			return
		}
		if len(note) > maxWalletNoteLength {
			writeError(w, 400, "invalid_request", "note must be at most 500 characters")
			return
		}
		*in.Note = note
	}
	var maxConcurrency *int
	if len(in.MaxConcurrency) > 0 {
		raw := strings.TrimSpace(string(in.MaxConcurrency))
		if raw != "null" {
			var value int
			if json.Unmarshal(in.MaxConcurrency, &value) != nil || value <= 0 || value > 10000 {
				writeError(w, 400, "invalid_request", "max_concurrency must be between 1 and 10000, or null")
				return
			}
			maxConcurrency = &value
		}
	}
	var inviterID *int64
	if len(in.InviterID) > 0 && strings.TrimSpace(string(in.InviterID)) != "null" {
		var value int64
		if json.Unmarshal(in.InviterID, &value) != nil || value <= 0 || value > maxEditableUserID {
			writeError(w, 400, "invalid_request", "inviter_id must be a positive integer up to 9007199254740991, or null")
			return
		}
		inviterID = &value
	}
	passwordHash := ""
	if in.Password != nil {
		var err error
		passwordHash, err = hashPassword(*in.Password)
		if err != nil {
			writeError(w, 500, "internal_error", "could not secure password")
			return
		}
	}
	actor := accountFromContext(r)
	userID := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not update user")
		return
	}
	defer tx.Rollback(r.Context())
	var currentRole string
	var currentEnabled bool
	if err = tx.QueryRow(r.Context(), `select role,enabled from users where id=$1 for update`, userID).Scan(&currentRole, &currentEnabled); err != nil {
		writeError(w, 404, "not_found", "user not found")
		return
	}
	resultingRole := currentRole
	if in.Role != nil {
		resultingRole = *in.Role
	}
	resultingEnabled := currentEnabled
	if in.Enabled != nil {
		resultingEnabled = *in.Enabled
	}
	if actor.userID == userID && (resultingRole != "admin" || !resultingEnabled) {
		writeError(w, 400, "invalid_request", "cannot remove or disable your own administrator account")
		return
	}
	if len(in.InviterID) > 0 {
		if inviterID != nil {
			if strconv.FormatInt(*inviterID, 10) == userID {
				writeError(w, 400, "invalid_request", "user cannot invite themselves")
				return
			}
			var exists bool
			if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, *inviterID).Scan(&exists); err != nil || !exists {
				writeError(w, 400, "invalid_request", "inviter not found")
				return
			}
			var cycle bool
			if err = tx.QueryRow(r.Context(), `with recursive chain(id) as (select invitee_id from invitations where inviter_id=$2 union all select i.invitee_id from invitations i join chain c on i.inviter_id=c.id) select exists(select 1 from chain where id=$1)`, userID, *inviterID).Scan(&cycle); err != nil {
				writeError(w, 500, "internal_error", "could not validate inviter")
				return
			}
			if cycle {
				writeError(w, 400, "invalid_request", "inviter would create a circular relationship")
				return
			}
		}
	}
	changed := map[string]any{}
	if in.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*in.Email))
		if _, err = tx.Exec(r.Context(), `update users set email=$1 where id=$2`, email, userID); err != nil {
			writeError(w, 409, "conflict", "email already exists or user could not be updated")
			return
		}
		changed["email"] = email
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if _, err = tx.Exec(r.Context(), `update users set name=$1 where id=$2`, name, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update name")
			return
		}
		changed["name"] = name
	}
	if in.Password != nil {
		if _, err = tx.Exec(r.Context(), `update users set password_hash=$1, must_change_password=true where id=$2`, passwordHash, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update password")
			return
		}
		if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not revoke sessions after password change")
			return
		}
		changed["password"] = true
		changed["must_change_password"] = true
		changed["sessions_revoked"] = true
	}
	if in.Role != nil {
		if _, err = tx.Exec(r.Context(), `update users set role=$1 where id=$2`, *in.Role, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update role")
			return
		}
		if currentRole == "admin" && *in.Role != "admin" {
			if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
				writeError(w, 500, "internal_error", "could not revoke sessions after role change")
				return
			}
			changed["sessions_revoked"] = true
		}
		changed["role"] = *in.Role
	}
	if in.Enabled != nil {
		if _, err = tx.Exec(r.Context(), `update users set enabled=$1 where id=$2`, *in.Enabled, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update status")
			return
		}
		if !*in.Enabled {
			if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
				writeError(w, 500, "internal_error", "could not revoke sessions after disable")
				return
			}
			if _, err = tx.Exec(r.Context(), `update api_keys set revoked_at=coalesce(revoked_at, now()) where user_id=$1 and revoked_at is null`, userID); err != nil {
				writeError(w, 500, "internal_error", "could not revoke API keys after disable")
				return
			}
			changed["sessions_revoked"] = true
			changed["api_keys_revoked"] = true
		}
		changed["enabled"] = *in.Enabled
	}
	if in.LeaderboardOptIn != nil {
		if _, err = tx.Exec(r.Context(), `update users set leaderboard_opt_in=$1 where id=$2`, *in.LeaderboardOptIn, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update leaderboard opt-in")
			return
		}
		changed["leaderboard_opt_in"] = *in.LeaderboardOptIn
	}
	if in.LeaderboardMaskName != nil {
		if _, err = tx.Exec(r.Context(), `update users set leaderboard_mask_name=$1 where id=$2`, *in.LeaderboardMaskName, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update leaderboard name masking")
			return
		}
		changed["leaderboard_mask_name"] = *in.LeaderboardMaskName
	}
	if in.DataUsageEnabled != nil {
		if _, err = tx.Exec(r.Context(), `update users set data_usage_enabled=$1 where id=$2`, *in.DataUsageEnabled, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update data usage setting")
			return
		}
		changed["data_usage_enabled"] = *in.DataUsageEnabled
	}
	if len(in.MaxConcurrency) > 0 {
		if _, err = tx.Exec(r.Context(), `update users set max_concurrency=$1 where id=$2`, maxConcurrency, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update concurrency limit")
			return
		}
		changed["max_concurrency"] = maxConcurrency
	}
	var oldBalance float64
	if in.Balance != nil {
		if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict(user_id) do nothing`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not load wallet")
			return
		}
		if err = tx.QueryRow(r.Context(), `select balance from user_wallets where user_id=$1 for update`, userID).Scan(&oldBalance); err != nil {
			writeError(w, 500, "internal_error", "could not lock wallet")
			return
		}
		if _, err = tx.Exec(r.Context(), `update user_wallets set balance=$1,updated_at=now() where user_id=$2`, *in.Balance, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update balance")
			return
		}
		changed["balance"] = *in.Balance
		if *in.Balance != oldBalance {
			id, _ := randomID()
			note := ""
			if in.Note != nil {
				note = strings.TrimSpace(*in.Note)
			}
			if note == "" {
				note = "管理员修改用户余额"
			}
			kind := "adjustment"
			if *in.Balance > oldBalance {
				kind = "topup"
			}
			if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,note) values($1,$2,$3,$4,$5,$6)`, id, userID, *in.Balance-oldBalance, *in.Balance, kind, note); err != nil {
				writeError(w, 500, "internal_error", "could not record balance change")
				return
			}
		}
	}
	if in.Permissions != nil {
		if _, err = tx.Exec(r.Context(), `delete from user_permissions where user_id=$1`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update permissions")
			return
		}
		for _, permission := range *in.Permissions {
			if _, err = tx.Exec(r.Context(), `insert into user_permissions(user_id,permission) values($1,$2)`, userID, permission); err != nil {
				writeError(w, 500, "internal_error", "could not update permissions")
				return
			}
		}
		changed["permissions"] = *in.Permissions
	}
	if in.Groups != nil {
		resolvedGroups := make([]string, 0, len(*in.Groups))
		seenGroups := map[string]bool{}
		for _, group := range *in.Groups {
			var groupID string
			if err = tx.QueryRow(r.Context(), `select id from groups where id::text=$1 or name=$1`, strings.TrimSpace(group)).Scan(&groupID); err != nil {
				writeError(w, 400, "invalid_request", "unknown group")
				return
			}
			if !seenGroups[groupID] {
				resolvedGroups = append(resolvedGroups, groupID)
				seenGroups[groupID] = true
			}
		}
		if _, err = tx.Exec(r.Context(), `delete from user_groups where user_id=$1`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update groups")
			return
		}
		for _, groupID := range resolvedGroups {
			if _, err = tx.Exec(r.Context(), `insert into user_groups(user_id,group_id) values($1,$2)`, userID, groupID); err != nil {
				writeError(w, 500, "internal_error", "could not update groups")
				return
			}
		}
		if _, err = tx.Exec(r.Context(), `update api_keys set group_id=null where user_id=$1 and group_id is not null and not exists(select 1 from user_groups ug where ug.user_id=$1 and ug.group_id=api_keys.group_id)`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update API key groups")
			return
		}
		changed["groups"] = resolvedGroups
	}
	if len(in.InviterID) > 0 {
		if inviterID == nil {
			if _, err = tx.Exec(r.Context(), `delete from invitations where invitee_id=$1`, userID); err != nil {
				writeError(w, 500, "internal_error", "could not clear inviter")
				return
			}
		} else if _, err = tx.Exec(r.Context(), `insert into invitations(id,inviter_id,invitee_id,code,inviter_reward,invitee_reward) select $1,$2,$3,'admin',s.inviter_reward,s.invitee_reward from site_settings s where s.id=true on conflict (invitee_id) do update set inviter_id=excluded.inviter_id`, func() string { id, _ := randomID(); return id }(), *inviterID, userID); err != nil {
			writeError(w, 400, "invalid_request", "could not update inviter")
			return
		}
		changed["inviter_id"] = inviterID
	}
	if in.ID != nil {
		if _, err = tx.Exec(r.Context(), `update users set id=$1 where id=$2`, *in.ID, userID); err != nil {
			writeError(w, 409, "conflict", "user id already exists or could not be updated")
			return
		}
		// Never move the auto-increment sequence backward so future
		// registrations cannot collide with an admin-assigned id.
		if _, err = tx.Exec(r.Context(), `select setval('users_id_seq', greatest((select last_value from users_id_seq), $1), true)`, *in.ID); err != nil {
			writeError(w, 500, "internal_error", "could not update user id")
			return
		}
		userID = strconv.FormatInt(*in.ID, 10)
		changed["id"] = *in.ID
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not update user")
		return
	}
	if in.Balance != nil && *in.Balance != oldBalance {
		note := ""
		if in.Note != nil {
			note = *in.Note
		}
		s.audit(r, "wallet.adjusted", "user", userID, map[string]any{"amount": *in.Balance - oldBalance, "balance_after": *in.Balance, "note": note})
	}
	s.audit(r, "user.updated", "user", userID, changed)
	if len(in.MaxConcurrency) > 0 {
		s.userConcurrencyCache.invalidate(userID)
	}
	if _, ok := changed["groups"]; ok {
		s.invalidateChannels()
	}
	writeJSON(w, 200, map[string]any{"id": userID, "updated": changed})
}

func (s *Service) listGroups(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	var total int
	if err := s.db.QueryRow(r.Context(), `select count(*) from groups`).Scan(&total); err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	rows, err := s.db.Query(r.Context(), `select id,name,display_name,description,multiplier,max_concurrency,"public",created_at from groups order by name limit $1 offset $2`, pageSize, offset)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name string
		var displayName, description *string
		var multiplier, maxConcurrency, created any
		var public bool
		if rows.Scan(&id, &name, &displayName, &description, &multiplier, &maxConcurrency, &public, &created) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "display_name": displayName, "description": description, "multiplier": multiplier, "max_concurrency": maxConcurrency, "public": public, "created_at": created})
		}
	}
	writePaged(w, data, total, page, pageSize)
}

func (s *Service) listGroupNames(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select name from groups order by name`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	writeJSON(w, 200, map[string]any{"data": names})
}

func (s *Service) groupNamesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `select g.name from groups g join user_groups ug on ug.group_id=g.id where ug.user_id=$1 order by g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func firstGroup(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	return groups[0]
}

func (s *Service) accountGroups(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	userGroups, err := s.groupNamesForUser(r.Context(), account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	rows, err := s.db.Query(r.Context(), `
		select g.id,g.name,g.display_name,g.description,g.multiplier,g."public",g.created_at from groups g
		left join user_groups ug on ug.group_id=g.id and ug.user_id=$1
		where g."public" or ug.user_id is not null
		order by g.name`, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	groups := []map[string]any{}
	names := []string{}
	for rows.Next() {
		var id, name string
		var displayName, description *string
		var multiplier, created any
		var public bool
		if rows.Scan(&id, &name, &displayName, &description, &multiplier, &public, &created) == nil {
			groups = append(groups, map[string]any{"id": id, "name": name, "display_name": displayName, "description": description, "multiplier": multiplier, "public": public, "created_at": created})
			names = append(names, name)
		}
	}
	writeJSON(w, 200, map[string]any{"data": names, "groups": groups, "user_groups": userGroups, "user_group": firstGroup(userGroups)})
}

func (s *Service) myGroups(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	names, err := s.groupNamesForUser(r.Context(), key.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	writeJSON(w, 200, map[string]any{"data": names, "user_groups": names, "user_group": firstGroup(names)})
}

func (s *Service) createGroup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name           string  `json:"name"`
		DisplayName    string  `json:"display_name"`
		Description    string  `json:"description"`
		Multiplier     float64 `json:"multiplier"`
		MaxConcurrency int     `json:"max_concurrency"`
		Public         bool    `json:"public"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "name is required")
		return
	}
	name := strings.TrimSpace(in.Name)
	if !validGroupName(name) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if !validGroupDisplayName(displayName) {
		writeError(w, 400, "invalid_request", "display name must be 100 characters or fewer")
		return
	}
	description := strings.TrimSpace(in.Description)
	if !validGroupDescription(description) {
		writeError(w, 400, "invalid_request", "description must be 500 characters or fewer")
		return
	}
	if in.Multiplier == 0 {
		in.Multiplier = 1
	}
	if !validGroupMultiplier(in.Multiplier) {
		writeError(w, 400, "invalid_request", "multiplier must be between 0 and 1000")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into groups(id,name,display_name,description,multiplier,max_concurrency,"public") values($1,$2,nullif($3,''),nullif($4,''),$5,$6,$7)`, id, name, displayName, description, in.Multiplier, in.MaxConcurrency, in.Public)
	if err != nil {
		writeError(w, 409, "conflict", "group name already exists")
		return
	}
	s.audit(r, "group.created", "group", id, map[string]any{"name": name, "display_name": displayName, "description": description, "multiplier": in.Multiplier, "max_concurrency": in.MaxConcurrency, "public": in.Public})
	writeJSON(w, 201, map[string]any{"id": id, "name": name, "display_name": nullStr(displayName), "description": nullStr(description), "multiplier": in.Multiplier, "max_concurrency": in.MaxConcurrency, "public": in.Public})
}

func (s *Service) importGroups(w http.ResponseWriter, r *http.Request) {
	var values map[string]float64
	if decode(r, &values) != nil || len(values) == 0 {
		writeError(w, 400, "invalid_request", "a non-empty name-to-multiplier object is required")
		return
	}
	if len(values) > maxGroupImportCount {
		writeError(w, 400, "invalid_request", "at most 500 groups can be imported at once")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not import groups")
		return
	}
	defer tx.Rollback(r.Context())
	for name, multiplier := range values {
		name = strings.TrimSpace(name)
		if !validGroupName(name) || !validGroupMultiplier(multiplier) {
			writeError(w, 400, "invalid_request", "group names must be 1-100 characters and multipliers must be between 0 and 1000")
			return
		}
		id, _ := randomID()
		if _, err = tx.Exec(r.Context(), `insert into groups(id,name,multiplier) values($1,$2,$3) on conflict(name) do update set multiplier=excluded.multiplier`, id, name, multiplier); err != nil {
			writeError(w, 409, "conflict", "could not import groups")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not import groups")
		return
	}
	s.groupCache.clear()
	s.audit(r, "groups.imported", "group", "bulk", map[string]any{"count": len(values)})
	writeJSON(w, 200, map[string]any{"count": len(values)})
}

func (s *Service) updateGroup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Multiplier     float64 `json:"multiplier"`
		MaxConcurrency int     `json:"max_concurrency"`
		Public         bool    `json:"public"`
		DisplayName    *string `json:"display_name"`
		Description    *string `json:"description"`
	}
	if decode(r, &in) != nil || !validGroupMultiplier(in.Multiplier) {
		writeError(w, 400, "invalid_request", "multiplier must be between 0 and 1000")
		return
	}
	displayName := ptrString(in.DisplayName)
	description := ptrString(in.Description)
	if displayName != nil && !validGroupDisplayName(displayName.(string)) {
		writeError(w, 400, "invalid_request", "display name must be 100 characters or fewer")
		return
	}
	if description != nil && !validGroupDescription(description.(string)) {
		writeError(w, 400, "invalid_request", "description must be 500 characters or fewer")
		return
	}
	result, err := s.db.Exec(r.Context(), `update groups set multiplier=$1, max_concurrency=$2, "public"=$3, display_name=case when $4::text is null then display_name else nullif($4::text,'') end, description=case when $5::text is null then description else nullif($5::text,'') end where id=$6`, in.Multiplier, in.MaxConcurrency, in.Public, displayName, description, r.PathValue("id"))
	if err != nil || result.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "group not found")
		return
	}
	s.groupCache.invalidate(r.PathValue("id"))
	s.groupConcurrencyCache.invalidate(r.PathValue("id"))
	s.audit(r, "group.updated", "group", r.PathValue("id"), map[string]any{"multiplier": in.Multiplier, "max_concurrency": in.MaxConcurrency, "public": in.Public, "display_name": displayName, "description": description})
	writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "multiplier": in.Multiplier, "max_concurrency": in.MaxConcurrency, "public": in.Public, "display_name": displayName, "description": description})
}

func (s *Service) batchUpdateGroups(w http.ResponseWriter, r *http.Request) {
	type groupUpdate struct {
		ID             string  `json:"id"`
		Multiplier     float64 `json:"multiplier"`
		MaxConcurrency int     `json:"max_concurrency"`
		Public         bool    `json:"public"`
		DisplayName    *string `json:"display_name"`
		Description    *string `json:"description"`
	}
	var in struct {
		Groups []groupUpdate `json:"groups"`
	}
	if decode(r, &in) != nil || len(in.Groups) == 0 {
		writeError(w, 400, "invalid_request", "a non-empty list of group updates is required")
		return
	}
	if len(in.Groups) > 100 {
		writeError(w, 400, "invalid_request", "at most 100 groups at a time")
		return
	}
	for _, update := range in.Groups {
		if strings.TrimSpace(update.ID) == "" {
			writeError(w, 400, "invalid_request", "every group update requires an id")
			return
		}
		if !validGroupMultiplier(update.Multiplier) {
			writeError(w, 400, "invalid_request", "multiplier must be between 0 and 1000")
			return
		}
		if update.MaxConcurrency < 0 {
			writeError(w, 400, "invalid_request", "max_concurrency must be 0 or greater")
			return
		}
		if ptrString(update.DisplayName) != nil && !validGroupDisplayName(ptrString(update.DisplayName).(string)) {
			writeError(w, 400, "invalid_request", "display name must be 100 characters or fewer")
			return
		}
		if ptrString(update.Description) != nil && !validGroupDescription(ptrString(update.Description).(string)) {
			writeError(w, 400, "invalid_request", "description must be 500 characters or fewer")
			return
		}
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not update groups")
		return
	}
	defer tx.Rollback(r.Context())
	var affected int64
	for _, update := range in.Groups {
		result, err := tx.Exec(r.Context(), `update groups set multiplier=$1, max_concurrency=$2, "public"=$3, display_name=case when $4::text is null then display_name else nullif($4::text,'') end, description=case when $5::text is null then description else nullif($5::text,'') end where id=$6`, update.Multiplier, update.MaxConcurrency, update.Public, ptrString(update.DisplayName), ptrString(update.Description), update.ID)
		if err != nil {
			writeError(w, 500, "internal_error", "could not update groups")
			return
		}
		affected += result.RowsAffected()
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not update groups")
		return
	}
	for _, update := range in.Groups {
		s.groupCache.invalidate(update.ID)
		s.groupConcurrencyCache.invalidate(update.ID)
	}
	s.audit(r, "groups.batch_updated", "group", "batch", map[string]any{"count": affected})
	writeJSON(w, 200, map[string]any{"affected": affected})
}

func (s *Service) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.Exec(r.Context(), `delete from groups where id=$1`, id)
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete group")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "group not found")
		return
	}
	s.groupCache.invalidate(id)
	s.groupConcurrencyCache.invalidate(id)
	s.audit(r, "group.deleted", "group", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func catalogModelID(model string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(model))))
	return "catalog-" + hex.EncodeToString(sum[:12])
}

func (s *Service) modelCatalog(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	providers := s.providers(r)
	rows, err := s.db.Query(r.Context(), `
		with available as (
			select trim(jsonb_array_elements_text(c.models)) as model, c.id as channel_id
			from channels c where c.enabled
			union
			select trim(m.public_model), c.id from model_routes m join channels c on c.id=m.channel_id
			where m.enabled and not m.hidden and c.enabled and trim(m.public_model) <> ''
		), catalog as (
			select distinct a.model, coalesce(g.id::text, '__public') as group_id,
				coalesce(g.display_name, g.name, '公共') as group_name, coalesce(g.multiplier, 1) as group_multiplier,
				coalesce(g."public", false) as group_public
			from available a
			left join channel_groups cg on cg.channel_id=a.channel_id
			left join groups g on g.id=cg.group_id
			where g.id is null or g."public" or exists(select 1 from user_groups ug where ug.user_id=nullif($1, '')::bigint and ug.group_id=g.id)
		)
		select c.model,c.group_id,c.group_name,c.group_multiplier,c.group_public,
			p.input_per_million,p.cached_input_per_million,p.output_per_million,p.multiplier
		from catalog c left join pricing_rules p on p.model=c.model and p.enabled
		order by c.model,c.group_name`, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	type catalogModel struct {
		ID, Model                         string
		Input, Cached, Output, Multiplier any
		Groups                            []map[string]any
	}
	models := map[string]*catalogModel{}
	order := []string{}
	groups := map[string]map[string]any{}
	for rows.Next() {
		var model, groupID, groupName string
		var groupMultiplier any
		var groupPublic bool
		var input, cached, output, modelMultiplier any
		if rows.Scan(&model, &groupID, &groupName, &groupMultiplier, &groupPublic, &input, &cached, &output, &modelMultiplier) != nil {
			continue
		}
		group := map[string]any{"id": groupID, "name": groupName, "multiplier": groupMultiplier, "public": groupPublic}
		groups[groupID] = group
		item := models[model]
		if item == nil {
			item = &catalogModel{Model: model, Input: input, Cached: cached, Output: output, Multiplier: modelMultiplier, Groups: []map[string]any{}}
			item.ID = catalogModelID(model)
			models[model] = item
			order = append(order, model)
		}
		item.Groups = append(item.Groups, group)
	}
	data := make([]map[string]any, 0, len(order))
	for _, model := range order {
		item := models[model]
		provider := providerForModel(item.Model, providers)
		data = append(data, map[string]any{"id": item.ID, "model": item.Model, "provider": provider.Name, "provider_slug": provider.Slug, "input_per_million": item.Input, "cached_input_per_million": item.Cached, "output_per_million": item.Output, "multiplier": item.Multiplier, "groups": item.Groups})
	}
	groupList := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		groupList = append(groupList, group)
	}
	sort.Slice(groupList, func(i, j int) bool { return groupList[i]["name"].(string) < groupList[j]["name"].(string) })
	writeJSON(w, 200, map[string]any{"data": data, "groups": groupList})
}

func (s *Service) setGroups(w http.ResponseWriter, r *http.Request, table, column, entity, entityType string) {
	var in struct {
		Groups   []string `json:"groups"`
		GroupIDs []string `json:"group_ids"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "groups are required")
		return
	}
	var entityExists bool
	if s.db.QueryRow(r.Context(), `select exists(select 1 from `+map[string]string{"user_groups": "users", "channel_groups": "channels"}[table]+` where id=$1)`, entity).Scan(&entityExists) != nil || !entityExists {
		writeError(w, 404, "not_found", entityType+" not found")
		return
	}
	refs := append(in.Groups, in.GroupIDs...)
	resolved := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		var id string
		if s.db.QueryRow(r.Context(), `select id from groups where id=$1 or name=$2`, ref, ref).Scan(&id) != nil {
			writeError(w, 400, "invalid_request", "unknown group")
			return
		}
		resolved[id] = true
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not update groups")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `delete from `+table+` where `+column+`=$1`, entity); err != nil {
		writeError(w, 500, "internal_error", "could not update groups")
		return
	}
	for id := range resolved {
		if _, err = tx.Exec(r.Context(), `insert into `+table+`(`+column+`,group_id) values($1,$2)`, entity, id); err != nil {
			writeError(w, 500, "internal_error", "could not update groups")
			return
		}
	}
	if table == "user_groups" {
		if _, err = tx.Exec(r.Context(), `update api_keys set group_id=null where user_id=$1 and group_id is not null and not exists(select 1 from user_groups ug where ug.user_id=$1 and ug.group_id=api_keys.group_id)`, entity); err != nil {
			writeError(w, 500, "internal_error", "could not update API key groups")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not update groups")
		return
	}
	s.audit(r, entityType+".groups_changed", entityType, entity, map[string]any{"groups": refs})
	writeJSON(w, 200, map[string]any{"groups": refs, "group_ids": sortedKeys(resolved)})
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Service) setUserGroups(w http.ResponseWriter, r *http.Request) {
	s.setGroups(w, r, "user_groups", "user_id", r.PathValue("id"), "user")
}
func (s *Service) setChannelGroups(w http.ResponseWriter, r *http.Request) {
	s.setGroups(w, r, "channel_groups", "channel_id", r.PathValue("id"), "channel")
}

var availablePermissions = map[string]bool{
	"users.read": true, "users.manage": true, "keys.manage": true, "channels.read": true,
	"channels.manage": true, "logs.read": true, "pricing.read": true, "pricing.manage": true,
	"audit.read": true, "wallets.manage": true, "routes.manage": true, "quotas.manage": true,
	"system.manage": true,
}

func validFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validNonNegativeFinite(value float64) bool {
	return validFinite(value) && value >= 0
}

func validPositiveFinite(value float64) bool {
	return validFinite(value) && value > 0
}

const maxGroupMultiplier = 1000.0
const maxPricingMultiplier = 1000.0
const maxPricingRate = 1_000_000.0
const maxPricePerQuotaUnit = 1_000_000.0
const maxNewAPIPricingModels = 5000
const maxWalletAdjustAmount = 1_000_000_000.0
const maxUserBalance = 999_999_999_999.0
const maxWalletNoteLength = 500
const maxQuotaLimit = int64(1_000_000_000_000)

// 2^53-1: the largest integer the web console can represent exactly.
const maxEditableUserID = int64(9007199254740991)

func validPricePerQuotaUnit(value float64) bool {
	return validNonNegativeFinite(value) && value <= maxPricePerQuotaUnit
}

func validGroupMultiplier(value float64) bool {
	return validNonNegativeFinite(value) && value <= maxGroupMultiplier
}

func validWalletAdjustAmount(value float64) bool {
	return validFinite(value) && value != 0 && value >= -maxWalletAdjustAmount && value <= maxWalletAdjustAmount
}

func validUserBalance(value float64) bool {
	return validNonNegativeFinite(value) && value <= maxUserBalance
}

func validQuotaLimit(value *int64) bool {
	if value == nil {
		return true
	}
	return *value >= 0 && *value <= maxQuotaLimit
}

func validQuotaCost(value *float64) bool {
	if value == nil {
		return true
	}
	return validNonNegativeFinite(*value) && *value <= maxKeyQuotaCost
}

func validPricingMultiplier(value float64) bool {
	return validPositiveFinite(value) && value <= maxPricingMultiplier
}

func validPricingRate(value float64) bool {
	return validNonNegativeFinite(value) && value <= maxPricingRate
}

func validPricingModel(model string) bool {
	return validModelName(model)
}

const maxPricingTiers = 20
const maxPricingTimeRules = 20
const maxTimeRuleNameLength = 100

func validTimeWindow(start, end int) bool {
	return start >= 0 && start < 1440 && end > 0 && end <= 1440 && start != end
}

func validWeekdays(wd string) bool {
	if len(wd) != 7 {
		return false
	}
	for _, c := range wd {
		if c != '0' && c != '1' {
			return false
		}
	}
	return true
}

func validModelName(model string) bool {
	return len(model) > 0 && len(model) <= 200
}

func validChannelProvider(provider string) bool {
	return map[string]bool{"openai": true, "ollama": true, "kimi": true, "opencode_go": true, "anthropic": true, "deepseek": true, "commandcode": true, "custom": true}[provider]
}

func validChannelKeyType(keyType string) bool {
	return keyType == "single" || keyType == "multi"
}

// validUpstreamFormat accepts the channel wire formats the gateway understands:
// auto (from provider), plain OpenAI (Responses passthrough eligible),
// openai_chat (OpenAI wire format, chat-completions only), and Anthropic.
func validUpstreamFormat(format string) bool {
	return map[string]bool{"": true, "openai": true, "openai_chat": true, "anthropic": true}[format]
}

func validChannelPriority(priority int) bool {
	return priority >= -10000 && priority <= 10000
}

const maxChannelModels = 500
const maxGroupImportCount = 500

func sanitizeChannelModels(models []string) ([]string, bool) {
	if len(models) > maxChannelModels*2 {
		return nil, false
	}
	out := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		if len(model) > 200 {
			return nil, false
		}
		seen[model] = true
		out = append(out, model)
		if len(out) > maxChannelModels {
			return nil, false
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func validGroupName(name string) bool {
	return len(name) > 0 && len(name) <= 100
}

func validGroupDisplayName(name string) bool {
	return len(name) <= 100
}

func validGroupDescription(description string) bool {
	return len(description) <= 500
}

func nullStr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func ptrString(p *string) any {
	if p == nil {
		return nil
	}
	return strings.TrimSpace(*p)
}

func validChannelName(name string) bool {
	return len(name) > 0 && len(name) <= 100
}

const (
	maxChannelAPIKeyLen  = 4096
	maxChannelBaseURLLen = 2048
)

func validChannelAPIKey(value string) bool {
	return len(value) > 0 && len(value) <= maxChannelAPIKeyLen
}

func parseChannelAPIKeys(value string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, line := range strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' }) {
		key := strings.TrimSpace(line)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func validChannelBaseURL(value string) bool {
	return len(value) > 0 && len(value) <= maxChannelBaseURLLen && validUpstreamURL(value) == nil
}

func validAPIKeyName(name string) bool {
	return len(name) > 0 && len(name) <= 100
}

func validProviderSlug(slug string) bool {
	if len(slug) == 0 || len(slug) > 64 {
		return false
	}
	for i, r := range slug {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		if r == '-' && i > 0 && i < len(slug)-1 {
			continue
		}
		return false
	}
	return true
}

func redactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && u.Host != "" {
		if u.User != nil {
			name := u.User.Username()
			if name == "" {
				name = "user"
			}
			u.User = url.UserPassword(name, "***")
		}
		return u.String()
	}
	if at := strings.LastIndex(dsn, "@"); at > 0 {
		head := dsn[:at]
		if colon := strings.Index(head, ":"); colon >= 0 {
			return head[:colon+1] + "***" + dsn[at:]
		}
	}
	return "[redacted-dsn]"
}

func (s *Service) setUserRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if decode(r, &in) != nil || (in.Role != "user" && in.Role != "operator" && in.Role != "admin") {
		writeError(w, http.StatusBadRequest, "invalid_request", "role must be user, operator, or admin")
		return
	}
	actor := accountFromContext(r)
	userID := r.PathValue("id")
	if actor.userID == userID && in.Role != "admin" {
		writeError(w, http.StatusBadRequest, "invalid_request", "cannot remove your own administrator role")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update role")
		return
	}
	defer tx.Rollback(r.Context())
	var currentRole string
	if err = tx.QueryRow(r.Context(), `select role from users where id=$1 for update`, userID).Scan(&currentRole); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if _, err = tx.Exec(r.Context(), `update users set role=$1 where id=$2`, in.Role, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update role")
		return
	}
	sessionsRevoked := false
	if currentRole == "admin" && in.Role != "admin" {
		if _, err = tx.Exec(r.Context(), `delete from user_sessions where user_id=$1`, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not revoke sessions after role change")
			return
		}
		sessionsRevoked = true
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update role")
		return
	}
	s.audit(r, "user.role_changed", "user", userID, map[string]any{"role": in.Role, "sessions_revoked": sessionsRevoked})
	writeJSON(w, http.StatusOK, map[string]any{"role": in.Role, "sessions_revoked": sessionsRevoked})
}

func (s *Service) setUserPermissions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Permissions []string `json:"permissions"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "permissions are required")
		return
	}
	seen := map[string]bool{}
	for _, permission := range in.Permissions {
		if !availablePermissions[permission] || seen[permission] {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid permissions")
			return
		}
		seen[permission] = true
	}
	userID := r.PathValue("id")
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	defer tx.Rollback(r.Context())
	var exists bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from users where id=$1)`, userID).Scan(&exists); err != nil || !exists {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if _, err = tx.Exec(r.Context(), `delete from user_permissions where user_id=$1`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	for permission := range seen {
		if _, err = tx.Exec(r.Context(), `insert into user_permissions(user_id,permission) values($1,$2)`, userID, permission); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update permissions")
		return
	}
	s.audit(r, "user.permissions_changed", "user", userID, map[string]any{"permissions": in.Permissions})
	writeJSON(w, http.StatusOK, map[string]any{"permissions": in.Permissions})
}
func (s *Service) createKey(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID    string `json:"user_id"`
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
		GroupID   string `json:"group_id"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.UserID) == "" {
		writeError(w, 400, "invalid_request", "user_id and name are required")
		return
	}
	name := strings.TrimSpace(in.Name)
	if !validAPIKeyName(name) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	expires, err := parseExpiry(in.ExpiresAt)
	if err != nil {
		writeError(w, 400, "invalid_request", "expires_at must be RFC3339")
		return
	}
	secret, err := randomSecret("sk-xh-")
	if err != nil {
		writeError(w, 500, "internal_error", "key generation failed")
		return
	}
	id, _ := randomID()
	groupID, err := s.validKeyGroup(r.Context(), in.UserID, in.GroupID)
	if err != nil {
		writeError(w, 400, "invalid_request", "group must belong to user")
		return
	}
	encryptedSecret, err := crypt(s.cfg.EncryptionKey, secret, false)
	if err != nil {
		writeError(w, 500, "internal_error", "could not create API key")
		return
	}
	_, err = s.db.Exec(r.Context(), `insert into api_keys(id,user_id,name,key_prefix,secret_hash,secret_encrypted,expires_at,group_id) values($1,$2,$3,$4,$5,$6,$7,$8)`, id, in.UserID, name, secret[:12], hashSecret(secret), encryptedSecret, expires, groupID)
	if err != nil {
		writeError(w, 400, "invalid_request", "unknown user")
		return
	}
	s.audit(r, "api_key.created", "api_key", id, map[string]any{"user_id": in.UserID, "name": name})
	writeJSON(w, 201, map[string]any{"id": id, "name": name, "key": secret, "expires_at": expires, "group_id": groupID})
}
func (s *Service) listKeys(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	var total int
	if err := s.db.QueryRow(r.Context(), `select count(*) from api_keys`).Scan(&total); err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	rows, err := s.db.Query(r.Context(), `select k.id,k.user_id,k.name,k.key_prefix,k.expires_at,k.revoked_at,k.last_used_at,k.created_at,coalesce(k.group_id::text,''),coalesce(g.name,''),k.secret_encrypted<>'' from api_keys k left join groups g on g.id=k.group_id order by k.created_at desc limit $1 offset $2`, pageSize, offset)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, uid, name, prefix string
		var groupID, groupName string
		var revealable bool
		var expiry, revoked, used, created any
		rows.Scan(&id, &uid, &name, &prefix, &expiry, &revoked, &used, &created, &groupID, &groupName, &revealable)
		data = append(data, map[string]any{"id": id, "user_id": uid, "name": name, "key_prefix": prefix, "expires_at": expiry, "revoked_at": revoked, "last_used_at": used, "created_at": created, "group_id": groupID, "group_name": groupName, "revealable": revealable})
	}
	writePaged(w, data, total, page, pageSize)
}

func (s *Service) validKeyGroup(ctx context.Context, userID, groupRef string) (any, error) {
	groupRef = strings.TrimSpace(groupRef)
	if groupRef == "" {
		return nil, nil
	}
	var groupID string
	err := s.db.QueryRow(ctx, `select g.id from groups g where (g."public" or exists(select 1 from user_groups ug where ug.user_id=$1 and ug.group_id=g.id)) and (g.id::text=$2 or g.name=$2)`, userID, groupRef).Scan(&groupID)
	return groupID, err
}

func (s *Service) setKeyGroup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GroupID string `json:"group_id"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "group_id is required")
		return
	}
	var userID string
	if s.db.QueryRow(r.Context(), `select user_id from api_keys where id=$1`, r.PathValue("id")).Scan(&userID) != nil {
		writeError(w, 404, "not_found", "API key not found")
		return
	}
	groupID, err := s.validKeyGroup(r.Context(), userID, in.GroupID)
	if err != nil {
		writeError(w, 400, "invalid_request", "group must belong to user")
		return
	}
	_, err = s.db.Exec(r.Context(), `update api_keys set group_id=$1 where id=$2`, groupID, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "internal_error", "could not update API key group")
		return
	}
	s.audit(r, "api_key.group_changed", "api_key", r.PathValue("id"), map[string]any{"group_id": groupID})
	writeJSON(w, 200, map[string]any{"group_id": groupID})
}
func (s *Service) revokeKey(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `update api_keys set revoked_at=coalesce(revoked_at, now()) where id=$1`, r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "API key not found")
		return
	}
	s.audit(r, "api_key.revoked", "api_key", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Service) revealKey(w http.ResponseWriter, r *http.Request) {
	keyID := strings.TrimSpace(r.PathValue("id"))
	if keyID == "" {
		writeError(w, 400, "invalid_request", "key id is required")
		return
	}
	var encrypted string
	if err := s.db.QueryRow(r.Context(), `select secret_encrypted from api_keys where id=$1`, keyID).Scan(&encrypted); err != nil {
		writeError(w, 404, "not_found", "API key not found")
		return
	}
	if encrypted == "" {
		writeError(w, 404, "not_recoverable", "this key was created before recovery was enabled and cannot be revealed")
		return
	}
	secret, err := crypt(s.cfg.EncryptionKey, encrypted, true)
	if err != nil {
		writeError(w, 500, "internal_error", "could not decrypt API key")
		return
	}
	s.audit(r, "api_key.revealed", "api_key", keyID, nil)
	writeJSON(w, 200, map[string]any{"key": secret})
}
func (s *Service) createChannel(w http.ResponseWriter, r *http.Request) {
	type routeInput struct {
		PublicModel   string `json:"public_model"`
		UpstreamModel string `json:"upstream_model"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Hidden        bool   `json:"hidden"`
	}
	var in struct {
		Name           string                   `json:"name"`
		BaseURL        string                   `json:"base_url"`
		KeyType        string                   `json:"key_type"`
		APIKeys        string                   `json:"api_keys"`
		Models         []string                 `json:"models"`
		TestModel      string                   `json:"test_model"`
		Priority       int                      `json:"priority"`
		Groups         []string                 `json:"groups"`
		UserEmail      *string                  `json:"user_email"`
		Provider       string                   `json:"provider"`
		ModelRoutes    []routeInput             `json:"model_routes"`
		AutoDisable    *bool                    `json:"auto_disable"`
		Overrides      *channelRequestOverrides `json:"request_overrides"`
		UAPool         []string                 `json:"ua_pool"`
		UpstreamPath   string                   `json:"upstream_path"`
		UpstreamFormat string                   `json:"upstream_format"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "name, key_type, api_keys, and models are required")
		return
	}
	if err := validRequestOverrides(in.Overrides); err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	if err := validUAPool(in.UAPool); err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	name := strings.TrimSpace(in.Name)
	if !validChannelName(name) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	in.Name = name
	in.KeyType = strings.TrimSpace(in.KeyType)
	if in.KeyType == "" {
		in.KeyType = "single"
	}
	if !validChannelKeyType(in.KeyType) {
		writeError(w, 400, "invalid_request", "key_type must be single or multi")
		return
	}
	modelsList, ok := sanitizeChannelModels(in.Models)
	if !ok {
		writeError(w, 400, "invalid_request", "at least one non-empty model name is required")
		return
	}
	in.Models = modelsList
	in.TestModel = strings.TrimSpace(in.TestModel)
	if in.TestModel != "" && !validModelName(in.TestModel) {
		writeError(w, 400, "invalid_request", "test_model must be at most 200 characters")
		return
	}
	if in.Provider == "" {
		in.Provider = "openai"
	}
	if !validChannelProvider(in.Provider) {
		writeError(w, 400, "invalid_request", "unsupported provider")
		return
	}
	in.UpstreamFormat = strings.TrimSpace(in.UpstreamFormat)
	if !validUpstreamFormat(in.UpstreamFormat) {
		writeError(w, 400, "invalid_request", "unsupported upstream format")
		return
	}
	if !validChannelPriority(in.Priority) {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	groupIDs := []string{}
	seenGroups := map[string]bool{}
	for _, groupRef := range in.Groups {
		groupRef = strings.TrimSpace(groupRef)
		var groupID string
		if s.db.QueryRow(r.Context(), `select id from groups where id=$1 or name=$2`, groupRef, groupRef).Scan(&groupID) != nil {
			writeError(w, 400, "invalid_request", "unknown group")
			return
		}
		if !seenGroups[groupID] {
			seenGroups[groupID] = true
			groupIDs = append(groupIDs, groupID)
		}
	}
	var userID *string
	if in.UserEmail != nil && strings.TrimSpace(*in.UserEmail) != "" {
		var resolved string
		if s.db.QueryRow(r.Context(), `select id from users where lower(email)=lower($1)`, strings.TrimSpace(*in.UserEmail)).Scan(&resolved) != nil {
			writeError(w, 400, "invalid_request", "unknown user")
			return
		}
		userID = &resolved
	}
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	if !validChannelBaseURL(in.BaseURL) {
		writeError(w, 400, "invalid_request", "base_url must be 1-2048 characters and use HTTP or HTTPS")
		return
	}
	keys := parseChannelAPIKeys(in.APIKeys)
	if len(keys) == 0 {
		writeError(w, 400, "invalid_request", "at least one api_key is required")
		return
	}
	if in.KeyType == "single" && len(keys) > 1 {
		writeError(w, 400, "invalid_request", "single-key channels accept only one key")
		return
	}
	for _, key := range keys {
		if !validChannelAPIKey(key) {
			writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
			return
		}
	}
	for i := range in.ModelRoutes {
		in.ModelRoutes[i].PublicModel = strings.TrimSpace(in.ModelRoutes[i].PublicModel)
		in.ModelRoutes[i].UpstreamModel = strings.TrimSpace(in.ModelRoutes[i].UpstreamModel)
		if !validModelName(in.ModelRoutes[i].PublicModel) || !validModelName(in.ModelRoutes[i].UpstreamModel) {
			writeError(w, 400, "invalid_request", "public_model and upstream_model must be 1-200 characters")
			return
		}
		if in.ModelRoutes[i].Weight < 0 || in.ModelRoutes[i].Weight > 10000 {
			writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
			return
		}
		if in.ModelRoutes[i].Priority < -10000 || in.ModelRoutes[i].Priority > 10000 {
			writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
			return
		}
	}
	models, _ := json.Marshal(in.Models)
	overrides, _ := json.Marshal(normalizedOverrides(in.Overrides))
	uaPool, _ := json.Marshal(normalizedUAPool(in.UAPool))
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not create channel")
		return
	}
	defer tx.Rollback(r.Context())
	autoDisable := true
	if in.AutoDisable != nil {
		autoDisable = *in.AutoDisable
	}
	var id string
	err = tx.QueryRow(r.Context(), `insert into channels(name,base_url,api_key,models,test_model,priority,provider,key_type,auto_disable,request_overrides,ua_pool,user_id,upstream_path,upstream_format) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) returning id`, in.Name, strings.TrimRight(in.BaseURL, "/"), keys[0], models, in.TestModel, in.Priority, in.Provider, in.KeyType, autoDisable, overrides, uaPool, userID, in.UpstreamPath, in.UpstreamFormat).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, 409, "conflict", "channel name already exists")
			return
		}
		writeError(w, 500, "internal_error", "could not create channel")
		return
	}
	for i, rawKey := range keys {
		keyName := "key_" + strconv.Itoa(i+1)
		kid, _ := randomID()
		if _, kerr := tx.Exec(r.Context(), `insert into channel_api_keys(id,channel_id,name,key_encrypted,enabled) values($1,$2,$3,$4,true)`, kid, id, keyName, rawKey); kerr != nil {
			writeError(w, 500, "internal_error", "could not save api key")
			return
		}
	}
	for _, groupID := range groupIDs {
		if _, err = tx.Exec(r.Context(), `insert into channel_groups(channel_id,group_id) values($1,$2)`, id, groupID); err != nil {
			writeError(w, 400, "invalid_request", "unknown group")
			return
		}
	}
	for _, rt := range in.ModelRoutes {
		if rt.Weight == 0 {
			rt.Weight = 100
		}
		rid, _ := randomID()
		if _, err = tx.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight,hidden) values($1,$2,$3,$4,$5,$6,$7)`, rid, rt.PublicModel, rt.UpstreamModel, id, rt.Priority, rt.Weight, rt.Hidden); err != nil {
			writeError(w, 400, "invalid_request", "could not create model route")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not create channel")
		return
	}
	s.audit(r, "channel.created", "channel", id, map[string]any{"name": in.Name, "models": in.Models, "provider": in.Provider, "key_type": in.KeyType, "key_count": len(keys)})
	s.invalidateChannels()
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "models": in.Models, "provider": in.Provider, "key_type": in.KeyType, "enabled": true})
}

// copyChannel duplicates a channel's configuration, API keys, groups, model
// routes, and quota limits into a new channel. The copy is created disabled so
// the admin can review and rename it before enabling traffic.
func (s *Service) copyChannel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if r.ContentLength > 0 {
		if decode(r, &in) != nil {
			writeError(w, 400, "invalid_request", "invalid request body")
			return
		}
	}
	sourceID := r.PathValue("id")
	var sourceName, baseURL, apiKey, testModel, provider, keyType, upstreamPath, upstreamFormat string
	var models, overrides, uaPool []byte
	var priority, weight int
	var autoDisable bool
	var userID *string
	err := s.db.QueryRow(r.Context(), `select name,base_url,api_key,models,test_model,priority,weight,provider,key_type,auto_disable,request_overrides,ua_pool,upstream_path,upstream_format,user_id::text from channels where id=$1`, sourceID).Scan(&sourceName, &baseURL, &apiKey, &models, &testModel, &priority, &weight, &provider, &keyType, &autoDisable, &overrides, &uaPool, &upstreamPath, &upstreamFormat, &userID)
	if err != nil {
		writeError(w, 404, "not_found", "channel not found")
		return
	}
	newName := strings.TrimSpace(in.Name)
	if newName == "" {
		newName = s.nextChannelCopyName(r.Context(), sourceName)
	}
	if !validChannelName(newName) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not copy channel")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `insert into channels(name,base_url,api_key,models,test_model,priority,weight,provider,key_type,auto_disable,request_overrides,ua_pool,user_id,upstream_path,upstream_format,enabled) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,false) returning id`, newName, baseURL, apiKey, models, testModel, priority, weight, provider, keyType, autoDisable, overrides, uaPool, userID, upstreamPath, upstreamFormat).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, 409, "conflict", "channel name already exists")
			return
		}
		writeError(w, 500, "internal_error", "could not copy channel")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into channel_api_keys(channel_id,name,key_encrypted,enabled,priority) select $1,name,key_encrypted,enabled,priority from channel_api_keys where channel_id=$2 order by priority desc nulls last,created_at`, id, sourceID); err != nil {
		writeError(w, 500, "internal_error", "could not copy api keys")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into channel_groups(channel_id,group_id) select $1,group_id from channel_groups where channel_id=$2`, id, sourceID); err != nil {
		writeError(w, 500, "internal_error", "could not copy groups")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight,enabled,hidden,created_at) select gen_random_uuid(),public_model,upstream_model,$1,priority,weight,enabled,hidden,created_at from model_routes where channel_id=$2`, id, sourceID); err != nil {
		writeError(w, 500, "internal_error", "could not copy model routes")
		return
	}
	if _, err = tx.Exec(r.Context(), `insert into channel_quota_limits(channel_id,"window",max_requests,max_tokens) select $1,"window",max_requests,max_tokens from channel_quota_limits where channel_id=$2`, id, sourceID); err != nil {
		writeError(w, 500, "internal_error", "could not copy quota limits")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not copy channel")
		return
	}
	s.audit(r, "channel.copied", "channel", id, map[string]any{"source": sourceID, "name": newName, "provider": provider, "key_type": keyType})
	s.invalidateChannels()
	writeJSON(w, 201, map[string]any{"id": id, "name": newName, "models": models, "provider": provider, "key_type": keyType, "enabled": false})
}

// nextChannelCopyName returns a unique name for a duplicated channel by
// appending " (copy)", " (copy 2)", ... until the name is not taken.
func (s *Service) nextChannelCopyName(ctx context.Context, source string) string {
	return channelCopyName(source, func(name string) bool {
		var taken bool
		if s.db.QueryRow(ctx, `select exists(select 1 from channels where name=$1)`, name).Scan(&taken) != nil {
			return true
		}
		return taken
	})
}

// channelCopyName is the pure name-generation core of nextChannelCopyName.
func channelCopyName(source string, taken func(string) bool) string {
	for i := 1; ; i++ {
		var suffix string
		if i == 1 {
			suffix = " (copy)"
		} else {
			suffix = " (copy " + strconv.Itoa(i) + ")"
		}
		candidate := truncateRunes(source, 100-len(suffix)) + suffix
		if !taken(candidate) {
			return candidate
		}
	}
}

func truncateRunes(s string, n int) string {
	if n < 0 {
		n = 0
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func (s *Service) updateChannel(w http.ResponseWriter, r *http.Request) {
	type routeInput struct {
		PublicModel   string `json:"public_model"`
		UpstreamModel string `json:"upstream_model"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Hidden        bool   `json:"hidden"`
	}
	var in struct {
		Name           string                   `json:"name"`
		BaseURL        string                   `json:"base_url"`
		KeyType        string                   `json:"key_type"`
		APIKeys        string                   `json:"api_keys"`
		Models         []string                 `json:"models"`
		TestModel      string                   `json:"test_model"`
		Priority       int                      `json:"priority"`
		Provider       string                   `json:"provider"`
		Groups         []string                 `json:"groups"`
		UserEmail      *string                  `json:"user_email"`
		ModelRoutes    []routeInput             `json:"model_routes"`
		AutoDisable    *bool                    `json:"auto_disable"`
		Overrides      *channelRequestOverrides `json:"request_overrides"`
		UAPool         []string                 `json:"ua_pool"`
		UpstreamPath   string                   `json:"upstream_path"`
		UpstreamFormat string                   `json:"upstream_format"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "name and models are required")
		return
	}
	if err := validRequestOverrides(in.Overrides); err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	if err := validUAPool(in.UAPool); err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	name := strings.TrimSpace(in.Name)
	if !validChannelName(name) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	in.Name = name
	in.KeyType = strings.TrimSpace(in.KeyType)
	if in.KeyType != "" && !validChannelKeyType(in.KeyType) {
		writeError(w, 400, "invalid_request", "key_type must be single or multi")
		return
	}
	modelsList, ok := sanitizeChannelModels(in.Models)
	if !ok {
		writeError(w, 400, "invalid_request", "at least one non-empty model name is required")
		return
	}
	in.Models = modelsList
	in.TestModel = strings.TrimSpace(in.TestModel)
	if in.TestModel != "" && !validModelName(in.TestModel) {
		writeError(w, 400, "invalid_request", "test_model must be at most 200 characters")
		return
	}
	if in.Provider == "" {
		in.Provider = "openai"
	}
	if !validChannelProvider(in.Provider) {
		writeError(w, 400, "invalid_request", "unsupported provider")
		return
	}
	in.UpstreamFormat = strings.TrimSpace(in.UpstreamFormat)
	if !validUpstreamFormat(in.UpstreamFormat) {
		writeError(w, 400, "invalid_request", "unsupported upstream format")
		return
	}
	if !validChannelPriority(in.Priority) {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	if !validChannelBaseURL(in.BaseURL) {
		writeError(w, 400, "invalid_request", "base_url must be 1-2048 characters and use HTTP or HTTPS")
		return
	}
	for i := range in.ModelRoutes {
		in.ModelRoutes[i].PublicModel = strings.TrimSpace(in.ModelRoutes[i].PublicModel)
		in.ModelRoutes[i].UpstreamModel = strings.TrimSpace(in.ModelRoutes[i].UpstreamModel)
		if !validModelName(in.ModelRoutes[i].PublicModel) || !validModelName(in.ModelRoutes[i].UpstreamModel) {
			writeError(w, 400, "invalid_request", "public_model and upstream_model must be 1-200 characters")
			return
		}
		if in.ModelRoutes[i].Weight < 0 || in.ModelRoutes[i].Weight > 10000 {
			writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
			return
		}
		if in.ModelRoutes[i].Priority < -10000 || in.ModelRoutes[i].Priority > 10000 {
			writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
			return
		}
		if in.ModelRoutes[i].Weight == 0 {
			in.ModelRoutes[i].Weight = 100
		}
	}
	channelID := r.PathValue("id")
	var userID *string
	if in.UserEmail != nil {
		if strings.TrimSpace(*in.UserEmail) != "" {
			var resolved string
			if s.db.QueryRow(r.Context(), `select id from users where lower(email)=lower($1)`, strings.TrimSpace(*in.UserEmail)).Scan(&resolved) != nil {
				writeError(w, 400, "invalid_request", "unknown user")
				return
			}
			userID = &resolved
		}
	}
	models, _ := json.Marshal(in.Models)
	query := `update channels set name=$1,base_url=$2,models=$3,priority=$4,provider=$5,test_model=$6,upstream_path=$7,upstream_format=$8`
	args := []any{in.Name, strings.TrimRight(in.BaseURL, "/"), models, in.Priority, in.Provider, in.TestModel, in.UpstreamPath, in.UpstreamFormat}
	argIdx := 9
	if in.UserEmail != nil {
		query += `,user_id=$` + strconv.Itoa(argIdx)
		args = append(args, userID)
		argIdx++
	}
	if in.AutoDisable != nil {
		query += `,auto_disable=$` + strconv.Itoa(argIdx)
		args = append(args, *in.AutoDisable)
		argIdx++
	}
	if in.Overrides != nil {
		overrides, _ := json.Marshal(normalizedOverrides(in.Overrides))
		query += `,request_overrides=$` + strconv.Itoa(argIdx)
		args = append(args, string(overrides))
		argIdx++
	}
	if in.UAPool != nil {
		uaPool, _ := json.Marshal(normalizedUAPool(in.UAPool))
		query += `,ua_pool=$` + strconv.Itoa(argIdx)
		args = append(args, string(uaPool))
		argIdx++
	}
	keys := parseChannelAPIKeys(in.APIKeys)
	if len(keys) > 0 {
		if in.KeyType == "" {
			if len(keys) == 1 {
				in.KeyType = "single"
			} else {
				in.KeyType = "multi"
			}
		}
		if in.KeyType == "single" && len(keys) > 1 {
			writeError(w, 400, "invalid_request", "single-key channels accept only one key")
			return
		}
		for _, key := range keys {
			if !validChannelAPIKey(key) {
				writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
				return
			}
		}
		query += `,key_type=$` + strconv.Itoa(argIdx) + `,api_key=$` + strconv.Itoa(argIdx+1)
		args = append(args, in.KeyType, keys[0])
		argIdx += 2
		if err := s.replaceChannelAPIKeys(r.Context(), channelID, keys); err != nil {
			writeError(w, 500, "internal_error", "could not update api keys")
			return
		}
	}
	query += `,updated_at=now() where id=$` + strconv.Itoa(argIdx)
	args = append(args, channelID)
	result, err := s.db.Exec(r.Context(), query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, 409, "conflict", "channel name already exists")
			return
		}
		writeError(w, 500, "internal_error", "could not update channel")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel not found")
		return
	}
	if in.ModelRoutes != nil {
		if _, derr := s.db.Exec(r.Context(), `delete from model_routes where channel_id=$1`, channelID); derr != nil {
			writeError(w, 500, "internal_error", "could not update model routes")
			return
		}
		for _, rt := range in.ModelRoutes {
			if rt.Weight == 0 {
				rt.Weight = 100
			}
			rid, _ := randomID()
			if _, err = s.db.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight,hidden) values($1,$2,$3,$4,$5,$6,$7)`, rid, rt.PublicModel, rt.UpstreamModel, channelID, rt.Priority, rt.Weight, rt.Hidden); err != nil {
				writeError(w, 400, "invalid_request", "could not create model route")
				return
			}
		}
	}
	s.audit(r, "channel.updated", "channel", channelID, map[string]any{"name": in.Name, "models": in.Models, "provider": in.Provider, "key_type": in.KeyType})
	s.invalidateChannels()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) replaceChannelAPIKeys(ctx context.Context, channelID string, keys []string) error {
	existing := map[string]struct {
		name     string
		priority int
	}{}
	krows, err := s.db.Query(ctx, `select key_encrypted,name,priority from channel_api_keys where channel_id=$1`, channelID)
	if err != nil {
		return err
	}
	for krows.Next() {
		var enc, name string
		var priority int
		if krows.Scan(&enc, &name, &priority) != nil {
			continue
		}
		plain, err := channelKeyValue(s.cfg.EncryptionKey, enc)
		if err != nil {
			continue
		}
		existing[plain] = struct {
			name     string
			priority int
		}{name, priority}
	}
	krows.Close()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `delete from channel_api_keys where channel_id=$1`, channelID); err != nil {
		return err
	}
	for i, key := range keys {
		id, _ := randomID()
		keyName := "key_" + strconv.Itoa(i+1)
		priority := 100
		if prev, ok := existing[key]; ok {
			keyName = prev.name
			priority = prev.priority
		}
		if _, err := tx.Exec(ctx, `insert into channel_api_keys(id,channel_id,name,key_encrypted,enabled,priority) values($1,$2,$3,$4,true,$5)`, id, channelID, keyName, key, priority); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.invalidateChannels()
	return nil
}
func decodeUpstreamUsageWindows(raw []byte) []upstreamUsageWindow {
	windows := make([]upstreamUsageWindow, 0)
	if len(raw) == 0 || json.Unmarshal(raw, &windows) != nil || windows == nil {
		return make([]upstreamUsageWindow, 0)
	}
	return windows
}

func (s *Service) listChannels(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	var total int
	if err := s.db.QueryRow(r.Context(), `select count(*) from channels`).Scan(&total); err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	rows, err := s.db.Query(r.Context(), `select c.id,c.name,c.base_url,c.models,c.test_model,c.enabled,c.auto_disabled,c.disabled_reason,c.priority,c.weight,c.last_checked_at,c.last_error,c.created_at,c.updated_at,coalesce((select array_agg(cg.group_id order by cg.group_id) from channel_groups cg where cg.channel_id=c.id), '{}'),c.provider,c.key_type,(select count(*) from channel_api_keys ak where ak.channel_id=c.id and ak.enabled),c.auto_disable,c.request_overrides,c.ua_pool,coalesce(u.id::text,''),coalesce(u.email,''),coalesce(u.name,''),coalesce(agg.avg_duration_ms,0),agg.avg_first_token_ms,coalesce(agg.used_requests,0),coalesce(agg.used_tokens,0),cb.balance,cb.used,cb.total,coalesce(cb.currency,'USD'),cb.usage,coalesce(cb.supported,false),coalesce(cb.error,''),cb.fetched_at from channels c left join users u on u.id=c.user_id left join lateral (select cb.balance,cb.used,cb.total,cb.currency,cb.usage,cb.supported,cb.error,cb.fetched_at from channel_balances cb join channel_api_keys k on k.id=cb.key_id and k.enabled where cb.channel_id=c.id order by k.priority desc nulls last,k.created_at limit 1) cb on true left join lateral (select avg(rl.duration_ms) as avg_duration_ms,avg(rl.first_token_ms) as avg_first_token_ms,count(*) as used_requests,coalesce(sum(rl.total_tokens),0) as used_tokens from request_logs rl where rl.channel_id=c.id) agg on true order by c.priority desc,c.id limit $1 offset $2`, pageSize, offset)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, base string
		var models []byte
		var testModel string
		var enabled, autoDisabled bool
		var disabledReason string
		var priority, weight int
		var lastChecked, lastError any
		var created, updated any
		var groups []string
		var provider, keyType string
		var keyCount int
		var autoDisable bool
		var overrides, uaPool []byte
		var userID, userEmail, userName string
		var avgDuration float64
		var avgFirstTokenMs *float64
		var usedRequests, usedTokens int64
		var balance, usedBalance, totalBalance *float64
		var balanceCurrency, balanceError string
		var usageJSON []byte
		var balanceSupported bool
		var balanceFetched any
		if rows.Scan(&id, &name, &base, &models, &testModel, &enabled, &autoDisabled, &disabledReason, &priority, &weight, &lastChecked, &lastError, &created, &updated, &groups, &provider, &keyType, &keyCount, &autoDisable, &overrides, &uaPool, &userID, &userEmail, &userName, &avgDuration, &avgFirstTokenMs, &usedRequests, &usedTokens, &balance, &usedBalance, &totalBalance, &balanceCurrency, &usageJSON, &balanceSupported, &balanceError, &balanceFetched) != nil {
			continue
		}
		var list []string
		json.Unmarshal(models, &list)
		var ov map[string]any
		if len(overrides) > 0 {
			json.Unmarshal(overrides, &ov)
		}
		if ov == nil {
			ov = map[string]any{}
		}
		var uaList []string
		if len(uaPool) > 0 {
			json.Unmarshal(uaPool, &uaList)
		}
		usageWindows := decodeUpstreamUsageWindows(usageJSON)
		routes := s.getChannelRoutes(r.Context(), id)
		data = append(data, map[string]any{"id": id, "name": name, "base_url": base, "models": list, "test_model": testModel, "provider": provider, "key_type": keyType, "enabled": enabled, "auto_disabled": autoDisabled, "disabled_reason": disabledReason, "priority": priority, "weight": weight, "last_test_time": lastChecked, "last_error": lastError, "response_time_ms": avgDuration, "avg_first_token_ms": avgFirstTokenMs, "used_requests": usedRequests, "used_tokens": usedTokens, "upstream_balance": balance, "upstream_used": usedBalance, "upstream_total": totalBalance, "upstream_currency": balanceCurrency, "upstream_usage_windows": usageWindows, "upstream_balance_supported": balanceSupported, "upstream_balance_error": balanceError, "upstream_balance_fetched_at": balanceFetched, "groups": groups, "key_count": keyCount, "created_at": created, "updated_at": updated, "model_routes": routes, "auto_disable": autoDisable, "request_overrides": ov, "ua_pool": uaList, "user_id": userID, "user_email": userEmail, "user_name": userName})
	}
	writePaged(w, data, total, page, pageSize)
}

func (s *Service) getChannelRoutes(ctx context.Context, channelID string) []map[string]any {
	rows, err := s.db.Query(ctx, `select id,public_model,upstream_model,priority,weight,enabled,hidden,created_at from model_routes where channel_id=$1 order by public_model,priority desc`, channelID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var routes []map[string]any
	for rows.Next() {
		var id, public, upstream string
		var priority, weight int
		var enabled, hidden bool
		var created any
		if rows.Scan(&id, &public, &upstream, &priority, &weight, &enabled, &hidden, &created) == nil {
			routes = append(routes, map[string]any{"id": id, "public_model": public, "upstream_model": upstream, "priority": priority, "weight": weight, "enabled": enabled, "hidden": hidden, "created_at": created})
		}
	}
	return routes
}
func (s *Service) batchSetChannelStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs     []string `json:"ids"`
		Enabled bool     `json:"enabled"`
	}
	if decode(r, &in) != nil || len(in.IDs) == 0 {
		writeError(w, 400, "invalid_request", "ids and enabled are required")
		return
	}
	if len(in.IDs) > 100 {
		writeError(w, 400, "invalid_request", "at most 100 channels at a time")
		return
	}
	result, err := s.db.Exec(r.Context(), `update channels set enabled=$1,auto_disabled=case when $1 then false else auto_disabled end,disabled_reason=case when $1 then '' else disabled_reason end,failure_count=case when $1 then 0 else failure_count end,cooldown_until=case when $1 then null else cooldown_until end,updated_at=now() where id = any($2)`, in.Enabled, in.IDs)
	if err != nil {
		writeError(w, 500, "internal_error", "could not update channels")
		return
	}
	affected := result.RowsAffected()
	s.audit(r, "channels.batch_status_changed", "channel", "batch", map[string]any{"count": affected, "enabled": in.Enabled})
	s.invalidateChannels()
	writeJSON(w, 200, map[string]any{"affected": affected})
}

func (s *Service) setChannelStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "enabled is required")
		return
	}
	result, err := s.db.Exec(r.Context(), `update channels set enabled=$1,auto_disabled=case when $1 then false else auto_disabled end,disabled_reason=case when $1 then '' else disabled_reason end,failure_count=case when $1 then 0 else failure_count end,cooldown_until=case when $1 then null else cooldown_until end, updated_at=now() where id=$2`, in.Enabled, r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel not found")
		return
	}
	s.audit(r, "channel.status_changed", "channel", r.PathValue("id"), map[string]any{"enabled": in.Enabled})
	s.invalidateChannels()
	writeJSON(w, 200, map[string]bool{"enabled": in.Enabled})
}

// syncChannelKeyType keeps channels.key_type consistent with the number of enabled keys.
func (s *Service) syncChannelKeyType(ctx context.Context, channelID string) {
	_, _ = s.db.Exec(ctx, `update channels set key_type=case when (select count(*) from channel_api_keys ak where ak.channel_id=channels.id and ak.enabled)>1 then 'multi' else 'single' end,updated_at=now() where id=$1`, channelID)
	s.invalidateChannels()
}

func (s *Service) listChannelKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select k.id,k.name,k.enabled,k.priority,k.last_checked_at,k.last_error,k.created_at,cb.balance,cb.used,cb.total,coalesce(cb.currency,'USD'),cb.usage,coalesce(cb.supported,false),coalesce(cb.error,''),cb.fetched_at from channel_api_keys k left join channel_balances cb on cb.channel_id=k.channel_id and cb.key_id=k.id where k.channel_id=$1 order by k.priority desc,k.created_at`, r.PathValue("id"))
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name string
		var enabled bool
		var priority int
		var lastChecked, lastError any
		var created any
		var balance, usedBalance, totalBalance *float64
		var balanceCurrency, balanceError string
		var usageJSON []byte
		var balanceSupported bool
		var balanceFetched any
		if rows.Scan(&id, &name, &enabled, &priority, &lastChecked, &lastError, &created, &balance, &usedBalance, &totalBalance, &balanceCurrency, &usageJSON, &balanceSupported, &balanceError, &balanceFetched) == nil {
			usageWindows := decodeUpstreamUsageWindows(usageJSON)
			data = append(data, map[string]any{"id": id, "name": name, "enabled": enabled, "priority": priority, "last_checked_at": lastChecked, "last_error": lastError, "created_at": created, "upstream_balance": balance, "upstream_used": usedBalance, "upstream_total": totalBalance, "upstream_currency": balanceCurrency, "upstream_usage_windows": usageWindows, "upstream_balance_supported": balanceSupported, "upstream_balance_error": balanceError, "upstream_balance_fetched_at": balanceFetched})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) createChannelKey(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		APIKey   string `json:"api_key"`
		Priority *int   `json:"priority"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "api_key is required")
		return
	}
	in.APIKey = strings.TrimSpace(in.APIKey)
	if !validChannelAPIKey(in.APIKey) {
		writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		in.Name = "key"
	} else if !validChannelName(in.Name) {
		writeError(w, 400, "invalid_request", "name must be 1-100 characters")
		return
	}
	priority := 100
	if in.Priority != nil {
		if !validChannelPriority(*in.Priority) {
			writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
			return
		}
		priority = *in.Priority
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into channel_api_keys(id,channel_id,name,key_encrypted,enabled,priority) values($1,$2,$3,$4,true,$5)`, id, r.PathValue("id"), in.Name, in.APIKey, priority)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not create api key")
		return
	}
	s.audit(r, "channel_key.created", "channel_api_key", id, map[string]any{"channel_id": r.PathValue("id"), "name": in.Name, "priority": priority})
	s.syncChannelKeyType(r.Context(), r.PathValue("id"))
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "enabled": true, "priority": priority})
}

func (s *Service) deleteChannelKey(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.Exec(r.Context(), `delete from channel_api_keys where id=$1 and channel_id=$2`, r.PathValue("keyId"), r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel API key not found")
		return
	}
	s.audit(r, "channel_key.deleted", "channel_api_key", r.PathValue("keyId"), map[string]any{"channel_id": r.PathValue("id")})
	s.syncChannelKeyType(r.Context(), r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) setChannelKeyStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "enabled is required")
		return
	}
	result, err := s.db.Exec(r.Context(), `update channel_api_keys set enabled=$1 where id=$2 and channel_id=$3`, in.Enabled, r.PathValue("keyId"), r.PathValue("id"))
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel API key not found")
		return
	}
	s.audit(r, "channel_key.status_changed", "channel_api_key", r.PathValue("keyId"), map[string]any{"enabled": in.Enabled})
	s.syncChannelKeyType(r.Context(), r.PathValue("id"))
	writeJSON(w, 200, map[string]bool{"enabled": in.Enabled})
}

func (s *Service) updateChannelKey(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     *string `json:"name"`
		APIKey   *string `json:"api_key"`
		Priority *int    `json:"priority"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid request")
		return
	}
	var name string
	var priority int
	if err := s.db.QueryRow(r.Context(), `select name,priority from channel_api_keys where id=$1 and channel_id=$2`, r.PathValue("keyId"), r.PathValue("id")).Scan(&name, &priority); err != nil {
		writeError(w, 404, "not_found", "channel API key not found")
		return
	}
	var apiKey string
	if in.APIKey != nil {
		apiKey = strings.TrimSpace(*in.APIKey)
		if !validChannelAPIKey(apiKey) {
			writeError(w, 400, "invalid_request", "api_key must be 1-4096 characters")
			return
		}
	}
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if !validChannelName(name) {
			writeError(w, 400, "invalid_request", "name must be 1-100 characters")
			return
		}
	}
	if in.Priority != nil {
		if !validChannelPriority(*in.Priority) {
			writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
			return
		}
		priority = *in.Priority
	}
	query := `update channel_api_keys set name=$1,priority=$2 where id=$3 and channel_id=$4`
	args := []any{name, priority, r.PathValue("keyId"), r.PathValue("id")}
	if in.APIKey != nil {
		query = `update channel_api_keys set name=$1,priority=$2,key_encrypted=$5,failure_count=0,last_error=null,last_checked_at=null where id=$3 and channel_id=$4`
		args = append(args, apiKey)
	}
	result, err := s.db.Exec(r.Context(), query, args...)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel API key not found")
		return
	}
	s.audit(r, "channel_key.updated", "channel_api_key", r.PathValue("keyId"), map[string]any{"channel_id": r.PathValue("id"), "name": name, "priority": priority, "secret_changed": in.APIKey != nil})
	s.invalidateChannels()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) revealChannelKey(w http.ResponseWriter, r *http.Request) {
	var encrypted string
	if err := s.db.QueryRow(r.Context(), `select key_encrypted from channel_api_keys where id=$1 and channel_id=$2`, r.PathValue("keyId"), r.PathValue("id")).Scan(&encrypted); err != nil {
		writeError(w, 404, "not_found", "channel API key not found")
		return
	}
	key, err := channelKeyValue(s.cfg.EncryptionKey, encrypted)
	if err != nil {
		writeError(w, 500, "internal_error", "could not read channel API key")
		return
	}
	s.audit(r, "channel_key.revealed", "channel_api_key", r.PathValue("keyId"), map[string]any{"channel_id": r.PathValue("id")})
	writeJSON(w, 200, map[string]any{"key": key})
}

func (s *Service) testChannelKey(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	keyID := r.PathValue("keyId")

	var baseURL, provider, testModel string
	var uaPool []byte
	var autoDisable bool
	if err := s.db.QueryRow(r.Context(), `select base_url,provider,test_model,ua_pool,auto_disable from channels where id=$1`, channelID).Scan(&baseURL, &provider, &testModel, &uaPool, &autoDisable); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}

	var encrypted string
	if err := s.db.QueryRow(r.Context(), `select key_encrypted from channel_api_keys where id=$1 and channel_id=$2`, keyID, channelID).Scan(&encrypted); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "channel API key not found")
		return
	}

	apiKey, err := channelKeyValue(s.cfg.EncryptionKey, encrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read key")
		return
	}

	settings := s.reliabilitySettings(r.Context())
	status, body, latency, testErr := s.testChannel(r.Context(), baseURL, apiKey, provider, testModel, parsedUAPool(uaPool))
	success := testErr == nil && status >= 200 && status < 300
	reason := healthFailureReason(status, testErr)

	if success && settings.AutoDisableSlowSeconds > 0 && latency > time.Duration(settings.AutoDisableSlowSeconds)*time.Second {
		success = false
		reason = "health_check_slow_response"
	}
	if !success && settings.autoDisableKeyword(string(body)) {
		reason = "health_check_keyword_match"
	}

	result := map[string]any{
		"success":     success,
		"status_code": status,
		"latency_ms":  latency.Milliseconds(),
	}

	if success {
		_, _ = s.db.Exec(r.Context(), `update channel_api_keys set enabled=true,failure_count=0,last_error=null,last_checked_at=now() where id=$1 and channel_id=$2`, keyID, channelID)
		var remaining int
		if err := s.db.QueryRow(r.Context(), `select count(*) from channel_api_keys where channel_id=$1 and enabled`, channelID).Scan(&remaining); err == nil && remaining <= 1 {
			_, _ = s.db.Exec(r.Context(), `update channels set enabled=true,auto_disabled=false,disabled_reason='',failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1`, channelID)
		} else {
			_, _ = s.db.Exec(r.Context(), `update channels set last_checked_at=now(),last_error=null,updated_at=now() where id=$1`, channelID)
		}
		s.syncChannelKeyType(r.Context(), channelID)
		result["auto_disabled"] = false
		writeJSON(w, http.StatusOK, result)
		return
	}

	result["reason"] = reason
	result["auto_disabled"] = false
	_, _ = s.db.Exec(r.Context(), `update channel_api_keys set last_checked_at=now(),last_error=$1 where id=$2 and channel_id=$3`, reason, keyID, channelID)
	_, _ = s.db.Exec(r.Context(), `update channels set last_checked_at=now(),last_error=$1,updated_at=now() where id=$2`, reason, channelID)
	if autoDisable {
		_, _ = s.db.Exec(r.Context(), `update channel_api_keys set enabled=false where id=$1 and channel_id=$2`, keyID, channelID)
		s.syncChannelKeyType(r.Context(), channelID)
		result["auto_disabled"] = true
		s.audit(r, "channel_key.auto_disabled", "channel_api_key", keyID, map[string]any{"channel_id": channelID, "reason": reason})
		var remaining int
		if err := s.db.QueryRow(r.Context(), `select count(*) from channel_api_keys where channel_id=$1 and enabled`, channelID).Scan(&remaining); err == nil && remaining == 0 {
			_, _ = s.db.Exec(r.Context(), `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_error=$1,last_checked_at=now(),updated_at=now() where id=$2`, reason, channelID)
			s.audit(r, "channel.auto_disabled", "channel", channelID, map[string]any{"key_id": keyID, "reason": reason})
			result["channel_disabled"] = true
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) testChannelHandler(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")

	var baseURL, provider, legacyKey, testModel string
	var uaPool []byte
	var autoDisable bool
	if err := s.db.QueryRow(r.Context(), `select base_url,provider,api_key,test_model,ua_pool,auto_disable from channels where id=$1`, channelID).Scan(&baseURL, &provider, &legacyKey, &testModel, &uaPool, &autoDisable); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}

	type keyRow struct {
		id, encrypted string
	}
	var keys []keyRow
	rows, err := s.db.Query(r.Context(), `select id,key_encrypted from channel_api_keys where channel_id=$1 and enabled order by priority desc nulls last, created_at`, channelID)
	if err == nil {
		for rows.Next() {
			var k keyRow
			if rows.Scan(&k.id, &k.encrypted) == nil {
				keys = append(keys, k)
			}
		}
		rows.Close()
	}
	if len(keys) == 0 {
		if legacyKey == "" {
			writeError(w, http.StatusBadRequest, "no_key", "channel has no API key to test")
			return
		}
		keys = []keyRow{{id: "", encrypted: legacyKey}}
	}

	settings := s.reliabilitySettings(r.Context())
	type keyResult struct {
		ID           string `json:"key_id"`
		Success      bool   `json:"success"`
		StatusCode   int    `json:"status_code"`
		LatencyMs    int64  `json:"latency_ms"`
		Reason       string `json:"reason,omitempty"`
		AutoDisabled bool   `json:"auto_disabled"`
	}

	keyResults := make([]keyResult, 0, len(keys))
	anySuccess := false
	var successStatus, lastStatus int
	var successLatency, lastLatency time.Duration
	channelReason := ""

	for _, k := range keys {
		apiKey, kerr := channelKeyValue(s.cfg.EncryptionKey, k.encrypted)
		if kerr != nil {
			keyResults = append(keyResults, keyResult{ID: k.id, Reason: "could not read key"})
			continue
		}

		status, body, latency, testErr := s.testChannel(r.Context(), baseURL, apiKey, provider, testModel, parsedUAPool(uaPool))
		success := testErr == nil && status >= 200 && status < 300
		reason := healthFailureReason(status, testErr)
		if success && settings.AutoDisableSlowSeconds > 0 && latency > time.Duration(settings.AutoDisableSlowSeconds)*time.Second {
			success = false
			reason = "health_check_slow_response"
		}
		if !success && settings.autoDisableKeyword(string(body)) {
			reason = "health_check_keyword_match"
		}

		lastStatus = status
		lastLatency = latency
		kr := keyResult{ID: k.id, Success: success, StatusCode: status, LatencyMs: latency.Milliseconds()}
		if success {
			if !anySuccess {
				successStatus = status
				successLatency = latency
			}
			if k.id != "" {
				_, _ = s.db.Exec(r.Context(), `update channel_api_keys set last_checked_at=now(),last_error=null where id=$1 and channel_id=$2`, k.id, channelID)
			}
			kr.AutoDisabled = false
			keyResults = append(keyResults, kr)
			anySuccess = true
			continue
		}

		kr.Reason = reason
		if k.id != "" {
			_, _ = s.db.Exec(r.Context(), `update channel_api_keys set last_checked_at=now(),last_error=$1 where id=$2 and channel_id=$3`, reason, k.id, channelID)
			if autoDisable {
				_, _ = s.db.Exec(r.Context(), `update channel_api_keys set enabled=false where id=$1 and channel_id=$2`, k.id, channelID)
				s.audit(r, "channel_key.auto_disabled", "channel_api_key", k.id, map[string]any{"channel_id": channelID, "reason": reason})
				kr.AutoDisabled = true
			}
		}
		keyResults = append(keyResults, kr)
		channelReason = reason
	}

	channelDisabled := false
	if anySuccess {
		_, _ = s.db.Exec(r.Context(), `update channels set enabled=true,auto_disabled=false,disabled_reason='',failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1`, channelID)
		s.syncChannelKeyType(r.Context(), channelID)
	} else if autoDisable {
		_, _ = s.db.Exec(r.Context(), `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_checked_at=now(),last_error=$1,updated_at=now() where id=$2`, channelReason, channelID)
		s.syncChannelKeyType(r.Context(), channelID)
		s.audit(r, "channel.auto_disabled", "channel", channelID, map[string]any{"reason": channelReason})
		channelDisabled = true
	}

	result := map[string]any{
		"success":          anySuccess,
		"status_code":      0,
		"latency_ms":       0,
		"channel_disabled": channelDisabled,
		"reason":           channelReason,
		"keys":             keyResults,
	}
	if anySuccess {
		result["status_code"] = successStatus
		result["latency_ms"] = successLatency.Milliseconds()
	} else {
		result["status_code"] = lastStatus
		result["latency_ms"] = lastLatency.Milliseconds()
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) migrateChannelKeys(w http.ResponseWriter, r *http.Request) {
	tag, err := s.db.Exec(r.Context(), `insert into channel_api_keys(id,channel_id,name,key_encrypted,enabled)
	select gen_random_uuid(), c.id, 'default', c.api_key, true
	from channels c
	where c.id=$1 and c.api_key != ''
	and not exists (select 1 from channel_api_keys ak where ak.channel_id=c.id)
	on conflict do nothing`, r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, 400, "invalid_request", "migrate failed or channel already has keys")
		return
	}
	s.audit(r, "channel.keys_migrated", "channel", r.PathValue("id"), nil)
	s.invalidateChannels()
	writeJSON(w, 200, map[string]any{"migrated": true})
}

func (s *Service) adjustBalance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "user_id, non-zero finite amount, and note are required")
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if in.UserID == "" || !validWalletAdjustAmount(in.Amount) || in.Note == "" || len(in.Note) > maxWalletNoteLength {
		writeError(w, 400, "invalid_request", "user_id, non-zero finite amount within ±1e9, and note (1-500 chars) are required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict(user_id) do nothing`, in.UserID); err != nil {
		writeError(w, 500, "internal_error", "could not load wallet")
		return
	}
	var balance float64
	if err = tx.QueryRow(r.Context(), `select balance from user_wallets where user_id=$1 for update`, in.UserID).Scan(&balance); err != nil || balance+in.Amount < 0 {
		writeError(w, 400, "invalid_request", "unknown user or insufficient balance")
		return
	}
	if _, err = tx.Exec(r.Context(), `update user_wallets set balance=balance+$1,updated_at=now() where user_id=$2`, in.Amount, in.UserID); err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	id, _ := randomID()
	kind := "adjustment"
	if in.Amount > 0 {
		kind = "topup"
	}
	if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,note) values($1,$2,$3,$4,$5,$6)`, id, in.UserID, in.Amount, balance+in.Amount, kind, in.Note); err != nil {
		writeError(w, 500, "internal_error", "could not record adjustment")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	s.audit(r, "wallet.adjusted", "user", in.UserID, map[string]any{"amount": in.Amount, "note": in.Note})
	writeJSON(w, 200, map[string]any{"balance": balance + in.Amount})
}

func (s *Service) listModelRoutes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,public_model,upstream_model,channel_id,priority,weight,enabled,hidden,created_at from model_routes order by public_model,priority desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, public, upstream, channelID string
		var priority, weight int
		var enabled, hidden bool
		var created any
		if rows.Scan(&id, &public, &upstream, &channelID, &priority, &weight, &enabled, &hidden, &created) == nil {
			data = append(data, map[string]any{"id": id, "public_model": public, "upstream_model": upstream, "channel_id": channelID, "priority": priority, "weight": weight, "enabled": enabled, "hidden": hidden, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) createModelRoute(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PublicModel   string `json:"public_model"`
		UpstreamModel string `json:"upstream_model"`
		ChannelID     string `json:"channel_id"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Hidden        bool   `json:"hidden"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "public_model, upstream_model, and channel_id are required")
		return
	}
	in.PublicModel = strings.TrimSpace(in.PublicModel)
	in.UpstreamModel = strings.TrimSpace(in.UpstreamModel)
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	if !validModelName(in.PublicModel) || !validModelName(in.UpstreamModel) || in.ChannelID == "" {
		writeError(w, 400, "invalid_request", "public_model and upstream_model must be 1-200 characters; channel_id is required")
		return
	}
	if in.Weight < 0 || in.Weight > 10000 {
		writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
		return
	}
	if in.Weight == 0 {
		in.Weight = 100
	}
	if in.Priority < -10000 || in.Priority > 10000 {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight,hidden) values($1,$2,$3,$4,$5,$6,$7)`, id, in.PublicModel, in.UpstreamModel, in.ChannelID, in.Priority, in.Weight, in.Hidden)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not create model route")
		return
	}
	s.audit(r, "model_route.created", "model_route", id, map[string]any{"public_model": in.PublicModel, "channel_id": in.ChannelID, "hidden": in.Hidden})
	s.invalidateChannels()
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Service) updateModelRoute(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PublicModel   *string `json:"public_model"`
		UpstreamModel *string `json:"upstream_model"`
		ChannelID     *string `json:"channel_id"`
		Priority      *int    `json:"priority"`
		Weight        *int    `json:"weight"`
		Enabled       *bool   `json:"enabled"`
		Hidden        *bool   `json:"hidden"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid model route update")
		return
	}
	routeID := r.PathValue("id")
	var channelID string
	if err := s.db.QueryRow(r.Context(), `select channel_id from model_routes where id=$1`, routeID).Scan(&channelID); err != nil {
		writeError(w, 404, "not_found", "model route not found")
		return
	}
	if in.ChannelID != nil {
		writeError(w, 400, "invalid_request", "channel_id cannot be changed on an existing route; delete and recreate it to move it to another channel")
		return
	}
	if in.PublicModel != nil {
		*in.PublicModel = strings.TrimSpace(*in.PublicModel)
		if !validModelName(*in.PublicModel) {
			writeError(w, 400, "invalid_request", "public_model must be 1-200 characters")
			return
		}
	}
	if in.UpstreamModel != nil {
		*in.UpstreamModel = strings.TrimSpace(*in.UpstreamModel)
		if !validModelName(*in.UpstreamModel) {
			writeError(w, 400, "invalid_request", "upstream_model must be 1-200 characters")
			return
		}
	}
	if in.Weight != nil && (*in.Weight < 0 || *in.Weight > 10000) {
		writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
		return
	}
	if in.Priority != nil && (*in.Priority < -10000 || *in.Priority > 10000) {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	changed := map[string]any{}
	if in.PublicModel != nil {
		changed["public_model"] = *in.PublicModel
	}
	if in.UpstreamModel != nil {
		changed["upstream_model"] = *in.UpstreamModel
	}
	if in.Priority != nil {
		changed["priority"] = *in.Priority
	}
	if in.Weight != nil {
		changed["weight"] = *in.Weight
	}
	if in.Enabled != nil {
		changed["enabled"] = *in.Enabled
	}
	if in.Hidden != nil {
		changed["hidden"] = *in.Hidden
	}
	if len(changed) == 0 {
		writeError(w, 400, "invalid_request", "no fields to update")
		return
	}
	result, err := s.db.Exec(r.Context(), `update model_routes set public_model=coalesce($1,public_model),upstream_model=coalesce($2,upstream_model),priority=coalesce($3,priority),weight=coalesce($4,weight),enabled=coalesce($5,enabled),hidden=coalesce($6,hidden) where id=$7`, in.PublicModel, in.UpstreamModel, in.Priority, in.Weight, in.Enabled, in.Hidden, routeID)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "model route not found or could not be updated")
		return
	}
	s.audit(r, "model_route.updated", "model_route", routeID, changed)
	s.invalidateChannels()
	writeJSON(w, 200, map[string]any{"id": routeID, "updated": changed})
}

func (s *Service) listChannelRoutes(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	rows, err := s.db.Query(r.Context(), `select id,public_model,upstream_model,priority,weight,enabled,hidden,created_at from model_routes where channel_id=$1 order by public_model,priority desc`, channelID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, public, upstream string
		var priority, weight int
		var enabled, hidden bool
		var created any
		if rows.Scan(&id, &public, &upstream, &priority, &weight, &enabled, &hidden, &created) == nil {
			data = append(data, map[string]any{"id": id, "public_model": public, "upstream_model": upstream, "priority": priority, "weight": weight, "enabled": enabled, "hidden": hidden, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) createChannelRoute(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	var in struct {
		PublicModel   string `json:"public_model"`
		UpstreamModel string `json:"upstream_model"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Hidden        bool   `json:"hidden"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "public_model, upstream_model are required")
		return
	}
	in.PublicModel = strings.TrimSpace(in.PublicModel)
	in.UpstreamModel = strings.TrimSpace(in.UpstreamModel)
	if !validModelName(in.PublicModel) || !validModelName(in.UpstreamModel) {
		writeError(w, 400, "invalid_request", "public_model and upstream_model must be 1-200 characters")
		return
	}
	if in.Weight < 0 || in.Weight > 10000 {
		writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
		return
	}
	if in.Weight == 0 {
		in.Weight = 100
	}
	if in.Priority < -10000 || in.Priority > 10000 {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	routeID, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight,hidden) values($1,$2,$3,$4,$5,$6,$7)`, routeID, in.PublicModel, in.UpstreamModel, channelID, in.Priority, in.Weight, in.Hidden)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not create model route")
		return
	}
	s.audit(r, "model_route.created", "model_route", routeID, map[string]any{"public_model": in.PublicModel, "channel_id": channelID, "hidden": in.Hidden})
	s.invalidateChannels()
	writeJSON(w, 201, map[string]any{"id": routeID})
}

func (s *Service) updateChannelRoute(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	routeID := r.PathValue("routeId")
	var rowChannelID string
	if err := s.db.QueryRow(r.Context(), `select channel_id from model_routes where id=$1`, routeID).Scan(&rowChannelID); err != nil || rowChannelID != channelID {
		writeError(w, 404, "not_found", "model route not found")
		return
	}
	var in struct {
		PublicModel   *string `json:"public_model"`
		UpstreamModel *string `json:"upstream_model"`
		Priority      *int    `json:"priority"`
		Weight        *int    `json:"weight"`
		Enabled       *bool   `json:"enabled"`
		Hidden        *bool   `json:"hidden"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid model route update")
		return
	}
	if in.PublicModel != nil {
		*in.PublicModel = strings.TrimSpace(*in.PublicModel)
		if !validModelName(*in.PublicModel) {
			writeError(w, 400, "invalid_request", "public_model must be 1-200 characters")
			return
		}
	}
	if in.UpstreamModel != nil {
		*in.UpstreamModel = strings.TrimSpace(*in.UpstreamModel)
		if !validModelName(*in.UpstreamModel) {
			writeError(w, 400, "invalid_request", "upstream_model must be 1-200 characters")
			return
		}
	}
	if in.Weight != nil && (*in.Weight < 0 || *in.Weight > 10000) {
		writeError(w, 400, "invalid_request", "weight must be between 0 and 10000")
		return
	}
	if in.Priority != nil && (*in.Priority < -10000 || *in.Priority > 10000) {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	result, err := s.db.Exec(r.Context(), `update model_routes set public_model=coalesce($1,public_model),upstream_model=coalesce($2,upstream_model),priority=coalesce($3,priority),weight=coalesce($4,weight),enabled=coalesce($5,enabled),hidden=coalesce($6,hidden) where id=$7`, in.PublicModel, in.UpstreamModel, in.Priority, in.Weight, in.Enabled, in.Hidden, routeID)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "model route not found or could not be updated")
		return
	}
	changed := map[string]any{}
	if in.PublicModel != nil {
		changed["public_model"] = *in.PublicModel
	}
	if in.UpstreamModel != nil {
		changed["upstream_model"] = *in.UpstreamModel
	}
	if in.Priority != nil {
		changed["priority"] = *in.Priority
	}
	if in.Weight != nil {
		changed["weight"] = *in.Weight
	}
	if in.Enabled != nil {
		changed["enabled"] = *in.Enabled
	}
	if in.Hidden != nil {
		changed["hidden"] = *in.Hidden
	}
	s.audit(r, "model_route.updated", "model_route", routeID, changed)
	s.invalidateChannels()
	writeJSON(w, 200, map[string]any{"id": routeID, "updated": changed})
}

func (s *Service) deleteChannelRoute(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("id")
	routeID := r.PathValue("routeId")
	result, err := s.db.Exec(r.Context(), `delete from model_routes where id=$1 and channel_id=$2`, routeID, channelID)
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete model route")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "model route not found")
		return
	}
	s.audit(r, "model_route.deleted", "model_route", routeID, map[string]any{"channel_id": channelID})
	s.invalidateChannels()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) upsertQuota(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID      string   `json:"user_id"`
		APIKeyID    string   `json:"api_key_id"`
		Model       string   `json:"model"`
		Window      string   `json:"window"`
		MaxRequests *int64   `json:"max_requests"`
		MaxTokens   *int64   `json:"max_tokens"`
		MaxCost     *float64 `json:"max_cost"`
	}
	if decode(r, &in) != nil || (in.UserID == "" && in.APIKeyID == "") || (in.Window != "minute" && in.Window != "day" && in.Window != "month" && in.Window != "total") || (in.MaxRequests == nil && in.MaxTokens == nil && in.MaxCost == nil) {
		writeError(w, 400, "invalid_request", "scope, window, and a limit are required")
		return
	}
	in.Model = strings.TrimSpace(in.Model)
	if in.Model != "" && !validModelName(in.Model) {
		writeError(w, 400, "invalid_request", "model must be 1-200 characters when set")
		return
	}
	if !validQuotaLimit(in.MaxRequests) || !validQuotaLimit(in.MaxTokens) || !validQuotaCost(in.MaxCost) {
		writeError(w, 400, "invalid_request", "limits must be between 0 and 1e12")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens,max_cost) values($1,nullif($2,'')::bigint,nullif($3,'')::uuid,nullif($4,''),$5,$6,$7,$8) on conflict (coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window") do update set max_requests=excluded.max_requests,max_tokens=excluded.max_tokens,max_cost=excluded.max_cost`, id, in.UserID, in.APIKeyID, in.Model, in.Window, in.MaxRequests, in.MaxTokens, in.MaxCost)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save quota")
		return
	}
	s.audit(r, "quota.updated", "quota", id, map[string]any{"user_id": in.UserID, "api_key_id": in.APIKeyID, "model": in.Model, "window": in.Window})
	s.invalidateQuotaAbsence()
	writeJSON(w, 200, map[string]any{"id": id})
}

// listQuotaLimits returns all quota_limits rows, optionally filtered by
// user_id and/or api_key_id query parameters.
func (s *Service) listQuotaLimits(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	apiKeyID := strings.TrimSpace(r.URL.Query().Get("api_key_id"))
	q := `select id,coalesce(user_id::text,''),coalesce(api_key_id::text,''),coalesce(model,''),"window",max_requests,max_tokens,max_cost,created_at from quota_limits`
	var args []any
	var clauses []string
	if userID != "" {
		args = append(args, userID)
		clauses = append(clauses, "user_id=$"+strconv.Itoa(len(args)))
	}
	if apiKeyID != "" {
		args = append(args, apiKeyID)
		clauses = append(clauses, "api_key_id=$"+strconv.Itoa(len(args)))
	}
	if len(clauses) > 0 {
		q += " where " + strings.Join(clauses, " and ")
	}
	q += ` order by created_at desc`
	rows, err := s.db.Query(r.Context(), q, args...)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, uid, keyID, model, window string
		var maxRequests, maxTokens *int64
		var maxCost *float64
		var created any
		if rows.Scan(&id, &uid, &keyID, &model, &window, &maxRequests, &maxTokens, &maxCost, &created) == nil {
			data = append(data, map[string]any{"id": id, "user_id": uid, "api_key_id": keyID, "model": model, "window": window, "max_requests": maxRequests, "max_tokens": maxTokens, "max_cost": maxCost, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

// deleteQuotaLimit removes a single quota_limits row by id.
func (s *Service) deleteQuotaLimit(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, 400, "invalid_request", "quota id is required")
		return
	}
	result, err := s.db.Exec(r.Context(), `delete from quota_limits where id=$1`, id)
	if err != nil {
		writeError(w, 500, "internal_error", "could not delete quota")
		return
	}
	if result.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "quota not found")
		return
	}
	s.audit(r, "quota.deleted", "quota", id, nil)
	s.invalidateQuotaAbsence()
	w.WriteHeader(http.StatusNoContent)
}
func (s *Service) listLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select rl.request_id,coalesce(rl.user_id::text,''),coalesce(u.name,'') as user_name,coalesce(rl.api_key_id::text,''),coalesce(ak.name,'') as key_name,coalesce(rl.channel_id::text,''),coalesce(c.name,'') as channel_name,coalesce(rl.channel_key_id::text,''),coalesce(ck.name,'') as channel_key_name,coalesce(rl.group_id::text,''),coalesce(g.name,'') as group_name,rl.model,rl.status_code,coalesce(rl.prompt_tokens,0),coalesce(rl.completion_tokens,0),coalesce(rl.total_tokens,0),rl.duration_ms,rl.first_token_ms,coalesce(rl.error_code,''),case when rl.error_code is not null or rl.status_code>=400 then rl.error_detail else '' end,rl.client_ip,rl.user_agent,rl.created_at from request_logs rl left join users u on u.id=rl.user_id left join api_keys ak on ak.id=rl.api_key_id left join channels c on c.id=rl.channel_id left join channel_api_keys ck on ck.id=rl.channel_key_id left join groups g on g.id=rl.group_id where coalesce(rl.error_code,'') not in ('user_concurrency_limit','group_concurrency_limit') order by rl.created_at desc limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	// Log views apply the same keyword rewrite as the gateway so an upstream's
	// specific account/quota error text never shows up here either.
	reliability := s.reliabilitySettings(r.Context())
	data := []map[string]any{}
	for rows.Next() {
		var requestID, userID, userName, apiKeyID, keyName, channelID, channelName, channelKeyID, channelKeyName, groupID, groupName, model, errorCode, errorDetail, clientIP, userAgent string
		var prompt, completion, total any
		var status, duration int
		var firstTokenMs *int
		var created any
		if err := rows.Scan(&requestID, &userID, &userName, &apiKeyID, &keyName, &channelID, &channelName, &channelKeyID, &channelKeyName, &groupID, &groupName, &model, &status, &prompt, &completion, &total, &duration, &firstTokenMs, &errorCode, &errorDetail, &clientIP, &userAgent, &created); err != nil {
			continue
		}
		errorDetail = s.clientUpstreamError(r.Context(), errorDetail, reliability)
		data = append(data, map[string]any{"request_id": requestID, "user_id": userID, "user_name": userName, "api_key_id": apiKeyID, "key_name": keyName, "channel_id": channelID, "channel_name": channelName, "channel_key_id": channelKeyID, "channel_key_name": channelKeyName, "group_id": groupID, "group_name": groupName, "model": model, "status_code": status, "prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total, "duration_ms": duration, "first_token_ms": firstTokenMs, "error_code": errorCode, "error_detail": errorDetail, "client_ip": clientIP, "user_agent": userAgent, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) runMigration(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SourceDSN    string `json:"source_dsn"`
		SourceDriver string `json:"source_driver"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON body")
		return
	}
	if in.SourceDSN == "" {
		writeError(w, 400, "invalid_request", "source_dsn is required")
		return
	}
	if in.SourceDriver == "" {
		in.SourceDriver = "mysql"
	}
	if err := validateSourceDSN(in.SourceDSN); err != nil {
		writeError(w, 400, "invalid_source_dsn", err.Error())
		return
	}
	in.SourceDSN = strings.TrimPrefix(in.SourceDSN, "mysql://")

	log.Printf("Migration requested: driver=%s source=%s target=%s", in.SourceDriver, redactDSN(in.SourceDSN), redactDSN(s.cfg.DatabaseURL))

	if !s.startMigration(in.SourceDSN, in.SourceDriver) {
		writeError(w, 409, "migration_already_running", "A migration is already in progress")
		return
	}

	s.audit(r, "system.migrate", "system", "", map[string]any{"source_driver": in.SourceDriver})
	writeJSON(w, 200, map[string]any{"message": "Migration started"})
}
