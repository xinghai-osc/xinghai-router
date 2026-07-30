package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxGatewayMaxTokens     = 200_000
	defaultGatewayMaxTokens = 4096
	maxUpstreamResponseBody = 16 << 20
	// settlementTimeout bounds the detached wallet/log writes that must complete even
	// after the client has disconnected.
	settlementTimeout = 15 * time.Second
)

// streamBufferPool recycles the copy buffers used to relay SSE responses.
var streamBufferPool = sync.Pool{New: func() any {
	buf := make([]byte, 32*1024)
	return &buf
}}

type channel struct {
	id, baseURL, apiKey, upstreamModel, provider string
	priority, weight                             int
}

type reservation struct{ amount float64 }

func validGatewayMaxTokens(maxTokens int) bool {
	return maxTokens > 0 && maxTokens <= maxGatewayMaxTokens
}

func resolveGatewayMaxTokens(maxTokens int) (int, bool) {
	if maxTokens > maxGatewayMaxTokens {
		return 0, false
	}
	if maxTokens <= 0 {
		return defaultGatewayMaxTokens, true
	}
	return maxTokens, true
}

func (s *Service) models(w http.ResponseWriter, r *http.Request) {
	key := r.Context().Value(contextKey{}).(keyContext)
	rows, err := s.db.Query(r.Context(), `select model from (select jsonb_array_elements_text(c.models) as model from channels c where c.enabled and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($2<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid)) or ($2='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1))) union select m.public_model as model from model_routes m join channels c on c.id=m.channel_id where m.enabled and not m.hidden and c.enabled and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($2<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid)) or ($2='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1)))) available order by model`, key.userID, key.groupID)
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
		Model     string `json:"model"`
		Stream    bool   `json:"stream"`
		MaxTokens int    `json:"max_tokens"`
	}
	if json.Unmarshal(body, &request) != nil {
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if !validModelName(request.Model) {
		writeError(w, 400, "invalid_request", "model must be 1-200 characters")
		return
	}
	if request.MaxTokens > maxGatewayMaxTokens {
		writeError(w, 400, "invalid_request", "max_tokens must be at most 200000")
		return
	}
	s.proxyChatCompletions(w, r, body, request.Model, request.Stream, request.MaxTokens, nil, nil)
}

type responseTransform func([]byte) ([]byte, error)
type streamTransform func(http.ResponseWriter, *http.Response) error

func (s *Service) proxyChatCompletions(w http.ResponseWriter, r *http.Request, body []byte, model string, stream bool, maxTokens int, transform responseTransform, streamFn streamTransform) {
	started := time.Now()
	ctx := r.Context()
	key := ctx.Value(contextKey{}).(keyContext)
	if maxTokens > maxGatewayMaxTokens {
		writeError(w, 400, "invalid_request", "max_tokens must be at most 200000")
		return
	}
	if err := s.checkQuota(ctx, key, model); err != nil {
		writeError(w, 429, "quota_exceeded", "request quota exceeded")
		return
	}
	// Pricing and the group multiplier are read once and reused by reservation and
	// settlement, which previously repeated the same two lookups.
	pricing := s.pricingFor(ctx, model)
	groupMultiplier := s.groupMultiplierFor(ctx, key.groupID)
	subscriptionAccess := s.subscriptionCoversModel(ctx, key.userID, model)
	var reserved reservation
	// Streaming responses are not settled; do not hold wallet reserved balance for them.
	if subscriptionAccess || stream {
		if !subscriptionAccess && stream && !pricing.found {
			writeError(w, 402, "pricing_unavailable", "no enabled pricing rule for this model")
			return
		}
	} else {
		var err error
		reserved, err = s.reserveUsage(ctx, key, model, len(body), maxTokens, pricing, groupMultiplier)
		if err != nil {
			if errors.Is(err, errPricingUnavailable) {
				writeError(w, 402, "pricing_unavailable", "no enabled pricing rule for this model")
				return
			}
			writeError(w, 402, "insufficient_quota", "insufficient balance for this request")
			return
		}
	}
	defer func() { s.releaseReservation(ctx, key, reserved) }()
	channels, err := s.channelsForModel(ctx, key, model)
	if err != nil {
		s.logRequest(ctx, key, "", model, 503, 0, 0, 0, time.Since(started), "no_channel")
		writeError(w, 503, "model_unavailable", "no enabled channel supports this model")
		return
	}
	reliability := s.reliabilitySettings(ctx)
	maxAttempts := reliability.RetryCount + 1
	if maxAttempts > len(channels) {
		maxAttempts = len(channels)
	}
	client := s.httpClient
	if stream && s.streamClient != nil {
		client = s.streamClient
	}
	accept := "application/json"
	if stream {
		accept = "text/event-stream"
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
		upstreamReq.Header.Set("Accept", accept)
		resp, err = client.Do(upstreamReq)
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
					s.autoDisableChannel(ctx, ch.id, failureReason)
				}
			}
			resp = nil
		}
		s.channelFailed(ctx, ch.id, failureReason)
		if attempts >= maxAttempts {
			break
		}
	}
	if resp == nil {
		s.logRequest(ctx, key, ch.id, model, 502, 0, 0, 0, time.Since(started), "upstream_unreachable")
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
		s.logRequest(ctx, key, ch.id, model, resp.StatusCode, 0, 0, 0, time.Since(started), "")
		s.channelSucceeded(ctx, ch.id)
		return
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBody))
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
	s.logRequest(ctx, key, ch.id, model, resp.StatusCode, prompt, completion, total, time.Since(started), errorCode(resp.StatusCode))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if !subscriptionAccess {
			reserved = s.settleUsage(ctx, key, reserved, model, prompt, completion, pricing, groupMultiplier)
		}
		s.channelSucceeded(ctx, ch.id)
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

// reserveUsage holds the worst-case cost of a request before it is proxied. The whole
// hold — balance check, reserve update and ledger entry — is a single atomic statement:
// PostgreSQL re-evaluates the WHERE clause after taking the row lock, so concurrent
// requests for the same wallet still cannot overspend.
func (s *Service) reserveUsage(ctx context.Context, key keyContext, model string, bodyLen, maxTokens int, pricing pricingRule, groupMultiplier float64) (reservation, error) {
	resolved, ok := resolveGatewayMaxTokens(maxTokens)
	if !ok {
		return reservation{}, errInvalid
	}
	if !pricing.found {
		return reservation{}, errPricingUnavailable
	}
	// Reserve the configured maximum output plus a conservative request-body estimate.
	amount := (float64(bodyLen/3)*pricing.input + float64(resolved)*pricing.output) / 1000000 * pricing.multiplier * groupMultiplier
	if amount == 0 {
		// Zero list prices are allowed only when an explicit enabled rule exists.
		return reservation{}, nil
	}
	id, _ := randomID()
	tag, err := s.db.Exec(ctx, `with held as (
		update user_wallets set reserved=reserved+$1,updated_at=now()
		where user_id=$2 and balance-reserved >= $1
		returning balance
	)
	insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note)
	select $3::uuid,$2,-$1,balance,'reservation',$4::text,$5::text from held`, amount, key.userID, id, requestID(ctx), model)
	if err != nil {
		return reservation{}, err
	}
	if tag.RowsAffected() == 0 {
		return reservation{}, errInvalid
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

// settleUsage charges the actual cost and releases the hold. Balance update, ledger
// entry and usage record are one statement so they commit or fail together, without the
// five extra round trips an explicit transaction needed.
func (s *Service) settleUsage(ctx context.Context, key keyContext, held reservation, model string, prompt, completion int, pricing pricingRule, groupMultiplier float64) reservation {
	if held.amount == 0 && prompt == 0 && completion == 0 {
		return held
	}
	cost := clampCostToHold(usageCost(prompt, completion, pricing.input, pricing.output, pricing.multiplier, groupMultiplier), held.amount)
	ledgerID, _ := randomID()
	usageID, _ := randomID()
	settleCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	tag, err := s.db.Exec(settleCtx, `with settled as (
		update user_wallets set balance=balance-$1, reserved=greatest(0,reserved-$2), updated_at=now()
		where user_id=$3
		returning balance
	), ledger as (
		insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note)
		select $4::uuid,$3,-$1,balance,'charge',$5::text,$6::text from settled
	)
	insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,completion_tokens,cost)
	select $7::uuid,$5::text,$3,$8::uuid,$6::text,$9::int,$10::int,$1 from settled
	on conflict(request_id) do update set prompt_tokens=excluded.prompt_tokens,completion_tokens=excluded.completion_tokens,cost=excluded.cost`,
		cost, held.amount, key.userID, ledgerID, requestID(ctx), model, usageID, key.keyID, prompt, completion)
	if err != nil || tag.RowsAffected() == 0 {
		return held
	}
	return reservation{}
}

func (s *Service) releaseReservation(ctx context.Context, key keyContext, held reservation) {
	if held.amount == 0 {
		return
	}
	// The client may already have hung up; the hold must be released regardless.
	releaseCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, _ = s.db.Exec(releaseCtx, `update user_wallets set reserved=greatest(0,reserved-$1),updated_at=now() where user_id=$2`, held.amount, key.userID)
}

// checkQuota evaluates every matching quota row in one query. Each row's usage window is
// aggregated by a lateral join instead of a follow-up query per row.
func (s *Service) checkQuota(ctx context.Context, key keyContext, model string) error {
	rows, err := s.db.Query(ctx, `select q.max_requests,q.max_tokens,agg.requests,agg.tokens
	from quota_limits q
	cross join lateral (
		select count(*) as requests, coalesce(sum(rl.total_tokens),0) as tokens
		from request_logs rl
		where rl.api_key_id=$2 and rl.created_at >= now() - ('1 '||q."window")::interval
	) agg
	where (q.user_id=$1 or q.user_id is null) and (q.api_key_id=$2 or q.api_key_id is null) and (q.model=$3 or q.model is null) and (q.max_requests is not null or q.max_tokens is not null)`, key.userID, key.keyID, model)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var maxRequests, maxTokens *int64
		var count, tokens int64
		if rows.Scan(&maxRequests, &maxTokens, &count, &tokens) != nil {
			return errInvalid
		}
		if (maxRequests != nil && count >= *maxRequests) || (maxTokens != nil && tokens >= *maxTokens) {
			return errInvalid
		}
	}
	return rows.Err()
}
func (s *Service) channelsForModel(ctx context.Context, key keyContext, model string) ([]channel, error) {
	rows, err := s.db.Query(ctx, `select c.id,c.base_url,c.api_key,coalesce(m.priority,c.priority),coalesce(m.weight,c.weight),coalesce(m.upstream_model,''),c.provider from channels c left join model_routes m on m.channel_id=c.id and m.public_model=$1 and m.enabled where c.enabled and (c.cooldown_until is null or c.cooldown_until<=now()) and (c.models ? $1 or m.public_model is not null) and (not exists(select 1 from channel_groups cg where cg.channel_id=c.id) or ($3<>'' and exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($3,'')::uuid)) or ($3='' and exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$2))) order by coalesce(m.priority,c.priority), c.priority, c.id`, model, key.userID, key.groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []channel
	seed := sha256.Sum256([]byte(requestID(ctx)))
	for rows.Next() {
		var ch channel
		var encrypted string
		if err := rows.Scan(&ch.id, &ch.baseURL, &encrypted, &ch.priority, &ch.weight, &ch.upstreamModel, &ch.provider); err != nil {
			return nil, err
		}
		ch.apiKey, err = s.selectChannelKey(ctx, ch.id, encrypted, seed[:])
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

func (s *Service) selectChannelKey(ctx context.Context, channelID, fallbackEncrypted string, seed []byte) (string, error) {
	krows, err := s.db.Query(ctx, `select key_encrypted from channel_api_keys where channel_id=$1 and enabled order by created_at`, channelID)
	if err != nil {
		return "", err
	}
	defer krows.Close()
	var keys []string
	for krows.Next() {
		var enc string
		if krows.Scan(&enc) != nil {
			continue
		}
		key, err := crypt(s.cfg.EncryptionKey, enc, true)
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) > 0 {
		pick := int(seed[0]) % len(keys)
		return keys[pick], nil
	}
	if fallbackEncrypted != "" {
		return crypt(s.cfg.EncryptionKey, fallbackEncrypted, true)
	}
	return "", errInvalid
}

// channelSucceeded clears failure bookkeeping in the background. The WHERE clause makes
// the common case (an already-healthy channel checked recently) touch no rows, which
// keeps a shared channel row from becoming a write hotspot under concurrent traffic.
func (s *Service) channelSucceeded(ctx context.Context, id string) {
	s.background.submit(func(ctx context.Context) {
		_, _ = s.db.Exec(ctx, `update channels set failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1 and (failure_count<>0 or cooldown_until is not null or last_error is not null or last_checked_at is null or last_checked_at < now()-interval '30 seconds')`, id)
	})
}
func (s *Service) channelFailed(ctx context.Context, id, reason string) {
	var failureCount int
	err := s.db.QueryRow(ctx, `update channels set failure_count=failure_count+1,cooldown_until=case when failure_count+1 >= 3 then now()+interval '1 minute' else cooldown_until end,last_error=$2,last_checked_at=now(),updated_at=now() where id=$1 returning failure_count`, id, reason).Scan(&failureCount)
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
	seed := sha256.Sum256([]byte(id + "test"))
	apiKey, err := s.selectChannelKey(ctx, id, encrypted, seed[:])
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
	buf := streamBufferPool.Get().(*[]byte)
	defer streamBufferPool.Put(buf)
	for {
		n, err := resp.Body.Read(*buf)
		if n > 0 {
			if _, writeErr := w.Write((*buf)[:n]); writeErr != nil {
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

// logRequest writes the audit/quota trail. It stays synchronous because usage_records
// has a foreign key onto request_logs, but runs on a detached context so a client that
// hangs up mid-stream does not silently lose its log row (and its quota accounting).
// The api_keys.last_used_at stamp it used to duplicate is now handled by the api
// middleware.
func (s *Service) logRequest(ctx context.Context, key keyContext, channelID, model string, status, prompt, completion, total int, d time.Duration, errorCode string) {
	id, _ := randomID()
	logCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, _ = s.db.Exec(logCtx, `insert into request_logs(id,request_id,user_id,api_key_id,channel_id,group_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code) values($1,$2,$3,$4,nullif($5,'')::uuid,nullif($6,'')::uuid,$7,$8,$9,$10,$11,$12,nullif($13,''))`, id, requestID(ctx), key.userID, key.keyID, channelID, key.groupID, model, status, prompt, completion, total, d.Milliseconds(), errorCode)
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
