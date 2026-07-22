package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type channel struct {
	id, baseURL, apiKey, upstreamModel, provider string
	priority, weight                             int
}

type reservation struct{ amount float64 }

func (s *Service) groupMultiplier(r *http.Request, key keyContext) float64 {
	if key.groupID == "" {
		return 1
	}
	var multiplier float64
	if err := s.db.QueryRow(r.Context(), `select multiplier from groups where id=$1`, key.groupID).Scan(&multiplier); err != nil || multiplier <= 0 {
		return 1
	}
	return multiplier
}

func (s *Service) models(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select model from (select jsonb_array_elements_text(c.models) as model from channels c where c.enabled and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($2<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid)) or ($2='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1))) union select m.public_model as model from model_routes m join channels c on c.id=m.channel_id where m.enabled and c.enabled and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($2<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid)) or ($2='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1)))) available order by model`, key.userID, key.groupID)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	seen := map[string]bool{}
	data := []map[string]any{}
	for rows.Next() {
		var model string
		if rows.Scan(&model) != nil {
			continue
		}
		if !seen[model] {
			seen[model] = true
			data = append(data, map[string]any{"id": model, "object": "model", "created": 0, "owned_by": "xinghai"})
		}
	}
	writeJSON(w, 200, map[string]any{"object": "list", "data": data})
}

func (s *Service) chatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, 400, "invalid_request", "could not read request")
		return
	}
	var request struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if json.Unmarshal(body, &request) != nil || request.Model == "" {
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	s.proxyChatCompletions(w, r, body, request.Model, request.Stream, nil, nil)
}

type responseTransform func([]byte) ([]byte, error)
type streamTransform func(http.ResponseWriter, *http.Response) error

func (s *Service) proxyChatCompletions(w http.ResponseWriter, r *http.Request, body []byte, model string, stream bool, transform responseTransform, streamFn streamTransform) {
	started := time.Now()
	key := r.Context().Value(contextKey{}).(keyContext)
	if err := s.checkQuota(r, key, model); err != nil {
		writeError(w, 429, "quota_exceeded", "request quota exceeded")
		return
	}
	subscriptionAccess := s.subscriptionCoversModel(r.Context(), key.userID, model)
	reserved, err := s.reserveUsage(r, key, model, body)
	if err != nil {
		if subscriptionAccess {
			reserved = reservation{}
		} else {
			writeError(w, 402, "insufficient_quota", "insufficient balance for this request")
			return
		}
	}
	defer func() { s.releaseReservation(r, key, reserved, model) }()
	channels, err := s.channelsForModel(r, model)
	if err != nil {
		s.logRequest(r, key, "", model, 503, 0, 0, 0, time.Since(started), "no_channel")
		writeError(w, 503, "model_unavailable", "no enabled channel supports this model")
		return
	}
	reliability := s.reliabilitySettings(r.Context())
	maxAttempts := reliability.RetryCount + 1
	if maxAttempts > len(channels) {
		maxAttempts = len(channels)
	}
	var resp *http.Response
	var ch channel
	attempts := 0
	for _, candidate := range channels {
		if attempts >= maxAttempts {
			break
		}
		attempts++
		ch = candidate
		upstreamURL := ch.baseURL + "/v1/chat/completions"
		upstreamBody := body
		if ch.upstreamModel != "" && ch.upstreamModel != model {
			var payload map[string]any
			if json.Unmarshal(body, &payload) == nil {
				payload["model"] = ch.upstreamModel
				upstreamBody, _ = json.Marshal(payload)
			}
		}
		if ch.provider == "anthropic" {
			upstreamURL = ch.baseURL + "/v1/messages"
			upstreamBody, err = openAIRequestToAnthropic(upstreamBody)
			if err != nil {
				continue
			}
		}
		upstreamReq, requestErr := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
		if requestErr != nil {
			continue
		}
		if ch.provider == "anthropic" {
			upstreamReq.Header.Set("X-API-Key", ch.apiKey)
			upstreamReq.Header.Set("Anthropic-Version", "2023-06-01")
		} else {
			upstreamReq.Header.Set("Authorization", "Bearer "+ch.apiKey)
		}
		upstreamReq.Header.Set("Content-Type", "application/json")
		upstreamReq.Header.Set("Accept", map[bool]string{true: "text/event-stream", false: "application/json"}[stream])
		resp, err = s.httpClient.Do(upstreamReq)
		if err == nil && !reliability.retryable(resp.StatusCode) {
			break
		}
		failureReason := "upstream_unreachable"
		if err == nil {
			failureReason = "upstream_status_" + strconv.Itoa(resp.StatusCode)
			// Apply auto-disable rules to the upstream error body before retrying.
			bodyPeek, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			resp.Body.Close()
			if readErr == nil {
				if reliability.autoDisableStatus(resp.StatusCode) || reliability.autoDisableKeyword(string(bodyPeek)) {
					s.autoDisableChannel(r.Context(), ch.id, failureReason)
				}
			}
			resp = nil
		}
		s.channelFailed(r, ch.id, failureReason)
		if attempts >= maxAttempts {
			break
		}
	}
	if resp == nil {
		s.logRequest(r, key, ch.id, model, 502, 0, 0, 0, time.Since(started), "upstream_unreachable")
		writeError(w, 502, "upstream_error", "all upstream channels failed")
		return
	}
	defer resp.Body.Close()
	if stream && resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		if ch.provider == "anthropic" && streamFn == nil {
			_ = streamAnthropicToOpenAI(w, resp)
		} else if streamFn != nil && ch.provider != "anthropic" {
			_ = streamFn(w, resp)
		} else {
			s.streamResponse(w, resp)
		}
		s.logRequest(r, key, ch.id, model, resp.StatusCode, 0, 0, 0, time.Since(started), "")
		s.channelSucceeded(r, ch.id)
		return
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, 502, "upstream_error", "could not read upstream response")
		return
	}
	if ch.provider == "anthropic" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		responseBody, err = anthropicResponseToOpenAI(responseBody)
		if err != nil {
			writeError(w, 502, "upstream_error", "could not convert upstream response")
			return
		}
	}
	prompt, completion, total := usage(responseBody)
	s.logRequest(r, key, ch.id, model, resp.StatusCode, prompt, completion, total, time.Since(started), errorCode(resp.StatusCode))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if !subscriptionAccess {
			reserved = s.settleUsage(r, key, reserved, model, prompt, completion)
		}
		s.channelSucceeded(r, ch.id)
		if transform != nil {
			responseBody, err = transform(responseBody)
			if err != nil {
				writeError(w, 502, "upstream_error", "could not convert upstream response")
				return
			}
		}
	}
	w.Header().Set("Content-Type", contentType(resp.Header.Get("Content-Type")))
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)
}

func (s *Service) reserveUsage(r *http.Request, key keyContext, model string, body []byte) (reservation, error) {
	var request struct {
		MaxTokens int `json:"max_tokens"`
	}
	_ = json.Unmarshal(body, &request)
	if request.MaxTokens <= 0 {
		request.MaxTokens = 4096
	}
	var input, output, multiplier float64
	if err := s.db.QueryRow(r.Context(), `select input_per_million,output_per_million,multiplier from pricing_rules where model=$1 and enabled`, model).Scan(&input, &output, &multiplier); err != nil {
		return reservation{}, nil
	}
	// Reserve the configured maximum output plus a conservative request-body estimate.
	amount := (float64(len(body)/3)*input + float64(request.MaxTokens)*output) / 1000000 * multiplier * s.groupMultiplier(r, key)
	if amount == 0 {
		return reservation{}, nil
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return reservation{}, err
	}
	defer tx.Rollback(r.Context())
	var balance, held float64
	if err = tx.QueryRow(r.Context(), `select balance,reserved from user_wallets where user_id=$1 for update`, key.userID).Scan(&balance, &held); err != nil || balance-held < amount {
		return reservation{}, errInvalid
	}
	if _, err = tx.Exec(r.Context(), `update user_wallets set reserved=reserved+$1,updated_at=now() where user_id=$2`, amount, key.userID); err != nil {
		return reservation{}, err
	}
	id, _ := randomID()
	if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,'reservation',$5,$6)`, id, key.userID, -amount, balance, requestID(r.Context()), model); err != nil {
		return reservation{}, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return reservation{}, err
	}
	return reservation{amount: amount}, nil
}

func usageCost(prompt, completion int, input, output, multiplier, groupMultiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	if groupMultiplier <= 0 {
		groupMultiplier = 1
	}
	return (float64(prompt)*input + float64(completion)*output) / 1000000 * multiplier * groupMultiplier
}

func clampCostToHold(cost, held float64) float64 {
	if cost < 0 {
		return 0
	}
	if held > 0 && cost > held {
		return held
	}
	return cost
}

func (s *Service) settleUsage(r *http.Request, key keyContext, held reservation, model string, prompt, completion int) reservation {
	if held.amount == 0 && prompt == 0 && completion == 0 {
		return held
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return held
	}
	defer tx.Rollback(r.Context())
	var input, cached, output, multiplier float64
	_ = tx.QueryRow(r.Context(), `select input_per_million,cached_input_per_million,output_per_million,multiplier from pricing_rules where model=$1 and enabled`, model).Scan(&input, &cached, &output, &multiplier)
	cost := clampCostToHold(usageCost(prompt, completion, input, output, multiplier, s.groupMultiplier(r, key)), held.amount)
	var balance float64
	if err = tx.QueryRow(r.Context(), `select balance from user_wallets where user_id=$1 for update`, key.userID).Scan(&balance); err != nil {
		return held
	}
	id, _ := randomID()
	requestID := requestID(r.Context())
	if _, err = tx.Exec(r.Context(), `update user_wallets set balance=balance-$1, reserved=greatest(0,reserved-$2), updated_at=now() where user_id=$3`, cost, held.amount, key.userID); err != nil {
		return held
	}
	var after float64
	if tx.QueryRow(r.Context(), `select balance from user_wallets where user_id=$1`, key.userID).Scan(&after) != nil {
		return held
	}
	if _, err = tx.Exec(r.Context(), `insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note) values($1,$2,$3,$4,'charge',$5,$6)`, id, key.userID, -cost, after, requestID, model); err != nil {
		return held
	}
	usageID, _ := randomID()
	if _, err = tx.Exec(r.Context(), `insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,completion_tokens,cost) values($1,$2,$3,$4,$5,$6,$7,$8) on conflict(request_id) do update set prompt_tokens=excluded.prompt_tokens,completion_tokens=excluded.completion_tokens,cost=excluded.cost`, usageID, requestID, key.userID, key.keyID, model, prompt, completion, cost); err != nil {
		return held
	}
	if err = tx.Commit(r.Context()); err != nil {
		return held
	}
	return reservation{}
}

func (s *Service) releaseReservation(r *http.Request, key keyContext, held reservation, model string) {
	if held.amount == 0 {
		return
	}
	_, _ = s.db.Exec(r.Context(), `update user_wallets set reserved=greatest(0,reserved-$1),updated_at=now() where user_id=$2`, held.amount, key.userID)
}

func (s *Service) checkQuota(r *http.Request, key keyContext, model string) error {
	rows, err := s.db.Query(r.Context(), `select "window",max_requests,max_tokens from quota_limits where (user_id=$1 or user_id is null) and (api_key_id=$2 or api_key_id is null) and (model=$3 or model is null) and (max_requests is not null or max_tokens is not null)`, key.userID, key.keyID, model)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var window string
		var maxRequests, maxTokens *int64
		if rows.Scan(&window, &maxRequests, &maxTokens) != nil {
			return errInvalid
		}
		interval := map[string]string{"minute": "1 minute", "day": "1 day", "month": "1 month"}[window]
		var count, tokens int64
		if s.db.QueryRow(r.Context(), `select count(*),coalesce(sum(total_tokens),0) from request_logs where api_key_id=$1 and created_at >= now() - $2::interval`, key.keyID, interval).Scan(&count, &tokens) != nil {
			return errInvalid
		}
		if (maxRequests != nil && count >= *maxRequests) || (maxTokens != nil && tokens >= *maxTokens) {
			return errInvalid
		}
	}
	return rows.Err()
}
func (s *Service) channelsForModel(r *http.Request, model string) ([]channel, error) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select c.id,c.base_url,c.api_key,coalesce(m.priority,c.priority),coalesce(m.weight,c.weight),coalesce(m.upstream_model,''),c.provider from channels c left join model_routes m on m.channel_id=c.id and m.public_model=$1 and m.enabled where c.enabled and (c.cooldown_until is null or c.cooldown_until<=now()) and (c.models ? $1 or m.public_model is not null) and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($3<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($3,'')::uuid)) or ($3='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$2))) order by coalesce(m.priority,c.priority), c.priority, c.id`, model, key.userID, key.groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []channel
	for rows.Next() {
		var ch channel
		var encrypted string
		if err := rows.Scan(&ch.id, &ch.baseURL, &encrypted, &ch.priority, &ch.weight, &ch.upstreamModel, &ch.provider); err != nil {
			return nil, err
		}
		ch.apiKey, err = crypt(s.cfg.EncryptionKey, encrypted, true)
		if err != nil {
			continue
		}
		result = append(result, ch)
	}
	if rows.Err() != nil || len(result) == 0 {
		return nil, errInvalid
	}
	priority := result[0].priority
	end := 0
	for end < len(result) && result[end].priority == priority {
		end++
	}
	if end > 1 {
		sum := 0
		for _, ch := range result[:end] {
			sum += ch.weight
		}
		seed := sha256.Sum256([]byte(requestID(r.Context())))
		pick := int(seed[0])<<8 | int(seed[1])
		pick %= sum
		selected := 0
		for i, ch := range result[:end] {
			pick -= ch.weight
			if pick < 0 {
				selected = i
				break
			}
		}
		result[0], result[selected] = result[selected], result[0]
	}
	return result, nil
}
func (s *Service) channelSucceeded(r *http.Request, id string) {
	_, _ = s.db.Exec(r.Context(), `update channels set failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1`, id)
}
func (s *Service) channelFailed(r *http.Request, id, reason string) {
	var failureCount int
	err := s.db.QueryRow(r.Context(), `update channels set failure_count=failure_count+1,cooldown_until=case when failure_count+1 >= 3 then now()+interval '1 minute' else cooldown_until end,last_error=$2,last_checked_at=now(),updated_at=now() where id=$1 returning failure_count`, id, reason).Scan(&failureCount)
	if err == nil && failureCount == 3 {
		go s.testFailedChannel(id)
	}
}

// testFailedChannel verifies a newly unhealthy channel outside the client request.
func (s *Service) testFailedChannel(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()
	var baseURL, encrypted, provider string
	var enabled, autoDisable bool
	if err := s.db.QueryRow(ctx, `select c.base_url,c.api_key,c.provider,c.enabled,ss.auto_disable_failed_channels from channels c cross join site_settings ss where c.id=$1 and ss.id=true`, id).Scan(&baseURL, &encrypted, &provider, &enabled, &autoDisable); err != nil || !enabled || !autoDisable {
		return
	}
	apiKey, err := crypt(s.cfg.EncryptionKey, encrypted, true)
	if err != nil {
		s.disableFailedChannel(ctx, id, "credential_decryption_failed")
		return
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
		if err != nil {
			s.disableFailedChannel(ctx, id, "invalid_test_request")
			return
		}
		if provider == "anthropic" {
			request.Header.Set("X-API-Key", apiKey)
			request.Header.Set("Anthropic-Version", "2023-06-01")
		} else {
			request.Header.Set("Authorization", "Bearer "+apiKey)
		}
		response, err := s.httpClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				_, _ = s.db.Exec(ctx, `update channels set failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1 and enabled`, id)
				return
			}
		}
	}
	s.disableFailedChannel(ctx, id, "system_test_failed")
}

func (s *Service) disableFailedChannel(ctx context.Context, id, reason string) {
	result, err := s.db.Exec(ctx, `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_error=$1,last_checked_at=now(),updated_at=now() where id=$2 and enabled and failure_count>=3`, reason, id)
	if err != nil || result.RowsAffected() != 1 {
		return
	}
	details, _ := json.Marshal(map[string]string{"reason": reason})
	auditID, _ := randomID()
	_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel.auto_disabled','system','channel',$2,$3,'SYSTEM','/system/channel-test')`, auditID, id, details)
}
func retryableStatus(status int) bool {
	settings := defaultReliabilitySettings()
	return settings.retryable(status)
}
func (s *Service) streamResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "internal_error", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", contentType(resp.Header.Get("Content-Type")))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			flusher.Flush()
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}
func (s *Service) logRequest(r *http.Request, key keyContext, channelID, model string, status, prompt, completion, total int, d time.Duration, errorCode string) {
	id, _ := randomID()
	_, _ = s.db.Exec(r.Context(), `insert into request_logs(id,request_id,user_id,api_key_id,channel_id,group_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code) values($1,$2,$3,$4,nullif($5,'')::uuid,nullif($6,'')::uuid,$7,$8,$9,$10,$11,$12,nullif($13,''))`, id, requestID(r.Context()), key.userID, key.keyID, channelID, key.groupID, model, status, prompt, completion, total, d.Milliseconds(), errorCode)
	_, _ = s.db.Exec(r.Context(), `update api_keys set last_used_at=now() where id=$1`, key.keyID)
}
func usage(body []byte) (int, int, int) {
	var v struct {
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
			Total      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &v) != nil {
		return 0, 0, 0
	}
	return v.Usage.Prompt, v.Usage.Completion, v.Usage.Total
}
func errorCode(status int) string {
	if status >= 400 {
		return "upstream_" + http.StatusText(status)
	}
	return ""
}
func contentType(value string) string {
	if strings.HasPrefix(value, "application/json") {
		return "application/json"
	}
	if strings.HasPrefix(value, "text/event-stream") {
		return "text/event-stream"
	}
	return "application/json"
}
