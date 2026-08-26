package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxContentPolicyRules = 1000
	maxContentPolicyTerm  = 512
	maxContentPolicyName  = 200
	maxPolicyScanBytes    = 2 << 20
	maxAuditExcerpt       = 1000
)

var policyUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type contentPolicySettings struct {
	Enabled       bool   `json:"request_audit_enabled"`
	StoreMode     string `json:"request_audit_store_mode"`
	RetentionDays int    `json:"request_audit_retention_days"`
	Mode          string `json:"content_policy_mode"`
}

type contentPolicyRule struct {
	ID            string
	Name          string
	Term          string
	Action        string
	CaseSensitive bool
	Enabled       bool
	Priority      int
}

type contentPolicySnapshot struct {
	Settings contentPolicySettings
	Rules    []contentPolicyRule
}

type contentPolicyResult struct {
	Decision       string
	MatchedRuleIDs []string
	Excerpt        string
	ContentHash    string
	ContentLength  int
}

type contentPolicyRequestKey struct{}
type contentPolicyEvaluatedKey struct{}

type contentPolicyRequest struct {
	body     []byte
	endpoint string
}

func withContentPolicyRequest(ctx context.Context, body []byte, endpoint string) context.Context {
	return context.WithValue(ctx, contentPolicyRequestKey{}, contentPolicyRequest{body: body, endpoint: endpoint})
}

func contentPolicyRequestFromContext(ctx context.Context) (contentPolicyRequest, bool) {
	request, ok := ctx.Value(contentPolicyRequestKey{}).(contentPolicyRequest)
	return request, ok
}

func markContentPolicyEvaluated(ctx context.Context) context.Context {
	return context.WithValue(ctx, contentPolicyEvaluatedKey{}, true)
}

func contentPolicyAlreadyEvaluated(ctx context.Context) bool {
	evaluated, _ := ctx.Value(contentPolicyEvaluatedKey{}).(bool)
	return evaluated
}

type contentPolicyRuleInput struct {
	Name          *string `json:"name"`
	Term          *string `json:"term"`
	Action        *string `json:"action"`
	CaseSensitive *bool   `json:"case_sensitive"`
	Enabled       *bool   `json:"enabled"`
	Priority      *int    `json:"priority"`
}

type contentPolicySettingsInput struct {
	Enabled       *bool   `json:"request_audit_enabled"`
	StoreMode     *string `json:"request_audit_store_mode"`
	RetentionDays *int    `json:"request_audit_retention_days"`
	Mode          *string `json:"content_policy_mode"`
}

func defaultContentPolicySettings() contentPolicySettings {
	return contentPolicySettings{Enabled: true, StoreMode: "hash", RetentionDays: 30, Mode: "off"}
}

func (s *Service) loadContentPolicy(ctx context.Context) (contentPolicySnapshot, error) {
	snapshot := contentPolicySnapshot{Settings: defaultContentPolicySettings()}
	if s.db == nil {
		return snapshot, fmt.Errorf("database is not configured")
	}
	if err := s.db.QueryRow(ctx, `select request_audit_enabled,request_audit_store_mode,request_audit_retention_days,content_policy_mode from site_settings where id=true`).Scan(&snapshot.Settings.Enabled, &snapshot.Settings.StoreMode, &snapshot.Settings.RetentionDays, &snapshot.Settings.Mode); err != nil {
		return snapshot, err
	}
	rows, err := s.db.Query(ctx, `select id,name,term,action,case_sensitive,enabled,priority from content_policy_rules where enabled order by priority asc,created_at asc limit $1`, maxContentPolicyRules)
	if err != nil {
		return snapshot, err
	}
	defer rows.Close()
	for rows.Next() {
		var rule contentPolicyRule
		if rows.Scan(&rule.ID, &rule.Name, &rule.Term, &rule.Action, &rule.CaseSensitive, &rule.Enabled, &rule.Priority) == nil {
			snapshot.Rules = append(snapshot.Rules, rule)
		}
	}
	return snapshot, rows.Err()
}

func (s *Service) contentPolicy(ctx context.Context) contentPolicySnapshot {
	if s.contentPolicyData != nil {
		snapshot, err := s.contentPolicyData.get(ctx, struct{}{}, func(ctx context.Context) (contentPolicySnapshot, error) {
			return s.loadContentPolicy(ctx)
		})
		if err == nil {
			return snapshot
		}
	} else if snapshot, err := s.loadContentPolicy(ctx); err == nil {
		return snapshot
	}
	return contentPolicySnapshot{Settings: defaultContentPolicySettings()}
}

func normalizePolicyText(value string, caseSensitive bool) string {
	value = strings.Join(strings.Fields(value), " ")
	if !caseSensitive {
		value = strings.ToLower(value)
	}
	return value
}

var policyTextKeys = map[string]bool{
	"arguments": true, "content": true, "description": true, "function": true,
	"input": true, "input_text": true, "instructions": true, "message": true,
	"messages": true, "name": true, "output": true, "prompt": true, "system": true,
	"text": true, "tool": true, "tool_choice": true, "tool_result": true, "tool_use": true,
	"tools": true,
}

func appendPolicyText(builder *strings.Builder, value any, key string) {
	if builder.Len() >= maxPolicyScanBytes {
		return
	}
	switch item := value.(type) {
	case string:
		if key == "" || !policyTextKeys[strings.ToLower(key)] {
			return
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		remaining := maxPolicyScanBytes - builder.Len()
		if len(item) > remaining {
			item = item[:remaining]
		}
		builder.WriteString(item)
	case []any:
		for _, child := range item {
			appendPolicyText(builder, child, key)
			if builder.Len() >= maxPolicyScanBytes {
				return
			}
		}
	case map[string]any:
		for childKey, child := range item {
			if policyTextKeys[strings.ToLower(childKey)] {
				appendPolicyText(builder, child, childKey)
			}
			if builder.Len() >= maxPolicyScanBytes {
				return
			}
		}
	}
}

func extractPolicyText(body []byte) string {
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	var builder strings.Builder
	if root, ok := payload.(map[string]any); ok {
		for key, value := range root {
			if policyTextKeys[strings.ToLower(key)] {
				appendPolicyText(&builder, value, key)
			}
		}
	}
	return builder.String()
}

func redactPolicyTerm(value, term string, caseSensitive bool) string {
	if term == "" {
		return value
	}
	needle := term
	comparison := value
	if !caseSensitive {
		needle = strings.ToLower(term)
		comparison = strings.ToLower(value)
	}
	var out strings.Builder
	last := 0
	for {
		index := strings.Index(comparison[last:], needle)
		if index < 0 {
			out.WriteString(value[last:])
			return out.String()
		}
		index += last
		out.WriteString(value[last:index])
		out.WriteString("[redacted]")
		last = index + len(term)
	}
}

func (s *Service) contentHash(value string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.EncryptionKey))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) evaluateContentPolicy(snapshot contentPolicySnapshot, body []byte) contentPolicyResult {
	text := extractPolicyText(body)
	result := contentPolicyResult{Decision: "allow", MatchedRuleIDs: []string{}, ContentLength: len([]byte(text))}
	normalized := normalizePolicyText(text, false)
	if snapshot.Settings.Mode == "off" {
		if snapshot.Settings.StoreMode == "hash" || snapshot.Settings.StoreMode == "excerpt" {
			result.ContentHash = s.contentHash(normalized)
		}
		if snapshot.Settings.StoreMode == "excerpt" {
			result.Excerpt = normalized
		}
		if len([]rune(result.Excerpt)) > maxAuditExcerpt {
			result.Excerpt = string([]rune(result.Excerpt)[:maxAuditExcerpt])
		}
		return result
	}
	for _, rule := range snapshot.Rules {
		if !rule.Enabled {
			continue
		}
		term := normalizePolicyText(rule.Term, rule.CaseSensitive)
		if term == "" || !strings.Contains(normalizePolicyText(text, rule.CaseSensitive), term) {
			continue
		}
		result.MatchedRuleIDs = append(result.MatchedRuleIDs, rule.ID)
		if snapshot.Settings.Mode == "block" && rule.Action == "block" {
			result.Decision = "block"
		} else if result.Decision == "allow" {
			result.Decision = "audit"
		}
		if snapshot.Settings.StoreMode == "excerpt" && result.Excerpt == "" {
			result.Excerpt = normalized
		}
	}
	if snapshot.Settings.StoreMode == "hash" || snapshot.Settings.StoreMode == "excerpt" {
		result.ContentHash = s.contentHash(normalized)
	}
	if len([]rune(result.Excerpt)) > maxAuditExcerpt {
		result.Excerpt = string([]rune(result.Excerpt)[:maxAuditExcerpt])
	}
	for _, rule := range snapshot.Rules {
		if containsString(result.MatchedRuleIDs, rule.ID) {
			result.Excerpt = redactPolicyTerm(result.Excerpt, normalizePolicyText(rule.Term, false), false)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Service) recordContentAudit(ctx context.Context, key keyContext, model, endpoint string, requestBytes int, result contentPolicyResult, settings contentPolicySettings) {
	if s.db == nil || !settings.Enabled {
		return
	}
	id, err := randomID()
	if err != nil {
		return
	}
	storedHash, storedExcerpt := result.ContentHash, result.Excerpt
	if settings.StoreMode == "none" {
		storedHash, storedExcerpt = "", ""
	}
	_, _ = s.db.Exec(ctx, `insert into request_content_audits(id,request_id,user_id,api_key_id,model,endpoint,decision,matched_rule_ids,request_bytes,content_length,content_hash,excerpt) values($1,$2,$3,$4,$5,$6,$7,$8::text[],$9,$10,$11,$12) on conflict(request_id) do update set decision=excluded.decision,matched_rule_ids=excluded.matched_rule_ids,request_bytes=excluded.request_bytes,content_length=excluded.content_length,content_hash=excluded.content_hash,excerpt=excluded.excerpt`, id, requestID(ctx), key.userID, key.keyID, model, endpoint, result.Decision, result.MatchedRuleIDs, requestBytes, result.ContentLength, storedHash, storedExcerpt)
}

func (s *Service) enforceContentPolicy(ctx context.Context, key keyContext, model, endpoint string, body []byte) (bool, context.Context) {
	snapshot := s.contentPolicy(ctx)
	result := s.evaluateContentPolicy(snapshot, body)
	s.recordContentAudit(ctx, key, model, endpoint, len(body), result, snapshot.Settings)
	if result.Decision == "block" {
		return false, markContentPolicyEvaluated(ctx)
	}
	return true, markContentPolicyEvaluated(withContentPolicyRequest(ctx, body, endpoint))
}

func (s *Service) getContentPolicySettings(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.loadContentPolicy(r.Context())
	if err != nil {
		snapshot.Settings = defaultContentPolicySettings()
	}
	writeJSON(w, http.StatusOK, snapshot.Settings)
}

func validContentPolicySettings(settings contentPolicySettings) bool {
	return (settings.StoreMode == "none" || settings.StoreMode == "hash" || settings.StoreMode == "excerpt") && (settings.Mode == "off" || settings.Mode == "audit" || settings.Mode == "block") && settings.RetentionDays >= 1 && settings.RetentionDays <= 3650
}

func (s *Service) updateContentPolicySettings(w http.ResponseWriter, r *http.Request) {
	var in contentPolicySettingsInput
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid content policy settings")
		return
	}
	current, err := s.loadContentPolicy(r.Context())
	if err != nil {
		current.Settings = defaultContentPolicySettings()
	}
	if in.Enabled != nil {
		current.Settings.Enabled = *in.Enabled
	}
	if in.StoreMode != nil {
		current.Settings.StoreMode = strings.TrimSpace(*in.StoreMode)
	}
	if in.RetentionDays != nil {
		current.Settings.RetentionDays = *in.RetentionDays
	}
	if in.Mode != nil {
		current.Settings.Mode = strings.TrimSpace(*in.Mode)
	}
	if !validContentPolicySettings(current.Settings) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid content policy settings")
		return
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set request_audit_enabled=$1,request_audit_store_mode=$2,request_audit_retention_days=$3,content_policy_mode=$4,updated_at=now() where id=true`, current.Settings.Enabled, current.Settings.StoreMode, current.Settings.RetentionDays, current.Settings.Mode); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save content policy settings")
		return
	}
	s.contentPolicyData.clear()
	s.audit(r, "content_policy.settings_updated", "site_settings", "content_policy", map[string]any{"enabled": current.Settings.Enabled, "store_mode": current.Settings.StoreMode, "retention_days": current.Settings.RetentionDays, "mode": current.Settings.Mode})
	writeJSON(w, http.StatusOK, current.Settings)
}

func contentPolicyRuleResponseFrom(rule contentPolicyRule, createdAt, updatedAt any) map[string]any {
	return map[string]any{"id": rule.ID, "name": rule.Name, "term": rule.Term, "action": rule.Action, "case_sensitive": rule.CaseSensitive, "enabled": rule.Enabled, "priority": rule.Priority, "created_at": createdAt, "updated_at": updatedAt}
}

func (s *Service) listContentPolicyRules(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,name,term,action,case_sensitive,enabled,priority,created_at,updated_at from content_policy_rules order by priority asc,created_at asc limit $1`, maxContentPolicyRules)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var rule contentPolicyRule
		var createdAt, updatedAt any
		if rows.Scan(&rule.ID, &rule.Name, &rule.Term, &rule.Action, &rule.CaseSensitive, &rule.Enabled, &rule.Priority, &createdAt, &updatedAt) == nil {
			data = append(data, contentPolicyRuleResponseFrom(rule, createdAt, updatedAt))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func validPolicyRule(rule contentPolicyRule) bool {
	return strings.TrimSpace(rule.Name) != "" && len([]rune(rule.Name)) <= maxContentPolicyName && strings.TrimSpace(rule.Term) != "" && len([]rune(rule.Term)) <= maxContentPolicyTerm && (rule.Action == "block" || rule.Action == "audit") && rule.Priority >= 0 && rule.Priority <= 100000
}

func (s *Service) createContentPolicyRule(w http.ResponseWriter, r *http.Request) {
	var in contentPolicyRuleInput
	if decode(r, &in) != nil || in.Name == nil || in.Term == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "name and term are required")
		return
	}
	rule := contentPolicyRule{Name: strings.TrimSpace(*in.Name), Term: strings.TrimSpace(*in.Term), Action: "block", Enabled: true, Priority: 100}
	if in.Action != nil {
		rule.Action = strings.TrimSpace(*in.Action)
	}
	if in.CaseSensitive != nil {
		rule.CaseSensitive = *in.CaseSensitive
	}
	if in.Enabled != nil {
		rule.Enabled = *in.Enabled
	}
	if in.Priority != nil {
		rule.Priority = *in.Priority
	}
	if !validPolicyRule(rule) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid content policy rule")
		return
	}
	id, _ := randomID()
	actor := accountFromContext(r).userID
	if _, err := s.db.Exec(r.Context(), `insert into content_policy_rules(id,name,term,action,case_sensitive,enabled,priority,created_by,updated_by) values($1,$2,$3,$4,$5,$6,$7,nullif($8,'')::bigint,nullif($8,'')::bigint)`, id, rule.Name, rule.Term, rule.Action, rule.CaseSensitive, rule.Enabled, rule.Priority, actor); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create content policy rule")
		return
	}
	s.contentPolicyData.clear()
	s.audit(r, "content_policy.rule_created", "content_policy_rule", id, map[string]any{"name": rule.Name, "action": rule.Action, "enabled": rule.Enabled})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Service) updateContentPolicyRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !policyUUIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid rule id")
		return
	}
	var in contentPolicyRuleInput
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid content policy rule")
		return
	}
	var rule contentPolicyRule
	if err := s.db.QueryRow(r.Context(), `select id,name,term,action,case_sensitive,enabled,priority from content_policy_rules where id=$1`, id).Scan(&rule.ID, &rule.Name, &rule.Term, &rule.Action, &rule.CaseSensitive, &rule.Enabled, &rule.Priority); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "content policy rule not found")
		return
	}
	if in.Name != nil {
		rule.Name = strings.TrimSpace(*in.Name)
	}
	if in.Term != nil {
		rule.Term = strings.TrimSpace(*in.Term)
	}
	if in.Action != nil {
		rule.Action = strings.TrimSpace(*in.Action)
	}
	if in.CaseSensitive != nil {
		rule.CaseSensitive = *in.CaseSensitive
	}
	if in.Enabled != nil {
		rule.Enabled = *in.Enabled
	}
	if in.Priority != nil {
		rule.Priority = *in.Priority
	}
	if !validPolicyRule(rule) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid content policy rule")
		return
	}
	actor := accountFromContext(r).userID
	if _, err := s.db.Exec(r.Context(), `update content_policy_rules set name=$1,term=$2,action=$3,case_sensitive=$4,enabled=$5,priority=$6,updated_by=nullif($7,'')::bigint,updated_at=now() where id=$8`, rule.Name, rule.Term, rule.Action, rule.CaseSensitive, rule.Enabled, rule.Priority, actor, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update content policy rule")
		return
	}
	s.contentPolicyData.clear()
	s.audit(r, "content_policy.rule_updated", "content_policy_rule", id, map[string]any{"name": rule.Name, "action": rule.Action, "enabled": rule.Enabled})
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Service) deleteContentPolicyRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !policyUUIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid rule id")
		return
	}
	if _, err := s.db.Exec(r.Context(), `delete from content_policy_rules where id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not delete content policy rule")
		return
	}
	s.contentPolicyData.clear()
	s.audit(r, "content_policy.rule_deleted", "content_policy_rule", id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) listRequestContentAudits(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiOrDefault(q.Get("page"), 1)
	pageSize := atoiOrDefault(q.Get("page_size"), 50)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	where := []string{"1=1"}
	args := []any{}
	arg := 1
	add := func(condition string, value any) {
		where = append(where, condition+"$"+strconv.Itoa(arg))
		args = append(args, value)
		arg++
	}
	if value := strings.TrimSpace(q.Get("user_id")); value != "" {
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid user id")
			return
		}
		add("user_id=", value)
	}
	if value := strings.TrimSpace(q.Get("model")); value != "" {
		add("model=", value)
	}
	if value := strings.TrimSpace(q.Get("decision")); value == "allow" || value == "audit" || value == "block" {
		add("decision=", value)
	}
	if value := strings.TrimSpace(q.Get("request_id")); value != "" {
		add("request_id like ", value+"%")
	}
	if value := strings.TrimSpace(q.Get("rule_id")); value != "" {
		if !policyUUIDPattern.MatchString(value) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid rule id")
			return
		}
		where = append(where, "$"+strconv.Itoa(arg)+"=any(matched_rule_ids)")
		args = append(args, value)
		arg++
	}
	if value := strings.TrimSpace(q.Get("start")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			add("created_at>=", parsed)
		}
	}
	if value := strings.TrimSpace(q.Get("end")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			add("created_at<=", parsed)
		}
	}
	whereClause := " where " + strings.Join(where, " and ")
	var total int
	if err := s.db.QueryRow(r.Context(), `select count(*) from request_content_audits`+whereClause, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	query := `select id,request_id,coalesce(user_id::text,''),coalesce(api_key_id::text,''),model,endpoint,decision,matched_rule_ids,request_bytes,content_length,content_hash,excerpt,created_at from request_content_audits` + whereClause + ` order by created_at desc limit $` + strconv.Itoa(arg) + ` offset $` + strconv.Itoa(arg+1)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, requestID, userID, apiKeyID, model, endpoint, decision, hash, excerpt string
		var ruleIDs []string
		var requestBytes, contentLength int
		var createdAt any
		if rows.Scan(&id, &requestID, &userID, &apiKeyID, &model, &endpoint, &decision, &ruleIDs, &requestBytes, &contentLength, &hash, &excerpt, &createdAt) == nil {
			data = append(data, map[string]any{"id": id, "request_id": requestID, "user_id": userID, "api_key_id": apiKeyID, "model": model, "endpoint": endpoint, "decision": decision, "matched_rule_ids": ruleIDs, "request_bytes": requestBytes, "content_length": contentLength, "content_hash": hash, "excerpt": excerpt, "created_at": createdAt})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func (s *Service) cleanupContentAudits(ctx context.Context) {
	if s.db == nil {
		return
	}
	settings := s.contentPolicy(ctx).Settings
	if settings.RetentionDays < 1 {
		return
	}
	_, _ = s.db.Exec(ctx, `delete from request_content_audits where created_at < now() - make_interval(days => $1)`, settings.RetentionDays)
}
