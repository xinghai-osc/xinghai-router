package app

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
)

func (s *Service) fetchChannelModels(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.BaseURL) == "" || strings.TrimSpace(in.APIKey) == "" {
		writeError(w, 400, "invalid_request", "base_url and api_key are required")
		return
	}
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "https" && !(baseURL.Scheme == "http" && isLoopbackHost(baseURL.Hostname()))) {
		writeError(w, 400, "invalid_request", "base_url must use HTTPS, except for loopback HTTP services")
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
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		writeError(w, 502, "upstream_error", "upstream models request failed")
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
	if decode(r, &in) != nil || in.Model == "" || in.Input < 0 || in.Cached < 0 || in.Output < 0 {
		writeError(w, 400, "invalid_request", "invalid pricing rule")
		return
	}
	if in.Multiplier == 0 {
		in.Multiplier = 1
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier) values($1,$2,$3,$4,$5,$6) on conflict(model) do update set input_per_million=excluded.input_per_million,cached_input_per_million=excluded.cached_input_per_million,output_per_million=excluded.output_per_million,multiplier=excluded.multiplier,updated_at=now()`, id, in.Model, in.Input, in.Cached, in.Output, in.Multiplier)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save pricing rule")
		return
	}
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
	if decode(r, &in) != nil || in.PricePerQuotaUnit < 0 {
		writeError(w, 400, "invalid_request", "invalid NewAPI pricing source")
		return
	}
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "https" && !(baseURL.Scheme == "http" && isLoopbackHost(baseURL.Hostname()))) {
		writeError(w, 400, "invalid_request", "base_url must use HTTPS, except for loopback HTTP services")
		return
	}
	fetch := func(path string, out any) error {
		target := *baseURL
		target.Path = strings.TrimRight(target.Path, "/") + path
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
		if err != nil {
			return err
		}
		if strings.TrimSpace(in.APIKey) != "" {
			request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(in.APIKey))
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
	if err := fetch("/api/status", &status); err != nil || !status.Success || status.Data.QuotaPerUnit <= 0 {
		writeError(w, 502, "upstream_error", "could not read NewAPI quota configuration")
		return
	}
	if err := fetch("/api/pricing", &pricing); err != nil || !pricing.Success {
		writeError(w, 502, "upstream_error", "could not read NewAPI pricing")
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
		if strings.TrimSpace(item.ModelName) == "" || item.QuotaType != 0 || item.ModelRatio < 0 || item.CompletionRatio < 0 {
			continue
		}
		input := newAPIPricePerMillion(item.ModelRatio, in.PricePerQuotaUnit, status.Data.QuotaPerUnit)
		output := input * item.CompletionRatio
		cached := 0.0
		if item.CacheRatio != nil && *item.CacheRatio >= 0 {
			cached = input * *item.CacheRatio
		}
		id, _ := randomID()
		if _, err = tx.Exec(r.Context(), `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier) values($1,$2,$3,$4,$5,1) on conflict(model) do update set input_per_million=excluded.input_per_million,cached_input_per_million=excluded.cached_input_per_million,output_per_million=excluded.output_per_million,updated_at=now()`, id, strings.TrimSpace(item.ModelName), input, cached, output); err != nil {
			writeError(w, 500, "internal_error", "could not save pricing rules")
			return
		}
		synced++
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not save pricing rules")
		return
	}
	s.audit(r, "pricing.newapi_synced", "pricing", "newapi", map[string]any{"count": synced, "quota_per_unit": status.Data.QuotaPerUnit})
	writeJSON(w, 200, map[string]any{"synced": synced, "skipped": len(pricing.Data) - synced})
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

func (s *Service) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select u.id,u.email,u.name,u.role,u.enabled,u.created_at,coalesce(w.balance,0),coalesce(w.reserved,0),coalesce(array_agg(p.permission) filter (where p.permission is not null), '{}'),coalesce((select array_agg(ug.group_id order by ug.group_id) from user_groups ug where ug.user_id=u.id), '{}') from users u left join user_permissions p on p.user_id=u.id left join user_wallets w on w.user_id=u.id group by u.id,w.balance,w.reserved order by u.created_at desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, email, name, role string
		var enabled bool
		var created any
		var balance, reserved any
		var permissions []string
		var groups []string
		rows.Scan(&id, &email, &name, &role, &enabled, &created, &balance, &reserved, &permissions, &groups)
		out = append(out, map[string]any{"id": id, "email": email, "name": name, "role": role, "enabled": enabled, "balance": balance, "reserved": reserved, "permissions": permissions, "groups": groups, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"data": out})
}

func (s *Service) updateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email       *string   `json:"email"`
		Name        *string   `json:"name"`
		Password    *string   `json:"password"`
		Role        *string   `json:"role"`
		Enabled     *bool     `json:"enabled"`
		Permissions *[]string `json:"permissions"`
		Groups      *[]string `json:"groups"`
		Balance     *float64  `json:"balance"`
		Note        *string   `json:"note"`
	}
	if decode(r, &in) != nil {
		writeError(w, 400, "invalid_request", "invalid user update")
		return
	}
	if in.Email == nil && in.Name == nil && in.Password == nil && in.Role == nil && in.Enabled == nil && in.Permissions == nil && in.Groups == nil && in.Balance == nil {
		writeError(w, 400, "invalid_request", "at least one user field is required")
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
	if in.Password != nil && len(*in.Password) < 8 {
		writeError(w, 400, "invalid_request", "password must be at least 8 characters")
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
	if in.Balance != nil && (math.IsNaN(*in.Balance) || math.IsInf(*in.Balance, 0) || *in.Balance < 0) {
		writeError(w, 400, "invalid_request", "balance must be a non-negative number")
		return
	}
	if in.Note != nil && in.Balance == nil {
		writeError(w, 400, "invalid_request", "note can only be provided with balance")
		return
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
		if _, err = tx.Exec(r.Context(), `update users set password_hash=$1 where id=$2`, passwordHash, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update password")
			return
		}
		changed["password"] = true
	}
	if in.Role != nil {
		if _, err = tx.Exec(r.Context(), `update users set role=$1 where id=$2`, *in.Role, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update role")
			return
		}
		changed["role"] = *in.Role
	}
	if in.Enabled != nil {
		if _, err = tx.Exec(r.Context(), `update users set enabled=$1 where id=$2`, *in.Enabled, userID); err != nil {
			writeError(w, 500, "internal_error", "could not update status")
			return
		}
		changed["enabled"] = *in.Enabled
	}
	var oldBalance float64
	if in.Balance != nil {
		if _, err = tx.Exec(r.Context(), `insert into user_wallets(user_id) values($1) on conflict(user_id) do nothing`, userID); err != nil {
			writeError(w, 500, "internal_error", "could not load wallet")
			return
		}
		var reserved float64
		if err = tx.QueryRow(r.Context(), `select balance,reserved from user_wallets where user_id=$1 for update`, userID).Scan(&oldBalance, &reserved); err != nil {
			writeError(w, 500, "internal_error", "could not lock wallet")
			return
		}
		if *in.Balance < reserved {
			writeError(w, 400, "invalid_request", "balance cannot be lower than reserved amount")
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
	writeJSON(w, 200, map[string]any{"id": userID, "updated": changed})
}

func (s *Service) listGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,name,multiplier,created_at from groups order by name`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name string
		var multiplier, created any
		if rows.Scan(&id, &name, &multiplier, &created) == nil {
			data = append(data, map[string]any{"id": id, "name": name, "multiplier": multiplier, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
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
	names, err := s.groupNamesForUser(r.Context(), account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	rows, err := s.db.Query(r.Context(), `select g.id,g.name,g.multiplier,g.created_at from groups g join user_groups ug on ug.group_id=g.id where ug.user_id=$1 order by g.name`, account.userID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	groups := []map[string]any{}
	for rows.Next() {
		var id, name string
		var multiplier, created any
		if rows.Scan(&id, &name, &multiplier, &created) == nil {
			groups = append(groups, map[string]any{"id": id, "name": name, "multiplier": multiplier, "created_at": created})
		}
	}
	writeJSON(w, 200, map[string]any{"data": names, "groups": groups, "user_groups": names, "user_group": firstGroup(names)})
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
		Name       string  `json:"name"`
		Multiplier float64 `json:"multiplier"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		writeError(w, 400, "invalid_request", "name is required")
		return
	}
	if in.Multiplier == 0 {
		in.Multiplier = 1
	}
	if in.Multiplier < 0 {
		writeError(w, 400, "invalid_request", "multiplier must not be negative")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into groups(id,name,multiplier) values($1,$2,$3)`, id, strings.TrimSpace(in.Name), in.Multiplier)
	if err != nil {
		writeError(w, 409, "conflict", "group name already exists")
		return
	}
	s.audit(r, "group.created", "group", id, map[string]any{"name": in.Name, "multiplier": in.Multiplier})
	writeJSON(w, 201, map[string]any{"id": id, "name": strings.TrimSpace(in.Name), "multiplier": in.Multiplier})
}

func (s *Service) importGroups(w http.ResponseWriter, r *http.Request) {
	var values map[string]float64
	if decode(r, &values) != nil || len(values) == 0 {
		writeError(w, 400, "invalid_request", "a non-empty name-to-multiplier object is required")
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
		if name == "" || multiplier < 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
			writeError(w, 400, "invalid_request", "group names must be non-empty and multipliers must not be negative")
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
	s.audit(r, "groups.imported", "group", "bulk", map[string]any{"count": len(values)})
	writeJSON(w, 200, map[string]any{"count": len(values)})
}

func (s *Service) updateGroup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Multiplier float64 `json:"multiplier"`
	}
	if decode(r, &in) != nil || in.Multiplier < 0 {
		writeError(w, 400, "invalid_request", "multiplier must not be negative")
		return
	}
	result, err := s.db.Exec(r.Context(), `update groups set multiplier=$1 where id=$2`, in.Multiplier, r.PathValue("id"))
	if err != nil || result.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "group not found")
		return
	}
	s.audit(r, "group.updated", "group", r.PathValue("id"), map[string]any{"multiplier": in.Multiplier})
	writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "multiplier": in.Multiplier})
}

func (s *Service) modelCatalog(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	providers := s.providers(r)
	rows, err := s.db.Query(r.Context(), `
		with available as (
			select jsonb_array_elements_text(c.models) as model, c.id as channel_id
			from channels c where c.enabled
			union
			select m.public_model, c.id from model_routes m join channels c on c.id=m.channel_id
			where m.enabled and c.enabled
		), catalog as (
			select distinct a.model, coalesce(g.id::text, '__public') as group_id,
				coalesce(g.name, '公共') as group_name, coalesce(g.multiplier, 1) as group_multiplier
			from available a
			left join channel_groups cg on cg.channel_id=a.channel_id
			left join groups g on g.id=cg.group_id
			where g.id is null or exists(select 1 from user_groups ug where ug.user_id=nullif($1, '')::bigint and ug.group_id=g.id)
		)
		select c.model,c.group_id,c.group_name,c.group_multiplier,
			p.id,p.input_per_million,p.cached_input_per_million,p.output_per_million,p.multiplier
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
		var priceID *string
		var input, cached, output, modelMultiplier any
		if rows.Scan(&model, &groupID, &groupName, &groupMultiplier, &priceID, &input, &cached, &output, &modelMultiplier) != nil {
			continue
		}
		group := map[string]any{"id": groupID, "name": groupName, "multiplier": groupMultiplier}
		groups[groupID] = group
		item := models[model]
		if item == nil {
			item = &catalogModel{Model: model, Input: input, Cached: cached, Output: output, Multiplier: modelMultiplier, Groups: []map[string]any{}}
			if priceID != nil {
				item.ID = *priceID
			}
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

func validChannelProvider(provider string) bool {
	return map[string]bool{"openai": true, "ollama": true, "kimi": true, "opencode_go": true, "anthropic": true}[provider]
}

func validChannelPriority(priority int) bool {
	return priority >= -10000 && priority <= 10000
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
	result, err := s.db.Exec(r.Context(), `update users set role=$1 where id=$2`, in.Role, userID)
	if err != nil || result.RowsAffected() != 1 {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	s.audit(r, "user.role_changed", "user", userID, map[string]any{"role": in.Role})
	writeJSON(w, http.StatusOK, map[string]string{"role": in.Role})
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
	if decode(r, &in) != nil || in.UserID == "" || in.Name == "" {
		writeError(w, 400, "invalid_request", "user_id and name are required")
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
	_, err = s.db.Exec(r.Context(), `insert into api_keys(id,user_id,name,key_prefix,secret_hash,expires_at,group_id) values($1,$2,$3,$4,$5,$6,$7)`, id, in.UserID, in.Name, secret[:12], hashSecret(secret), expires, groupID)
	if err != nil {
		writeError(w, 400, "invalid_request", "unknown user")
		return
	}
	s.audit(r, "api_key.created", "api_key", id, map[string]any{"user_id": in.UserID, "name": in.Name})
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "key": secret, "expires_at": expires, "group_id": groupID})
}
func (s *Service) listKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select k.id,k.user_id,k.name,k.key_prefix,k.expires_at,k.revoked_at,k.last_used_at,k.created_at,coalesce(k.group_id::text,''),coalesce(g.name,'') from api_keys k left join groups g on g.id=k.group_id order by k.created_at desc`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, uid, name, prefix string
		var groupID, groupName string
		var expiry, revoked, used, created any
		rows.Scan(&id, &uid, &name, &prefix, &expiry, &revoked, &used, &created, &groupID, &groupName)
		data = append(data, map[string]any{"id": id, "user_id": uid, "name": name, "key_prefix": prefix, "expires_at": expiry, "revoked_at": revoked, "last_used_at": used, "created_at": created, "group_id": groupID, "group_name": groupName})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func (s *Service) validKeyGroup(ctx context.Context, userID, groupRef string) (any, error) {
	groupRef = strings.TrimSpace(groupRef)
	if groupRef == "" {
		return nil, nil
	}
	var groupID string
	err := s.db.QueryRow(ctx, `select g.id from groups g join user_groups ug on ug.group_id=g.id where ug.user_id=$1 and (g.id::text=$2 or g.name=$2)`, userID, groupRef).Scan(&groupID)
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
func (s *Service) createChannel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string   `json:"name"`
		BaseURL  string   `json:"base_url"`
		APIKey   string   `json:"api_key"`
		Models   []string `json:"models"`
		Priority int      `json:"priority"`
		Groups   []string `json:"groups"`
		Provider string   `json:"provider"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.APIKey) == "" || len(in.Models) == 0 {
		writeError(w, 400, "invalid_request", "name, api_key, and models are required")
		return
	}
	if in.Provider == "" {
		in.Provider = "openai"
	}
	if !validChannelProvider(in.Provider) {
		writeError(w, 400, "invalid_request", "unsupported provider")
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
	u, err := url.Parse(in.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname()))) {
		writeError(w, 400, "invalid_request", "base_url must use HTTPS, except for loopback HTTP services")
		return
	}
	encrypted, err := crypt(s.cfg.EncryptionKey, in.APIKey, false)
	if err != nil {
		writeError(w, 500, "internal_error", "credential encryption failed")
		return
	}
	models, _ := json.Marshal(in.Models)
	id, _ := randomID()
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not create channel")
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `insert into channels(id,name,base_url,api_key,models,priority,provider) values($1,$2,$3,$4,$5,$6,$7)`, id, in.Name, strings.TrimRight(in.BaseURL, "/"), encrypted, models, in.Priority, in.Provider)
	if err != nil {
		writeError(w, 409, "conflict", "channel name already exists")
		return
	}
	for _, groupID := range groupIDs {
		if _, err = tx.Exec(r.Context(), `insert into channel_groups(channel_id,group_id) values($1,$2)`, id, groupID); err != nil {
			writeError(w, 400, "invalid_request", "unknown group")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, 500, "internal_error", "could not create channel")
		return
	}
	s.audit(r, "channel.created", "channel", id, map[string]any{"name": in.Name, "models": in.Models, "provider": in.Provider})
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "models": in.Models, "provider": in.Provider, "enabled": true})
}
func (s *Service) updateChannel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string   `json:"name"`
		BaseURL  string   `json:"base_url"`
		APIKey   string   `json:"api_key"`
		Models   []string `json:"models"`
		Priority int      `json:"priority"`
		Provider string   `json:"provider"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || len(in.Models) == 0 {
		writeError(w, 400, "invalid_request", "name and models are required")
		return
	}
	if in.Provider == "" {
		in.Provider = "openai"
	}
	if !validChannelProvider(in.Provider) {
		writeError(w, 400, "invalid_request", "unsupported provider")
		return
	}
	if !validChannelPriority(in.Priority) {
		writeError(w, 400, "invalid_request", "priority must be between -10000 and 10000")
		return
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname()))) {
		writeError(w, 400, "invalid_request", "base_url must use HTTPS, except for loopback HTTP services")
		return
	}
	models, _ := json.Marshal(in.Models)
	args := []any{strings.TrimSpace(in.Name), strings.TrimRight(in.BaseURL, "/"), models, in.Priority, in.Provider, r.PathValue("id")}
	query := `update channels set name=$1,base_url=$2,models=$3,priority=$4,provider=$5,updated_at=now() where id=$6`
	if strings.TrimSpace(in.APIKey) != "" {
		encrypted, err := crypt(s.cfg.EncryptionKey, in.APIKey, false)
		if err != nil {
			writeError(w, 500, "internal_error", "credential encryption failed")
			return
		}
		args = []any{strings.TrimSpace(in.Name), strings.TrimRight(in.BaseURL, "/"), encrypted, models, in.Priority, in.Provider, r.PathValue("id")}
		query = `update channels set name=$1,base_url=$2,api_key=$3,models=$4,priority=$5,provider=$6,updated_at=now() where id=$7`
	}
	result, err := s.db.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, 409, "conflict", "channel name already exists")
		return
	}
	if result.RowsAffected() != 1 {
		writeError(w, 404, "not_found", "channel not found")
		return
	}
	s.audit(r, "channel.updated", "channel", r.PathValue("id"), map[string]any{"name": in.Name, "models": in.Models, "provider": in.Provider})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Service) listChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select c.id,c.name,c.base_url,c.models,c.enabled,c.auto_disabled,c.disabled_reason,c.priority,c.created_at,c.updated_at,coalesce((select array_agg(cg.group_id order by cg.group_id) from channel_groups cg where cg.channel_id=c.id), '{}'),c.provider from channels c order by c.priority,c.id`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, name, base string
		var models []byte
		var enabled, autoDisabled bool
		var disabledReason string
		var priority int
		var created, updated any
		var groups []string
		var provider string
		if rows.Scan(&id, &name, &base, &models, &enabled, &autoDisabled, &disabledReason, &priority, &created, &updated, &groups, &provider) != nil {
			continue
		}
		var list []string
		json.Unmarshal(models, &list)
		data = append(data, map[string]any{"id": id, "name": name, "base_url": base, "models": list, "provider": provider, "enabled": enabled, "auto_disabled": autoDisabled, "disabled_reason": disabledReason, "priority": priority, "groups": groups, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]any{"data": data})
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
	writeJSON(w, 200, map[string]bool{"enabled": in.Enabled})
}

func (s *Service) adjustBalance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if decode(r, &in) != nil || in.UserID == "" || in.Amount == 0 || in.Note == "" {
		writeError(w, 400, "invalid_request", "user_id, non-zero amount, and note are required")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "internal_error", "could not adjust balance")
		return
	}
	defer tx.Rollback(r.Context())
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
	rows, err := s.db.Query(r.Context(), `select id,public_model,upstream_model,channel_id,priority,weight,enabled,created_at from model_routes order by public_model,priority`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, public, upstream, channelID string
		var priority, weight int
		var enabled bool
		var created any
		if rows.Scan(&id, &public, &upstream, &channelID, &priority, &weight, &enabled, &created) == nil {
			data = append(data, map[string]any{"id": id, "public_model": public, "upstream_model": upstream, "channel_id": channelID, "priority": priority, "weight": weight, "enabled": enabled, "created_at": created})
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
	}
	if decode(r, &in) != nil || in.PublicModel == "" || in.UpstreamModel == "" || in.ChannelID == "" {
		writeError(w, 400, "invalid_request", "public_model, upstream_model, and channel_id are required")
		return
	}
	if in.Weight <= 0 {
		in.Weight = 100
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into model_routes(id,public_model,upstream_model,channel_id,priority,weight) values($1,$2,$3,$4,$5,$6)`, id, in.PublicModel, in.UpstreamModel, in.ChannelID, in.Priority, in.Weight)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not create model route")
		return
	}
	s.audit(r, "model_route.created", "model_route", id, map[string]any{"public_model": in.PublicModel, "channel_id": in.ChannelID})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Service) upsertQuota(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID      string `json:"user_id"`
		APIKeyID    string `json:"api_key_id"`
		Model       string `json:"model"`
		Window      string `json:"window"`
		MaxRequests *int64 `json:"max_requests"`
		MaxTokens   *int64 `json:"max_tokens"`
	}
	if decode(r, &in) != nil || (in.UserID == "" && in.APIKeyID == "") || (in.Window != "minute" && in.Window != "day" && in.Window != "month") || (in.MaxRequests == nil && in.MaxTokens == nil) {
		writeError(w, 400, "invalid_request", "scope, window, and a limit are required")
		return
	}
	if (in.MaxRequests != nil && *in.MaxRequests < 0) || (in.MaxTokens != nil && *in.MaxTokens < 0) {
		writeError(w, 400, "invalid_request", "limits cannot be negative")
		return
	}
	id, _ := randomID()
	_, err := s.db.Exec(r.Context(), `insert into quota_limits(id,user_id,api_key_id,model,"window",max_requests,max_tokens) values($1,nullif($2,'')::bigint,nullif($3,'')::uuid,nullif($4,''),$5,$6,$7) on conflict (coalesce(user_id, 0), coalesce(api_key_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(model, ''), "window") do update set max_requests=excluded.max_requests,max_tokens=excluded.max_tokens`, id, in.UserID, in.APIKeyID, in.Model, in.Window, in.MaxRequests, in.MaxTokens)
	if err != nil {
		writeError(w, 400, "invalid_request", "could not save quota")
		return
	}
	s.audit(r, "quota.updated", "quota", id, map[string]any{"user_id": in.UserID, "api_key_id": in.APIKeyID, "model": in.Model, "window": in.Window})
	writeJSON(w, 200, map[string]any{"id": id})
}
func (s *Service) listLogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select request_id,user_id,api_key_id,channel_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code,created_at from request_logs order by created_at desc limit 100`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var requestID, model string
		var uid, kid, cid, prompt, completion, total, errorCode, created any
		var status, duration int
		rows.Scan(&requestID, &uid, &kid, &cid, &model, &status, &prompt, &completion, &total, &duration, &errorCode, &created)
		data = append(data, map[string]any{"request_id": requestID, "user_id": uid, "api_key_id": kid, "channel_id": cid, "model": model, "status_code": status, "prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total, "duration_ms": duration, "error_code": errorCode, "created_at": created})
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
	in.SourceDSN = strings.TrimPrefix(in.SourceDSN, "mysql://")

	log.Printf("Migration requested: driver=%s source=%s target=%s", in.SourceDriver, in.SourceDSN, s.cfg.DatabaseURL)

	if !s.startMigration(in.SourceDSN, in.SourceDriver) {
		writeError(w, 409, "migration_already_running", "A migration is already in progress")
		return
	}

	s.audit(r, "system.migrate", "system", "", map[string]any{"source_driver": in.SourceDriver})
	writeJSON(w, 200, map[string]any{"message": "Migration started"})
}
