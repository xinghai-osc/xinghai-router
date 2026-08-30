package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRequestTimeoutSeconds = 90
	minRequestTimeoutSeconds     = 1
	maxRequestTimeoutSeconds     = 3600
	healthCheckProbeTimeout      = 30 * time.Second
)

type reliabilitySettings struct {
	RequestTimeoutSeconds      int    `json:"request_timeout_seconds"`
	RetryCount                 int    `json:"retry_count"`
	RetryStatusCodes           string `json:"retry_status_codes"`
	HealthCheckMode            string `json:"health_check_mode"`
	HealthCheckIntervalMinutes int    `json:"health_check_interval_minutes"`
	HealthCheckAutoRecover     bool   `json:"health_check_auto_recover"`
	HealthCheckChannelIDs      string `json:"health_check_channel_ids"`
	AutoDisableOnTestFailure   bool   `json:"auto_disable_on_test_failure"`
	AutoDisableSlowSeconds     int    `json:"auto_disable_slow_seconds"`
	AutoDisableStatusCodes     string `json:"auto_disable_status_codes"`
	AutoDisableKeywords        string `json:"auto_disable_keywords"`
	parsedRetryCodes           *statusMatcher
	parsedDisableCodes         *statusMatcher
	parsedKeywords             []string
	parsedChannelIDs           map[string]bool
}

func requestTimeoutDuration(seconds int) time.Duration {
	if seconds < minRequestTimeoutSeconds || seconds > maxRequestTimeoutSeconds {
		seconds = defaultRequestTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func defaultReliabilitySettings() reliabilitySettings {
	s := reliabilitySettings{
		RequestTimeoutSeconds:      defaultRequestTimeoutSeconds,
		RetryCount:                 3,
		RetryStatusCodes:           "100-199,300-407,409-503,505-523,525-599",
		HealthCheckMode:            "off",
		HealthCheckIntervalMinutes: 5,
		HealthCheckAutoRecover:     true,
		HealthCheckChannelIDs:      "",
		AutoDisableOnTestFailure:   false,
		AutoDisableSlowSeconds:     0,
		AutoDisableStatusCodes:     "401,429,503",
		AutoDisableKeywords: strings.Join([]string{
			"Your credit balance is too low",
			"This organization has been disabled.",
			"You exceeded your current quota",
			"Permission denied",
			"The security token included in the request is invalid",
			"Operation not allowed",
			"Your account is not authorized",
			"订阅额度不足或未配置订阅",
			"所有账号暂时不可用",
			"已达到 Token Plan 用量上限",
			"Weekly usage limit reached.",
			"5-hour usage limit reached",
			"Invalid token",
			"Too Many Requests",
			"You have exceeded the monthly usage quota",
			"You have exceeded the weekly usage quota. It will reset at ",
		}, "\n"),
	}
	s.compile()
	return s
}

func (s *reliabilitySettings) compile() {
	s.parsedRetryCodes = parseStatusMatcher(s.RetryStatusCodes)
	s.parsedDisableCodes = parseStatusMatcher(s.AutoDisableStatusCodes)
	s.parsedKeywords = splitKeywords(s.AutoDisableKeywords)
	s.parsedChannelIDs = parseIDList(s.HealthCheckChannelIDs)
}

func (s reliabilitySettings) retryable(status int) bool {
	if s.parsedRetryCodes == nil {
		return false
	}
	return s.parsedRetryCodes.match(status)
}

func (s reliabilitySettings) autoDisableStatus(status int) bool {
	if s.parsedDisableCodes == nil {
		return false
	}
	return s.parsedDisableCodes.match(status)
}

func (s reliabilitySettings) autoDisableKeyword(body string) bool {
	return s.matchedKeyword(body) != ""
}

// matchedKeyword returns the first configured keyword present in the response
// body, or the empty string when no keyword matches.
func (s reliabilitySettings) matchedKeyword(body string) string {
	lower := strings.ToLower(body)
	for _, keyword := range s.parsedKeywords {
		if strings.Contains(lower, keyword) {
			return keyword
		}
	}
	if strings.Contains(lower, "credit balance is too low") {
		return "your credit balance is too low"
	}
	return ""
}

type statusMatcher struct {
	set    map[int]bool
	ranges [][2]int
}

func parseStatusMatcher(input string) *statusMatcher {
	m := &statusMatcher{set: map[int]bool{}}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, errLo := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, errHi := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if errLo != nil || errHi != nil || lo < 100 || hi > 599 || lo > hi {
				continue
			}
			m.ranges = append(m.ranges, [2]int{lo, hi})
			continue
		}
		code, err := strconv.Atoi(part)
		if err != nil || code < 100 || code > 599 {
			continue
		}
		m.set[code] = true
	}
	return m
}

func (m *statusMatcher) match(status int) bool {
	if m.set[status] {
		return true
	}
	for _, r := range m.ranges {
		if status >= r[0] && status <= r[1] {
			return true
		}
	}
	return false
}

func validStatusCodeSpec(input string) bool {
	seen := false
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			// Trailing or repeated separators are not accepted.
			return false
		}
		seen = true
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, errLo := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, errHi := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if errLo != nil || errHi != nil || lo < 100 || hi > 599 || lo > hi {
				return false
			}
			continue
		}
		code, err := strconv.Atoi(part)
		if err != nil || code < 100 || code > 599 {
			return false
		}
	}
	return seen
}

func splitKeywords(input string) []string {
	var keywords []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "" {
			keywords = append(keywords, line)
		}
	}
	return keywords
}

func parseIDList(input string) map[string]bool {
	ids := map[string]bool{}
	for _, field := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r' }) {
		field = strings.TrimSpace(field)
		if field != "" {
			ids[field] = true
		}
	}
	return ids
}

// reliabilitySettings is read on every proxied request, so it is cached for a few
// seconds. updateReliabilitySettings invalidates the cache on write.
func (s *Service) reliabilitySettings(ctx context.Context) reliabilitySettings {
	settings, err := s.reliabilityData.get(ctx, struct{}{}, s.loadReliabilitySettings)
	if err != nil {
		return defaultReliabilitySettings()
	}
	return settings
}

func (s *Service) loadReliabilitySettings(ctx context.Context) (reliabilitySettings, error) {
	settings := defaultReliabilitySettings()
	if s.db == nil {
		return settings, errors.New("database is unavailable")
	}
	var requestTimeoutSeconds, retryCount, interval, slowSeconds int
	var retryCodes, mode, channelIDs, disableCodes, keywords string
	var autoRecover, autoDisableOnFailure bool
	err := s.db.QueryRow(ctx, `select request_timeout_seconds,retry_count,retry_status_codes,health_check_mode,health_check_interval_minutes,health_check_auto_recover,health_check_channel_ids,auto_disable_on_test_failure,auto_disable_slow_seconds,auto_disable_status_codes,auto_disable_keywords from site_settings where id=true`).Scan(&requestTimeoutSeconds, &retryCount, &retryCodes, &mode, &interval, &autoRecover, &channelIDs, &autoDisableOnFailure, &slowSeconds, &disableCodes, &keywords)
	if err != nil {
		return settings, err
	}
	if requestTimeoutSeconds >= minRequestTimeoutSeconds && requestTimeoutSeconds <= maxRequestTimeoutSeconds {
		settings.RequestTimeoutSeconds = requestTimeoutSeconds
	}
	settings.RetryCount = retryCount
	settings.RetryStatusCodes = retryCodes
	settings.HealthCheckMode = mode
	settings.HealthCheckIntervalMinutes = interval
	settings.HealthCheckAutoRecover = autoRecover
	settings.HealthCheckChannelIDs = channelIDs
	settings.AutoDisableOnTestFailure = autoDisableOnFailure
	settings.AutoDisableSlowSeconds = slowSeconds
	settings.AutoDisableStatusCodes = disableCodes
	settings.AutoDisableKeywords = keywords
	settings.compile()
	return settings, nil
}

// getReliabilitySettings bypasses the cache so the console always shows persisted values.
func (s *Service) getReliabilitySettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.loadReliabilitySettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load reliability settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Service) updateReliabilitySettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RequestTimeoutSeconds      *int    `json:"request_timeout_seconds"`
		RetryCount                 *int    `json:"retry_count"`
		RetryStatusCodes           *string `json:"retry_status_codes"`
		HealthCheckMode            *string `json:"health_check_mode"`
		HealthCheckIntervalMinutes *int    `json:"health_check_interval_minutes"`
		HealthCheckAutoRecover     *bool   `json:"health_check_auto_recover"`
		HealthCheckChannelIDs      *string `json:"health_check_channel_ids"`
		AutoDisableOnTestFailure   *bool   `json:"auto_disable_on_test_failure"`
		AutoDisableSlowSeconds     *int    `json:"auto_disable_slow_seconds"`
		AutoDisableStatusCodes     *string `json:"auto_disable_status_codes"`
		AutoDisableKeywords        *string `json:"auto_disable_keywords"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid reliability settings")
		return
	}
	if in.RequestTimeoutSeconds != nil && (*in.RequestTimeoutSeconds < 1 || *in.RequestTimeoutSeconds > 3600) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request_timeout_seconds must be between 1 and 3600")
		return
	}
	if in.RetryCount != nil && (*in.RetryCount < 0 || *in.RetryCount > 10) {
		writeError(w, http.StatusBadRequest, "invalid_request", "retry_count must be between 0 and 10")
		return
	}
	if in.RetryStatusCodes != nil && !validStatusCodeSpec(*in.RetryStatusCodes) {
		writeError(w, http.StatusBadRequest, "invalid_request", "retry_status_codes must be comma-separated status codes or inclusive ranges between 100 and 599")
		return
	}
	if in.HealthCheckMode != nil && *in.HealthCheckMode != "off" && *in.HealthCheckMode != "scheduled_all" && *in.HealthCheckMode != "passive_recovery" {
		writeError(w, http.StatusBadRequest, "invalid_request", "health_check_mode must be off, scheduled_all, or passive_recovery")
		return
	}
	if in.HealthCheckIntervalMinutes != nil && (*in.HealthCheckIntervalMinutes < 1 || *in.HealthCheckIntervalMinutes > 1440) {
		writeError(w, http.StatusBadRequest, "invalid_request", "health_check_interval_minutes must be between 1 and 1440")
		return
	}
	if in.AutoDisableSlowSeconds != nil && (*in.AutoDisableSlowSeconds < 0 || *in.AutoDisableSlowSeconds > 600) {
		writeError(w, http.StatusBadRequest, "invalid_request", "auto_disable_slow_seconds must be between 0 and 600")
		return
	}
	if in.AutoDisableStatusCodes != nil && !validStatusCodeSpec(*in.AutoDisableStatusCodes) {
		writeError(w, http.StatusBadRequest, "invalid_request", "auto_disable_status_codes must be comma-separated status codes or inclusive ranges between 100 and 599")
		return
	}
	current, err := s.loadReliabilitySettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load reliability settings")
		return
	}
	if in.RequestTimeoutSeconds != nil {
		current.RequestTimeoutSeconds = *in.RequestTimeoutSeconds
	}
	if in.RetryCount != nil {
		current.RetryCount = *in.RetryCount
	}
	if in.RetryStatusCodes != nil {
		current.RetryStatusCodes = strings.TrimSpace(*in.RetryStatusCodes)
	}
	if in.HealthCheckMode != nil {
		current.HealthCheckMode = *in.HealthCheckMode
	}
	if in.HealthCheckIntervalMinutes != nil {
		current.HealthCheckIntervalMinutes = *in.HealthCheckIntervalMinutes
	}
	if in.HealthCheckAutoRecover != nil {
		current.HealthCheckAutoRecover = *in.HealthCheckAutoRecover
	}
	if in.HealthCheckChannelIDs != nil {
		current.HealthCheckChannelIDs = strings.TrimSpace(*in.HealthCheckChannelIDs)
	}
	if in.AutoDisableOnTestFailure != nil {
		current.AutoDisableOnTestFailure = *in.AutoDisableOnTestFailure
	}
	if in.AutoDisableSlowSeconds != nil {
		current.AutoDisableSlowSeconds = *in.AutoDisableSlowSeconds
	}
	if in.AutoDisableStatusCodes != nil {
		current.AutoDisableStatusCodes = strings.TrimSpace(*in.AutoDisableStatusCodes)
	}
	if in.AutoDisableKeywords != nil {
		current.AutoDisableKeywords = strings.TrimRight(*in.AutoDisableKeywords, "\n")
	}
	_, err = s.db.Exec(r.Context(), `update site_settings set request_timeout_seconds=$1,retry_count=$2,retry_status_codes=$3,health_check_mode=$4,health_check_interval_minutes=$5,health_check_auto_recover=$6,health_check_channel_ids=$7,auto_disable_on_test_failure=$8,auto_disable_slow_seconds=$9,auto_disable_status_codes=$10,auto_disable_keywords=$11,updated_at=now() where id=true`, current.RequestTimeoutSeconds, current.RetryCount, current.RetryStatusCodes, current.HealthCheckMode, current.HealthCheckIntervalMinutes, current.HealthCheckAutoRecover, current.HealthCheckChannelIDs, current.AutoDisableOnTestFailure, current.AutoDisableSlowSeconds, current.AutoDisableStatusCodes, current.AutoDisableKeywords)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save reliability settings")
		return
	}
	s.reliabilityData.clear()
	s.audit(r, "reliability.updated", "site_settings", "reliability", map[string]any{"request_timeout_seconds": current.RequestTimeoutSeconds, "retry_count": current.RetryCount, "health_check_mode": current.HealthCheckMode})
	writeJSON(w, http.StatusOK, s.reliabilitySettings(r.Context()))
}

// autoDisableChannel marks a channel or, when the failing key is known (multi-key
// channels), just that channel API key as automatically disabled with an audit
// record. A key is disabled only if the channel allows auto-disable; when the last
// enabled key of a channel is disabled the whole channel is disabled too.
func (s *Service) autoDisableChannel(ctx context.Context, channelID int64, keyID, reason string) {
	if keyID != "" {
		result, err := s.db.Exec(ctx, `update channel_api_keys set enabled=false,failure_count=0,last_error=$1,last_checked_at=now() where id=$2 and channel_id=$3 and enabled and exists(select 1 from channels c where c.id=channel_api_keys.channel_id and c.enabled and c.auto_disable)`, reason, keyID, channelID)
		if err != nil || result.RowsAffected() != 1 {
			return
		}
		s.syncChannelKeyType(ctx, strconv.FormatInt(channelID, 10))
		details, _ := json.Marshal(map[string]string{"reason": reason})
		auditID, _ := randomID()
		_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel_key.auto_disabled','system','channel_api_key',$2,$3,'SYSTEM','/gateway')`, auditID, keyID, details)
		s.disableChannelIfKeyless(ctx, channelID, reason)
		return
	}
	result, err := s.db.Exec(ctx, `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_error=$1,last_checked_at=now(),updated_at=now() where id=$2 and enabled and auto_disable`, reason, channelID)
	if err != nil || result.RowsAffected() != 1 {
		return
	}
	s.invalidateChannels()
	details, _ := json.Marshal(map[string]string{"reason": reason})
	auditID, _ := randomID()
	_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel.auto_disabled','system','channel',$2,$3,'SYSTEM','/system/channel-test')`, auditID, channelID, details)
}

// disableChannelIfKeyless auto-disables a channel when all of its API keys are
// disabled, so a channel left without usable credentials is retired as a whole.
func (s *Service) disableChannelIfKeyless(ctx context.Context, channelID int64, reason string) {
	var left int
	if err := s.db.QueryRow(ctx, `select count(*) from channel_api_keys where channel_id=$1 and enabled`, channelID).Scan(&left); err != nil || left > 0 {
		return
	}
	result, err := s.db.Exec(ctx, `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_error=$1,last_checked_at=now(),updated_at=now() where id=$2 and enabled and auto_disable`, reason, channelID)
	if err != nil || result.RowsAffected() != 1 {
		return
	}
	s.invalidateChannels()
	details, _ := json.Marshal(map[string]string{"reason": reason})
	auditID, _ := randomID()
	_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel.auto_disabled','system','channel',$2,$3,'SYSTEM','/system/channel-test')`, auditID, channelID, details)
}

// testChannel probes a channel and returns the status code and latency. When
// testModel is set it issues a real chat completion with that model; otherwise
// it falls back to GET /v1/models.
func (s *Service) testChannel(ctx context.Context, baseURL, apiKey, provider, testModel string, uaPool []string, timeout time.Duration) (int, []byte, time.Duration, error) {
	if timeout <= 0 {
		timeout = healthCheckProbeTimeout
	}
	path, method := "/v1/models", http.MethodGet
	if provider == "commandcode" {
		path = commandCodeModelsPath
	}
	var requestBody []byte
	if testModel != "" {
		payload := map[string]any{
			"model":      testModel,
			"max_tokens": 16,
			"messages":   []map[string]any{{"role": "user", "content": "ping"}},
		}
		requestBody, _ = json.Marshal(payload)
		switch provider {
		case "anthropic":
			path, method = "/v1/messages", http.MethodPost
		case "commandcode":
			path, method = commandCodeGeneratePath, http.MethodPost
			if ccBody, ccErr := commandCodeBodyFromOpenAI(requestBody, commandCodeWorkingDir); ccErr == nil {
				requestBody = ccBody
			}
		default:
			path, method = "/v1/chat/completions", http.MethodPost
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(requestBody))
	if err != nil {
		return 0, nil, 0, err
	}
	if testModel != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if provider == "anthropic" {
		request.Header.Set("X-API-Key", apiKey)
		request.Header.Set("Anthropic-Version", "2023-06-01")
	} else {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if provider == "commandcode" && testModel != "" {
		request.Header.Set("x-command-code-version", commandCodeCLIVersion)
		request.Header.Set("x-cli-environment", "production")
		request.Header.Set("x-project-slug", commandCodeProjectSlug(commandCodeWorkingDir))
		request.Header.Set("x-taste-learning", "true")
		request.Header.Set("x-co-flag", "false")
	}
	if ua := randomUA(uaPool); ua != "" {
		request.Header.Set("User-Agent", ua)
	}
	started := time.Now()
	response, err := clientWithTimeout(s.httpClient, timeout).Do(request)
	latency := time.Since(started)
	if err != nil {
		return 0, nil, latency, err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if readErr != nil {
		return 0, body, latency, readErr
	}
	return response.StatusCode, body, latency, nil
}

// runHealthChecks tests channels according to the configured mode. Multi-key
// channels are probed key by key, so a failed key is auto-disabled on its own
// instead of taking the whole channel down; channels without channel_api_keys
// rows fall back to the legacy api_key column and are handled as a whole.
func (s *Service) runHealthChecks(ctx context.Context) {
	settings := s.reliabilitySettings(ctx)
	if settings.HealthCheckMode == "off" {
		return
	}
	var query string
	var args []any
	if settings.HealthCheckMode == "scheduled_all" {
		// Scheduled full tests only probe channels not manually disabled and
		// only their enabled keys.
		query = `select c.id,c.base_url,c.provider,c.test_model,c.ua_pool,k.id,k.key_encrypted from channels c left join channel_api_keys k on k.channel_id=c.id and k.enabled where c.enabled and not c.auto_disabled order by c.id,k.priority,k.created_at`
		if len(settings.parsedChannelIDs) > 0 {
			ids := make([]string, 0, len(settings.parsedChannelIDs))
			for id := range settings.parsedChannelIDs {
				ids = append(ids, id)
			}
			query = `select c.id,c.base_url,c.provider,c.test_model,c.ua_pool,k.id,k.key_encrypted from channels c left join channel_api_keys k on k.channel_id=c.id and k.enabled where c.enabled and not c.auto_disabled and c.id::text = any($1) order by c.id,k.priority,k.created_at`
			args = append(args, ids)
		}
	} else {
		// Passive recovery checks channels auto-disabled by failed real requests
		// and individual keys auto-disabled while their channel stayed enabled.
		query = `select c.id,c.base_url,c.provider,c.test_model,c.ua_pool,k.id,k.key_encrypted from channels c left join channel_api_keys k on k.channel_id=c.id where (not c.enabled and c.auto_disabled) or (c.enabled and k.enabled=false) order by c.id,k.priority,k.created_at`
		if len(settings.parsedChannelIDs) > 0 {
			ids := make([]string, 0, len(settings.parsedChannelIDs))
			for id := range settings.parsedChannelIDs {
				ids = append(ids, id)
			}
			query = `select c.id,c.base_url,c.provider,c.test_model,c.ua_pool,k.id,k.key_encrypted from channels c left join channel_api_keys k on k.channel_id=c.id where ((not c.enabled and c.auto_disabled) or (c.enabled and k.enabled=false)) and c.id::text = any($1) order by c.id,k.priority,k.created_at`
			args = append(args, ids)
		}
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return
	}
	type target struct {
		id                                  int64
		baseURL, provider, testModel, keyID string
		uaPool                              []string
		encrypted                           string
		hasKey                              bool
	}
	var targets []target
	for rows.Next() {
		var t target
		var keyID, encrypted *string
		var uaPool []byte
		if rows.Scan(&t.id, &t.baseURL, &t.provider, &t.testModel, &uaPool, &keyID, &encrypted) == nil {
			t.uaPool = parsedUAPool(uaPool)
			t.hasKey = keyID != nil
			if t.hasKey {
				t.keyID = *keyID
				t.encrypted = *encrypted
			}
			targets = append(targets, t)
		}
	}
	rows.Close()
	for _, t := range targets {
		var apiKey string
		if t.hasKey {
			apiKey, err = channelKeyValue(s.cfg.EncryptionKey, t.encrypted)
		} else {
			var legacy string
			if qerr := s.db.QueryRow(ctx, `select api_key from channels where id=$1`, t.id).Scan(&legacy); qerr != nil {
				continue
			}
			apiKey, err = channelKeyValue(s.cfg.EncryptionKey, legacy)
		}
		if err != nil {
			continue
		}
		status, _, latency, testErr := s.testChannel(ctx, t.baseURL, apiKey, t.provider, t.testModel, t.uaPool, healthCheckProbeTimeout)
		success := testErr == nil && status >= 200 && status < 300
		if success {
			if settings.HealthCheckAutoRecover {
				if t.hasKey {
					_, _ = s.db.Exec(ctx, `update channel_api_keys set enabled=true,failure_count=0,last_error=null,last_checked_at=now() where id=$1 and channel_id=$2`, t.keyID, t.id)
				}
				_, _ = s.db.Exec(ctx, `update channels set enabled=true,auto_disabled=false,disabled_reason='',failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1`, t.id)
				s.invalidateChannels()
			} else if t.hasKey {
				_, _ = s.db.Exec(ctx, `update channel_api_keys set last_checked_at=now() where id=$1`, t.keyID)
			} else {
				_, _ = s.db.Exec(ctx, `update channels set last_checked_at=now() where id=$1`, t.id)
			}
			continue
		}
		reason := healthFailureReason(status, testErr)
		if t.hasKey {
			_, _ = s.db.Exec(ctx, `update channel_api_keys set last_checked_at=now(),last_error=$2 where id=$1 and channel_id=$3`, t.keyID, reason, t.id)
		} else {
			_, _ = s.db.Exec(ctx, `update channels set last_checked_at=now(),last_error=$2 where id=$1`, t.id, reason)
		}
		if settings.HealthCheckMode == "scheduled_all" && settings.AutoDisableOnTestFailure {
			if settings.AutoDisableSlowSeconds > 0 && latency > time.Duration(settings.AutoDisableSlowSeconds)*time.Second {
				reason = "health_check_slow_response"
			}
			s.autoDisableChannel(ctx, t.id, t.keyID, reason)
		}
	}
}

func healthFailureReason(status int, err error) string {
	if err != nil {
		return "health_check_unreachable"
	}
	return "health_check_status_" + strconv.Itoa(status)
}

// startHealthCheckScheduler periodically runs channel health checks.
func (s *Service) startHealthCheckScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		lastRun := time.Time{}
		lastInterval := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				settings := s.reliabilitySettings(ctx)
				if settings.HealthCheckMode == "off" {
					lastRun = time.Time{}
					continue
				}
				if lastInterval != settings.HealthCheckIntervalMinutes {
					lastRun = time.Time{}
					lastInterval = settings.HealthCheckIntervalMinutes
				}
				if time.Since(lastRun) < time.Duration(settings.HealthCheckIntervalMinutes)*time.Minute {
					continue
				}
				lastRun = time.Now()
				s.runHealthChecks(ctx)
			}
		}
	}()
}
