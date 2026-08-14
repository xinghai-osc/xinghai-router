package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

type channel struct {
	id                                     int64
	baseURL, apiKey, keyID, upstreamModel  string
	provider, upstreamPath, upstreamFormat string
	priority, weight                       int
	inKeyGroup                             bool
	overrides                              channelRequestOverrides
	uaPool                                 []string
	keys                                   []channelKeyCredential
	keyIndex                               int
}

// channelKeyCredential is one enabled API key of a channel, in priority order.
type channelKeyCredential struct {
	id       string
	key      string
	priority int
}

// channelRequestOverrides configures per-channel edits applied to the upstream
// request body after every built-in rewrite: Delete removes top-level fields,
// Set overwrites top-level fields with the configured value.
type channelRequestOverrides struct {
	Delete []string       `json:"delete"`
	Set    map[string]any `json:"set"`
}

const (
	maxRequestOverrideFields = 50
	maxRequestOverrideKeyLen = 100
	maxUAPoolEntries         = 200
	maxUALength              = 512
)

// validRequestOverrides checks field names and sizes so a misconfigured channel
// cannot wedge the gateway hot path or smuggle oversized payloads upstream.
func validRequestOverrides(ov *channelRequestOverrides) error {
	if ov == nil {
		return nil
	}
	if len(ov.Delete) > maxRequestOverrideFields || len(ov.Set) > maxRequestOverrideFields {
		return fmt.Errorf("at most %d delete and %d set fields are allowed", maxRequestOverrideFields, maxRequestOverrideFields)
	}
	seen := map[string]bool{}
	for _, field := range ov.Delete {
		field = strings.TrimSpace(field)
		if field == "" || len(field) > maxRequestOverrideKeyLen || seen[field] {
			return fmt.Errorf("delete fields must be unique, non-empty, and at most %d characters", maxRequestOverrideKeyLen)
		}
		seen[field] = true
	}
	for key := range ov.Set {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > maxRequestOverrideKeyLen {
			return fmt.Errorf("set field names must be non-empty and at most %d characters", maxRequestOverrideKeyLen)
		}
	}
	return nil
}

// normalizedOverrides trims whitespace from delete fields and set keys so the
// stored configuration matches exactly what the gateway applies.
func normalizedOverrides(ov *channelRequestOverrides) channelRequestOverrides {
	if ov == nil {
		return channelRequestOverrides{Set: map[string]any{}}
	}
	out := channelRequestOverrides{Set: map[string]any{}}
	for _, field := range ov.Delete {
		if field = strings.TrimSpace(field); field != "" {
			out.Delete = append(out.Delete, field)
		}
	}
	for key, value := range ov.Set {
		if key = strings.TrimSpace(key); key != "" {
			out.Set[key] = value
		}
	}
	return out
}

// validUAPool checks a channel's User-Agent pool so a misconfigured channel
// cannot wedge the gateway hot path or smuggle oversized headers upstream.
func validUAPool(pool []string) error {
	if len(pool) > maxUAPoolEntries {
		return fmt.Errorf("at most %d user agents are allowed", maxUAPoolEntries)
	}
	for _, ua := range pool {
		if ua = strings.TrimSpace(ua); ua == "" || len(ua) > maxUALength {
			return fmt.Errorf("each user agent must be 1-%d characters", maxUALength)
		}
	}
	return nil
}

// normalizedUAPool trims whitespace and drops empty or duplicate entries so the
// stored configuration matches exactly what the gateway applies.
func normalizedUAPool(pool []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(pool))
	for _, ua := range pool {
		if ua = strings.TrimSpace(ua); ua == "" || seen[ua] {
			continue
		}
		seen[ua] = true
		out = append(out, ua)
	}
	return out
}

// pickUA returns one entry of the channel's UA pool for a request, chosen
// deterministically from the request seed so retries of the same request keep a
// single UA while distinct requests spread across the pool. An empty pool
// yields an empty string and leaves the default client User-Agent untouched.
func (ch *channel) pickUA(seed []byte) string {
	if len(ch.uaPool) == 0 {
		return ""
	}
	return ch.uaPool[int(seed[0])%len(ch.uaPool)]
}

// uaSeed derives the User-Agent pick seed for a channel from the request ID so
// every attempt of one request uses the same UA while different requests pick
// different entries of the pool.
func uaSeed(ctx context.Context, channelID int64) []byte {
	seed := sha256.Sum256([]byte(requestID(ctx) + "|ua|" + strconv.FormatInt(channelID, 10)))
	return seed[:]
}

// randomUA returns a uniformly chosen entry of a UA pool, or "" when empty.
// Used by out-of-request probes (health checks and channel tests) that have no
// request ID to seed a deterministic pick.
func randomUA(pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}

// parsedUAPool decodes a channel's stored ua_pool JSON into a string slice.
func parsedUAPool(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var pool []string
	if json.Unmarshal(raw, &pool) != nil {
		return nil
	}
	return pool
}

// applyRequestOverrides edits a request body according to the channel's
// overrides: configured fields are deleted, then configured values are set.
// Non-JSON bodies and unknown keys pass through untouched.
func applyRequestOverrides(body []byte, ov channelRequestOverrides) []byte {
	if len(ov.Delete) == 0 && len(ov.Set) == 0 {
		return body
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	changed := false
	for _, key := range ov.Delete {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	for key, value := range ov.Set {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		payload[key] = value
		changed = true
	}
	if !changed {
		return body
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

type reservation struct{ amount float64 }

func validGatewayMaxTokens(maxTokens int) bool {
	return maxTokens > 0 && maxTokens <= maxGatewayMaxTokens
}

// stripGatewayExtensions removes router-reserved extension fields from a
// request body before it is forwarded to an upstream. Clients may add these
// fields for gateway-local behavior (e.g. prompt caching), but strict
// OpenAI-compatible upstreams reject unknown fields with a 400.
func stripGatewayExtensions(body []byte) []byte {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	changed := false
	for _, key := range []string{"promptCacheKey"} {
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	if !changed {
		return body
	}
	stripped, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return stripped
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
	// Only models served by a channel the caller may use are listed: the key's
	// bound group when set, otherwise the groups the user belongs to, plus any
	// channel restricted to the caller alone. Public and ungrouped channels are
	// not exposed to callers.
	rows, err := s.db.Query(r.Context(), `select model from (
		select jsonb_array_elements_text(c.models) as model from channels c where c.enabled and (
			($2<>'' and (exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid) or c.user_id=$1))
			or ($2='' and (exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1) or c.user_id=$1))
		)
		union
		select m.public_model as model from model_routes m join channels c on c.id=m.channel_id where m.enabled and not m.hidden and c.enabled and (
			($2<>'' and (exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($2,'')::uuid) or c.user_id=$1))
			or ($2='' and (exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$1) or c.user_id=$1))
		)
	) available order by model`, key.userID, key.groupID)
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
	started := time.Now()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logReject(r.Context(), "", 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "could not read request")
		return
	}
	var request struct {
		Model     string `json:"model"`
		Stream    bool   `json:"stream"`
		MaxTokens int    `json:"max_tokens"`
	}
	if json.Unmarshal(body, &request) != nil {
		s.logReject(r.Context(), "", 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "model is required")
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if !validModelName(request.Model) {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "model must be 1-200 characters")
		return
	}
	if request.MaxTokens > maxGatewayMaxTokens {
		s.logReject(r.Context(), request.Model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "max_tokens must be at most 200000")
		return
	}
	s.proxyChatCompletions(w, r, body, request.Model, request.Stream, request.MaxTokens, nil, nil, nil, nil, nil)
}

type responseTransform func([]byte) ([]byte, error)

type requestTransform func([]byte, string) []byte

type providerResponseTransform func([]byte, string) ([]byte, error)

type providerStreamTransform func(http.ResponseWriter, *http.Response, string) (streamStats, error)

// reasoningProvider reports whether a channel routes DeepSeek-style
// reasoning_content verbatim. Only the DeepSeek thinking mode requires the
// reasoning text of a prior assistant turn to be passed back unchanged; every
// other provider (including OpenCode Go) follows OpenAI behavior and drops it.
func reasoningProvider(provider string) bool {
	return provider == "deepseek"
}

// streamStats carries the token counts extracted from an SSE stream's usage
// events. These are used to bill streaming requests after the stream closes.
type streamStats struct {
	prompt, cached, completion int
	usageReported              bool
	usageComplete              bool
	promptReported             bool
	completionReported         bool
}

type streamTransform func(http.ResponseWriter, *http.Response) (streamStats, error)

// logReject records an authenticated gateway request that was rejected before any
// upstream attempt, so the request log covers every call, not just ones that
// reached a channel. Requests without key context (unauthenticated) are skipped.
func (s *Service) logReject(ctx context.Context, model string, status int, code string, started time.Time) {
	key, ok := ctx.Value(contextKey{}).(keyContext)
	if !ok {
		return
	}
	s.logRequest(ctx, key, 0, "", model, status, 0, 0, 0, time.Since(started), code, "")
}

func (s *Service) proxyChatCompletions(w http.ResponseWriter, r *http.Request, body []byte, model string, stream bool, maxTokens int, transform responseTransform, streamFn streamTransform, requestFn requestTransform, providerTransform providerResponseTransform, providerStreamFn providerStreamTransform) {
	started := time.Now()
	ctx := r.Context()
	key := ctx.Value(contextKey{}).(keyContext)
	if maxTokens > maxGatewayMaxTokens {
		s.logReject(ctx, model, 400, "invalid_request", started)
		writeError(w, 400, "invalid_request", "max_tokens must be at most 200000")
		return
	}
	if err := s.checkQuota(ctx, key, model); err != nil {
		s.logReject(ctx, model, 429, "quota_exceeded", started)
		writeError(w, 429, "quota_exceeded", "request quota exceeded")
		return
	}
	// Pricing and the group multiplier are read once and reused by reservation and
	// settlement, which previously repeated the same two lookups.
	pricing := s.pricingFor(ctx, model)
	groupMultiplier := s.groupMultiplierFor(ctx, key.groupID)
	subscriptionAccess := s.subscriptionCoversModel(ctx, key.userID, model)
	ctx = context.WithValue(ctx, subscriptionCoveredKey{}, subscriptionAccess)
	var reserved reservation
	if subscriptionAccess.Covered {
		// Subscription-covered requests are never billed.
	} else if subscriptionAccess.OveragePolicy == "block" {
		s.logReject(ctx, model, 402, "subscription_quota_exceeded", started)
		writeError(w, 402, "subscription_quota_exceeded", "subscription period quota exhausted")
		return
	} else {
		var err error
		reserved, err = s.reserveUsage(ctx, key, model, len(body), maxTokens, pricing, groupMultiplier)
		if err != nil {
			if errors.Is(err, errPricingUnavailable) {
				s.logReject(ctx, model, 402, "pricing_unavailable", started)
				writeError(w, 402, "pricing_unavailable", "no enabled pricing rule for this model")
				return
			}
			s.logReject(ctx, model, 402, "insufficient_quota", started)
			writeError(w, 402, "insufficient_quota", "insufficient balance for this request")
			return
		}
	}
	defer func() { s.releaseReservation(ctx, key, reserved) }()
	maxUserConcurrency := s.userConcurrencyLimitFor(ctx, key.userID)
	if maxUserConcurrency > 0 && !s.userLimiter.acquire(key.userID, maxUserConcurrency) {
		s.releaseReservation(ctx, key, reserved)
		reserved = reservation{}
		writeError(w, 429, "user_concurrency_exceeded", "user concurrency limit exceeded")
		return
	}
	defer func() {
		if maxUserConcurrency > 0 {
			s.userLimiter.release(key.userID)
		}
	}()
	// Group concurrency limit: reject when the group's limit is reached,
	// preventing a single group from saturating all its channels.
	maxConcurrency := 0
	if key.groupID != "" {
		maxConcurrency = s.groupConcurrencyLimitFor(ctx, key.groupID)
	}
	if maxConcurrency > 0 && !s.groupLimiter.acquire(key.groupID, maxConcurrency) {
		s.releaseReservation(ctx, key, reserved)
		reserved = reservation{}
		writeError(w, 429, "group_concurrency_exceeded", "group concurrency limit exceeded")
		return
	}
	defer func() {
		if key.groupID != "" && maxConcurrency > 0 {
			s.groupLimiter.release(key.groupID)
		}
	}()
	channels, err := s.channelsForModel(ctx, key, model)
	if err != nil {
		if errors.Is(err, errChannelCredentials) {
			s.logRequest(ctx, key, 0, "", model, 503, 0, 0, 0, time.Since(started), "channel_credentials", "channel credentials unavailable")
			log.Printf("gateway: model %q has enabled channels but no usable credentials (ENCRYPTION_KEY mismatch or missing keys)", model)
			writeError(w, 503, "credential_unavailable", "enabled channels for this model have no usable credentials")
			return
		}
		if !errors.Is(err, errInvalid) {
			log.Printf("gateway: channel lookup for model %q failed: %v", model, err)
		}
		s.logRequest(ctx, key, 0, "", model, 503, 0, 0, 0, time.Since(started), "no_channel", "no usable channel supports this model")
		writeError(w, 503, "model_unavailable", "no usable channel supports this model")
		return
	}
	reliability := s.reliabilitySettings(ctx)
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
	prefill := ""
	failCode := "upstream_unreachable"
	failDetail := failCode
	// channelsForModel already returns only channels of the key's own group
	// context, so every candidate is in-group and retries can never spill onto
	// channels belonging to another group.
	retryChannels := channels
	if key.groupID != "" {
		var inGroup []channel
		for _, c := range channels {
			if c.inKeyGroup {
				inGroup = append(inGroup, c)
			}
		}
		if len(inGroup) > 0 {
			retryChannels = inGroup
		}
	}
tryChannels:
	for pass := 0; pass <= reliability.RetryCount; pass++ {
		for i := range retryChannels {
			ch = retryChannels[i]
			if s.checkChannelQuota(ctx, ch.id, model) != nil {
				continue
			}
			upstreamFormat := ch.upstreamFormat
			if upstreamFormat == "" {
				if ch.provider == "anthropic" {
					upstreamFormat = "anthropic"
				} else {
					upstreamFormat = "openai"
				}
			}
			upstreamPath := ch.upstreamPath
			if upstreamPath == "" {
				if upstreamFormat == "anthropic" {
					upstreamPath = "/v1/messages"
				} else {
					upstreamPath = "/v1/chat/completions"
				}
			}
			upstreamURL := ch.baseURL + upstreamPath
			upstreamBody := stripGatewayExtensions(body)
			if stream && upstreamFormat != "anthropic" {
				var payload map[string]any
				if json.Unmarshal(upstreamBody, &payload) == nil {
					so, _ := payload["stream_options"].(map[string]any)
					if so == nil {
						so = map[string]any{}
					}
					so["include_usage"] = true
					payload["stream_options"] = so
					if upstreamBody, err = json.Marshal(payload); err != nil {
						continue
					}
				}
			}
			if ch.upstreamModel != "" && ch.upstreamModel != model {
				var payload map[string]any
				if json.Unmarshal(upstreamBody, &payload) == nil {
					payload["model"] = ch.upstreamModel
					upstreamBody, _ = json.Marshal(payload)
				}
			}
			if upstreamFormat == "anthropic" {
				var prefillText string
				upstreamBody, prefillText, err = openAIRequestToAnthropic(upstreamBody)
				if err != nil {
					continue
				}
				prefill = prefillText
			}
			if requestFn != nil {
				upstreamBody = requestFn(upstreamBody, ch.provider)
			}
			// Channel-configured deletes/overrides apply last, after every built-in
			// rewrite, so the admin's configuration is authoritative.
			upstreamBody = applyRequestOverrides(upstreamBody, ch.overrides)
			upstreamReq, requestErr := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
			if requestErr != nil {
				continue
			}
			if upstreamFormat == "anthropic" {
				upstreamReq.Header.Set("X-API-Key", ch.apiKey)
				upstreamReq.Header.Set("Anthropic-Version", "2023-06-01")
			} else {
				upstreamReq.Header.Set("Authorization", "Bearer "+ch.apiKey)
			}
			upstreamReq.Header.Set("Content-Type", "application/json")
			upstreamReq.Header.Set("Accept", accept)
			if ua := ch.pickUA(uaSeed(r.Context(), ch.id)); ua != "" {
				upstreamReq.Header.Set("User-Agent", ua)
			}
			resp, err = client.Do(upstreamReq)
			if err != nil {
				if code, detail, ok := classifyContextError(err); ok {
					// The client hung up or the request timeout elapsed: retrying
					// other channels cannot help, and a healthy channel must not be
					// counted as failed for a problem that is not its fault.
					failCode, failDetail = code, detail
					break tryChannels
				}
			}
			if err == nil && !reliability.retryable(resp.StatusCode) {
				break tryChannels
			}
			failureReason := "upstream_unreachable"
			failDetail = failureReason
			if err != nil {
				failDetail = err.Error()
			}
			if err == nil {
				failureReason = "upstream_status_" + strconv.Itoa(resp.StatusCode)
				// Apply auto-disable rules to the upstream error body before retrying.
				bodyPeek, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
				resp.Body.Close()
				if readErr == nil {
					failDetail = failureReason + ": " + string(bodyPeek)
					if reliability.autoDisableStatus(resp.StatusCode) || reliability.autoDisableKeyword(string(bodyPeek)) {
						s.autoDisableChannel(ctx, ch.id, ch.keyID, failureReason)
					}
				}
				resp = nil
			}
			s.channelFailed(ctx, ch.id, ch.keyID, failureReason)
			// Retries rotate to the next credential in priority order so a
			// multi-key channel is not replayed with the same key.
			retryChannels[i].rotateKey()
		}
	}
	if resp == nil {
		prompt := len(body) / 3
		s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, prompt, 0, prompt, time.Since(started), failCode, failDetail)
		clientDetail := "all upstream channels failed"
		if failCode == "request_timeout" || failCode == "client_canceled" {
			clientDetail = failDetail
		}
		writeError(w, 502, "upstream_error", clientDetail)
		return
	}
	defer resp.Body.Close()
	selectedFormat := ch.upstreamFormat
	if selectedFormat == "" {
		if ch.provider == "anthropic" {
			selectedFormat = "anthropic"
		} else {
			selectedFormat = "openai"
		}
	}
	if stream && resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		var st streamStats
		cacheEnabled := s.conversationCacheSettings(ctx).Enabled
		capture := newStreamCaptureWriter(w, cacheEnabled)
		w = capture
		var streamErr error
		if providerStreamFn != nil {
			st, streamErr = providerStreamFn(w, resp, ch.provider)
		} else if streamFn != nil {
			// streamFn is format-aware: it converts whatever the upstream actually
			// speaks (chat-completions chunks or Anthropic events) into the client
			// format. For Anthropic-to-Anthropic streams it falls back to relaying
			// the events verbatim.
			st, streamErr = streamFn(w, resp)
		} else if selectedFormat == "anthropic" {
			st, streamErr = streamAnthropicToOpenAI(w, resp, prefill)
		} else {
			st, streamErr = s.streamResponse(w, resp)
		}
		status := resp.StatusCode
		if !st.usageComplete && streamErr == nil {
			// Do not let providers that omit streaming usage turn a successful request
			// into a free request. Fall back to the same conservative token estimate
			// used by the reservation path and charge that hold.
			st.prompt, st.completion = estimatedStreamUsage(body, maxTokens)
			st.cached = 0
		} else if st.cached == 0 {
			// The upstream did not serve this prompt from its cache. Fall back to the
			// local prefix cache so overlapping prompts are billed at the cached rate
			// instead of paying full input price every time.
			st.cached = int(s.promptCache.cached(model, normalizedPrompt(body), int64(st.prompt)))
		}
		total := st.prompt + st.completion
		code, detail := "", ""
		contextError := false
		if code, detail, contextError = classifyContextError(streamErr); !contextError {
			switch {
			case streamErr != nil && !capture.wrote && !capture.headerSent:
				// The stream failed before emitting anything: hand the client a clean error.
				code, detail = "upstream_stream_error", streamErr.Error()
			case !capture.wrote && !capture.headerSent:
				// The upstream accepted a 2xx streaming request but never sent a single byte.
				// Relaying that as an empty 200 confuses clients, so convert it into a 502.
				code, detail = "empty_upstream_response", "upstream returned an empty streaming response"
			case streamErr != nil:
				// Headers were already sent; the client saw a partial stream. Record the
				// failure but keep the original status because it is already dispatching.
				code, detail = "upstream_stream_error", streamErr.Error()
			}
		}
		if code != "" {
			if !capture.wrote && !capture.headerSent {
				status = 502
			}
			if st.prompt == 0 {
				st.prompt = len(body) / 3
				total = st.prompt + st.completion
			}
			s.logRequest(ctx, key, ch.id, ch.keyID, model, status, st.prompt, st.completion, total, time.Since(started), code, detail)
			if !capture.wrote && !capture.headerSent {
				writeError(w, status, "upstream_error", s.clientUpstreamError(ctx, detail, reliability))
			}
			if !contextError {
				s.channelFailed(ctx, ch.id, ch.keyID, code)
			}
		} else {
			s.logRequest(ctx, key, ch.id, ch.keyID, model, status, st.prompt, st.completion, total, time.Since(started), "", "")
			if streamErr == nil {
				s.promptCache.store(model, normalizedPrompt(body), int64(st.prompt))
			}
			if st.prompt > 0 || st.completion > 0 {
				if subscriptionAccess.Covered {
					s.settleSubscriptionUsage(ctx, key, model, st.prompt, st.cached, st.completion, pricing, groupMultiplier)
				} else {
					reserved = s.settleUsage(ctx, key, reserved, model, st.prompt, st.cached, st.completion, pricing, groupMultiplier)
				}
			}
			s.channelSucceeded(ctx, ch.id, ch.keyID)
		}
		// Only cache a stream that ran to completion. A stream interrupted by a
		// client disconnect or an upstream error is partial output and useless for
		// offline analysis, plus dropping it avoids recording half-sent turns.
		if cacheEnabled && streamErr == nil {
			s.storeConversationCache(ctx, key, model, true, body, capture.bytes(), status, time.Since(started).Milliseconds())
		}
		return
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBody))
	if err != nil {
		prompt := len(body) / 3
		code, detail, contextError := classifyContextError(err)
		if !contextError {
			code, detail = "upstream_read_error", err.Error()
		}
		s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, prompt, 0, prompt, time.Since(started), code, detail)
		writeError(w, 502, "upstream_error", "could not read upstream response")
		if !contextError {
			s.channelFailed(ctx, ch.id, ch.keyID, "upstream_read_error")
		}
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified && len(responseBody) == 0 {
		prompt := len(body) / 3
		s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, prompt, 0, prompt, time.Since(started), "empty_upstream_response", "upstream returned an empty response")
		writeError(w, 502, "upstream_error", "upstream returned an empty response")
		s.channelFailed(ctx, ch.id, ch.keyID, "empty_upstream_response")
		return
	}
	if selectedFormat == "anthropic" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		responseBody, err = anthropicResponseToOpenAI(responseBody, prefill)
		if err != nil {
			s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, 0, 0, 0, time.Since(started), "upstream_convert_error", err.Error())
			writeError(w, 502, "upstream_error", "could not convert upstream response")
			s.channelFailed(ctx, ch.id, ch.keyID, "upstream_convert_error")
			return
		}
	}
	prompt, completion, total, cached := usage(responseBody)
	if cached == 0 {
		// Same accounting assist as the streaming path: an upstream cache miss does
		// not mean the prompt prefixes were never seen here before.
		cached = int(s.promptCache.cached(model, normalizedPrompt(body), int64(prompt)))
	}
	detail := ""
	if resp.StatusCode >= 400 {
		detail = string(responseBody)
		if prompt == 0 && completion == 0 {
			prompt = len(body) / 3
			total = prompt
		}
	}
	s.logRequest(ctx, key, ch.id, ch.keyID, model, resp.StatusCode, prompt, completion, total, time.Since(started), errorCode(resp.StatusCode), detail)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.promptCache.store(model, normalizedPrompt(body), int64(prompt))
		if subscriptionAccess.Covered {
			s.settleSubscriptionUsage(ctx, key, model, prompt, cached, completion, pricing, groupMultiplier)
		} else {
			reserved = s.settleUsage(ctx, key, reserved, model, prompt, cached, completion, pricing, groupMultiplier)
		}
		s.channelSucceeded(ctx, ch.id, ch.keyID)
		if providerTransform != nil {
			responseBody, err = providerTransform(responseBody, ch.provider)
			if err != nil {
				s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, 0, 0, 0, time.Since(started), "upstream_convert_error", err.Error())
				writeError(w, 502, "upstream_error", "could not convert upstream response")
				return
			}
		} else if transform != nil {
			responseBody, err = transform(responseBody)
			if err != nil {
				s.logRequest(ctx, key, ch.id, ch.keyID, model, 502, 0, 0, 0, time.Since(started), "upstream_convert_error", err.Error())
				writeError(w, 502, "upstream_error", "could not convert upstream response")
				return
			}
		}
	} else if resp.StatusCode >= 400 {
		// Mirror the streaming path: real-request failures are counted and can
		// auto-disable the channel, so a persistently broken upstream is retired.
		// The body the client receives runs through the keyword/URL rewrites;
		// the request log keeps the original upstream text.
		failureReason := "upstream_status_" + strconv.Itoa(resp.StatusCode)
		if reliability.autoDisableStatus(resp.StatusCode) || reliability.autoDisableKeyword(detail) {
			s.autoDisableChannel(ctx, ch.id, ch.keyID, failureReason)
		}
		s.channelFailed(ctx, ch.id, ch.keyID, failureReason)
		responseBody = []byte(s.clientUpstreamError(ctx, detail, reliability))
	}
	w.Header().Set("Content-Type", contentType(resp.Header.Get("Content-Type")))
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)
	s.storeConversationCache(ctx, key, model, false, body, responseBody, resp.StatusCode, time.Since(started).Milliseconds())
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
	// Apply time-based pricing override for the current moment.
	input, cachedInput, output := pricing.resolvePricing(time.Now())
	// Reserve the configured maximum output plus a conservative request-body estimate.
	amount := (float64(bodyLen/3)*input + float64(resolved)*output) / 1000000 * pricing.multiplier * groupMultiplier
	_ = cachedInput // not used in reservation estimate
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

func usageCost(prompt, cached, completion int, input, cachedInput, output, multiplier, groupMultiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	if groupMultiplier <= 0 {
		groupMultiplier = 1
	}
	nonCached := prompt - cached
	if nonCached < 0 {
		nonCached = 0
	}
	return (float64(nonCached)*input + float64(cached)*cachedInput + float64(completion)*output) / 1000000 * multiplier * groupMultiplier
}

// tieredUsageCost computes cost with tiered pricing: the total token count (prompt +
// completion) selects the active price band, then the standard formula applies.
func tieredUsageCost(prompt, cached, completion int, pricing pricingRule, input, cachedInput, output float64) float64 {
	multiplier := pricing.multiplier
	if multiplier <= 0 {
		multiplier = 1
	}
	totalTokens := int64(prompt) + int64(completion)
	ti, tc, to := pricing.resolveTier(totalTokens, input, cachedInput, output)
	nonCached := prompt - cached
	if nonCached < 0 {
		nonCached = 0
	}
	return (float64(nonCached)*ti + float64(cached)*tc + float64(completion)*to) / 1000000 * multiplier
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
func (s *Service) settleUsage(ctx context.Context, key keyContext, held reservation, model string, prompt, cached, completion int, pricing pricingRule, groupMultiplier float64) reservation {
	if held.amount == 0 && prompt == 0 && completion == 0 {
		return held
	}
	cost := computeUsageCost(prompt, cached, completion, pricing, groupMultiplier)
	cost = clampCostToHold(cost, held.amount)
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
	insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost)
	select $7::uuid,$5::text,$3,$8::uuid,$6::text,$9::int,$10::int,$11::int,$1 from settled
	on conflict(request_id) do update set prompt_tokens=excluded.prompt_tokens,cached_prompt_tokens=excluded.cached_prompt_tokens,completion_tokens=excluded.completion_tokens,cost=excluded.cost`,
		cost, held.amount, key.userID, ledgerID, requestID(ctx), model, usageID, key.keyID, prompt, cached, completion)
	if err != nil || tag.RowsAffected() == 0 {
		return held
	}
	return reservation{}
}

// computeUsageCost returns the cost of a request under the active pricing rule,
// shared by the wallet settlement and the subscription-covered accounting path.
func computeUsageCost(prompt, cached, completion int, pricing pricingRule, groupMultiplier float64) float64 {
	// Apply time-based pricing, then tiered pricing for the actual token count.
	input, cachedInput, output := pricing.resolvePricing(time.Now())
	var cost float64
	if len(pricing.tiers) > 0 {
		cost = tieredUsageCost(prompt, cached, completion, pricing, input, cachedInput, output)
		if groupMultiplier > 0 {
			cost *= groupMultiplier
		}
	} else {
		cost = usageCost(prompt, cached, completion, input, cachedInput, output, pricing.multiplier, groupMultiplier)
	}
	return cost
}

// settleSubscriptionUsage records the cost of a successful subscription-covered request
// without touching the wallet. The usage_records row feeds the per-period credit quota in
// subscriptionCoversModel, so a subscription's monthly credit cap counts what its requests
// would have cost under normal pricing.
func (s *Service) settleSubscriptionUsage(ctx context.Context, key keyContext, model string, prompt, cached, completion int, pricing pricingRule, groupMultiplier float64) {
	if prompt == 0 && completion == 0 {
		return
	}
	cost := computeUsageCost(prompt, cached, completion, pricing, groupMultiplier)
	// The covering subscription is resolved during the coverage check; its
	// per-period counters must be decremented once per settled request.
	access, _ := ctx.Value(subscriptionCoveredKey{}).(subscriptionAccess)
	usageID, _ := randomID()
	settleCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, err := s.db.Exec(settleCtx, `with inserted as (
		insert into usage_records(id,request_id,user_id,api_key_id,model,prompt_tokens,cached_prompt_tokens,completion_tokens,cost)
		values($1::uuid,$2::text,$3,$4::uuid,$5::text,$6::int,$7::int,$8::int,$9)
		on conflict(request_id) do nothing
		returning 1
	), subscription as (
		update user_subscriptions us set
			remaining_requests = case when us.remaining_requests is null then null
				when exists (select 1 from subscription_plan_model_quotas q where q.plan_id=us.plan_id and q.model=$5 and q.max_requests_per_period is not null)
					then us.remaining_requests
				else greatest(0, us.remaining_requests-1) end,
			remaining_credit = case when us.remaining_credit is null then null
				when exists (select 1 from subscription_plan_model_quotas q where q.plan_id=us.plan_id and q.model=$5 and q.max_credit_per_period is not null)
					then us.remaining_credit
				else greatest(0, us.remaining_credit-$9) end,
			updated_at=now()
		where us.id=$10 and exists (select 1 from inserted)
		returning 1
	)
	update user_subscription_model_usage set
		remaining_requests = case when remaining_requests is null then null else greatest(0, remaining_requests-1) end,
		remaining_credit = case when remaining_credit is null then null else greatest(0, remaining_credit-$9) end
	where subscription_id=$10 and model=$5 and exists (select 1 from inserted)`,
		usageID, requestID(ctx), key.userID, key.keyID, model, prompt, cached, completion, cost, access.SubscriptionID)
	if err != nil {
		log.Printf("settleSubscriptionUsage failed: %v", err)
	}
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

// checkQuota evaluates every matching quota row in one query. Each row's usage
// window is aggregated by a lateral join instead of a follow-up query per row.
// "total" rows aggregate lifetime usage (no created_at cutoff); day/month/minute
// rows use a rolling window. Cost is summed from usage_records via the shared
// request_id so a key can also be capped on spend.
func (s *Service) checkQuota(ctx context.Context, key keyContext, model string) error {
	rows, err := s.db.Query(ctx, `select q.max_requests,q.max_tokens,q.max_cost,agg.requests,agg.tokens,agg.cost
	from quota_limits q
	cross join lateral (
		select count(rl.*) as requests, coalesce(sum(rl.total_tokens),0) as tokens, coalesce(sum(ur.cost),0) as cost
		from request_logs rl
		left join usage_records ur on ur.request_id=rl.request_id
		where rl.api_key_id=$2 and (q."window"='total' or rl.created_at >= now() - ('1 '||q."window")::interval)
	) agg
	where (q.user_id=$1 or q.user_id is null) and (q.api_key_id=$2 or q.api_key_id is null) and (q.model=$3 or q.model is null) and (q.max_requests is not null or q.max_tokens is not null or q.max_cost is not null)`, key.userID, key.keyID, model)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var maxRequests, maxTokens *int64
		var maxCost *float64
		var count, tokens int64
		var cost float64
		if rows.Scan(&maxRequests, &maxTokens, &maxCost, &count, &tokens, &cost) != nil {
			return errInvalid
		}
		if (maxRequests != nil && count >= *maxRequests) || (maxTokens != nil && tokens >= *maxTokens) || (maxCost != nil && cost >= *maxCost) {
			return errInvalid
		}
	}
	return rows.Err()
}

// channelsForModel returns the candidate channels for a model, memoised for a
// few seconds per (user,group,model). Channel configuration changes rarely; the
// cache removes the 1+N channel and key queries from every proxied request. A
// fresh copy of the slice is returned so the retry loop's credential rotation
// never mutates the shared cache entry.
func (s *Service) channelsForModel(ctx context.Context, key keyContext, model string) ([]channel, error) {
	ck := channelRouteKey{userID: key.userID, groupID: key.groupID, model: model}
	channels, err := s.channelCache.get(ctx, ck, func(ctx context.Context) ([]channel, error) {
		return s.loadChannelsForModel(ctx, key, model)
	})
	if err != nil {
		return nil, err
	}
	return cloneChannels(channels), nil
}

func (s *Service) loadChannelsForModel(ctx context.Context, key keyContext, model string) ([]channel, error) {
	rows, err := s.db.Query(ctx, `select c.id,c.base_url,c.api_key,coalesce(m.priority,c.priority),coalesce(m.weight,c.weight),coalesce(m.upstream_model,''),c.provider,c.upstream_path,c.upstream_format,c.request_overrides,c.ua_pool,case when $3='' then exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$2) or c.user_id=$2 else exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($3,'')::uuid) or c.user_id=$2 end as in_key_group from channels c left join model_routes m on m.channel_id=c.id and m.public_model=$1 and m.enabled where (c.enabled or c.auto_disabled) and (c.models ? $1 or m.public_model is not null) and (($3<>'' and (exists(select 1 from channel_groups cg where cg.channel_id=c.id and cg.group_id=nullif($3,'')::uuid) or c.user_id=$2)) or ($3='' and (exists(select 1 from channel_groups cg join user_groups ug on ug.group_id=cg.group_id where cg.channel_id=c.id and ug.user_id=$2) or c.user_id=$2))) order by (c.enabled and not c.auto_disabled) desc, coalesce(m.priority,c.priority) desc, c.priority desc, c.id`, model, key.userID, key.groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []channel
	skipped := 0
	seed := sha256.Sum256([]byte(requestID(ctx)))
	for rows.Next() {
		var ch channel
		var encrypted string
		var overrides, uaPool []byte
		if err := rows.Scan(&ch.id, &ch.baseURL, &encrypted, &ch.priority, &ch.weight, &ch.upstreamModel, &ch.provider, &ch.upstreamPath, &ch.upstreamFormat, &overrides, &uaPool, &ch.inKeyGroup); err != nil {
			return nil, err
		}
		if len(overrides) > 0 {
			_ = json.Unmarshal(overrides, &ch.overrides)
		}
		if len(uaPool) > 0 {
			_ = json.Unmarshal(uaPool, &ch.uaPool)
		}
		keys, err := s.channelKeys(ctx, ch.id)
		if err != nil {
			return nil, err
		}
		if len(keys) > 0 {
			ch.keys = keys
			ch.keyIndex = initialKeyIndex(keys, seed[:])
			ch.apiKey = keys[ch.keyIndex].key
			ch.keyID = keys[ch.keyIndex].id
		} else if encrypted != "" {
			ch.apiKey, err = channelKeyValue(s.cfg.EncryptionKey, encrypted)
			if err != nil {
				skipped++
				continue
			}
		} else {
			skipped++
			continue
		}
		result = append(result, ch)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if len(result) == 0 {
		if skipped > 0 {
			return nil, errChannelCredentials
		}
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

// channelKeys returns every enabled key of a channel in priority order
// (descending priority, then creation time), memoised per channel for the same
// window as the channel lists. Callers receive a fresh copy of the slice.
func (s *Service) channelKeys(ctx context.Context, channelID int64) ([]channelKeyCredential, error) {
	keys, err := s.channelKeyCache.get(ctx, channelID, func(ctx context.Context) ([]channelKeyCredential, error) {
		return s.loadChannelKeys(ctx, channelID)
	})
	if err != nil {
		return nil, err
	}
	return cloneChannelKeys(keys), nil
}

func (s *Service) loadChannelKeys(ctx context.Context, channelID int64) ([]channelKeyCredential, error) {
	krows, err := s.db.Query(ctx, `select id,key_encrypted,priority from channel_api_keys where channel_id=$1 and enabled order by priority desc,created_at`, channelID)
	if err != nil {
		return nil, err
	}
	defer krows.Close()
	var keys []channelKeyCredential
	for krows.Next() {
		var id, enc string
		var priority int
		if krows.Scan(&id, &enc, &priority) != nil {
			continue
		}
		key, err := channelKeyValue(s.cfg.EncryptionKey, enc)
		if err != nil {
			continue
		}
		keys = append(keys, channelKeyCredential{id, key, priority})
	}
	return keys, krows.Err()
}

// initialKeyIndex picks the starting credential for a request deterministically
// from the request seed, preferring the highest-priority group so lower-priority
// keys are only used once the primary group is exhausted (e.g. after keys are
// disabled). Retries rotate from this index through every key in order.
func initialKeyIndex(keys []channelKeyCredential, seed []byte) int {
	if len(keys) <= 1 {
		return 0
	}
	end := 1
	for end < len(keys) && keys[end].priority == keys[0].priority {
		end++
	}
	return int(seed[0]) % end
}

// rotateKey advances a multi-key channel to the next credential in priority
// order for a retry attempt; single-key and legacy channels are unchanged.
func (ch *channel) rotateKey() {
	if len(ch.keys) > 1 {
		ch.keyIndex = (ch.keyIndex + 1) % len(ch.keys)
		ch.apiKey = ch.keys[ch.keyIndex].key
		ch.keyID = ch.keys[ch.keyIndex].id
	}
}

func (s *Service) selectChannelKey(ctx context.Context, channelID int64, fallbackEncrypted string, seed []byte) (string, string, error) {
	keys, err := s.channelKeys(ctx, channelID)
	if err != nil {
		return "", "", err
	}
	if len(keys) > 0 {
		picked := keys[initialKeyIndex(keys, seed)]
		return picked.key, picked.id, nil
	}
	if fallbackEncrypted != "" {
		k, err := channelKeyValue(s.cfg.EncryptionKey, fallbackEncrypted)
		return k, "", err
	}
	return "", "", errInvalid
}

// channelSucceeded clears failure bookkeeping in the background. The WHERE clause makes
// the common case (an already-healthy channel checked recently) touch no rows, which
// keeps a shared channel row from becoming a write hotspot under concurrent traffic.
func (s *Service) channelSucceeded(ctx context.Context, id int64, keyID string) {
	s.background.submit(func(ctx context.Context) {
		changed := false
		if keyID != "" {
			tag, err := s.db.Exec(ctx, `update channel_api_keys set failure_count=0,last_error=null,last_checked_at=now() where id=$1 and channel_id=$2 and (failure_count<>0 or last_error is not null or last_checked_at is null or last_checked_at < now()-interval '30 seconds')`, keyID, id)
			if err == nil && tag.RowsAffected() > 0 {
				changed = true
			}
		}
		tag, err := s.db.Exec(ctx, `update channels set failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now(),enabled=case when auto_disabled then true else enabled end,auto_disabled=case when auto_disabled then false else auto_disabled end,disabled_reason=case when auto_disabled then '' else disabled_reason end where id=$1 and (failure_count<>0 or cooldown_until is not null or last_error is not null or last_checked_at is null or last_checked_at < now()-interval '30 seconds')`, id)
		if err == nil && tag.RowsAffected() > 0 {
			changed = true
		}
		if changed {
			s.invalidateChannels()
		}
	})
}

// channelFailed counts a failure against the channel or, when the failing key is
// known (multi-key channels), against that key. Three failures on the same key
// trigger an out-of-request verification test of exactly that key.
func (s *Service) channelFailed(ctx context.Context, channelID int64, keyID, reason string) {
	if keyID != "" {
		var failureCount int
		err := s.db.QueryRow(ctx, `update channel_api_keys set failure_count=failure_count+1,last_error=$2,last_checked_at=now() where id=$1 and channel_id=$3 returning failure_count`, keyID, reason, channelID).Scan(&failureCount)
		if err == nil && failureCount == 3 {
			go s.testFailedChannelKey(channelID, keyID)
		}
		return
	}
	var failureCount int
	err := s.db.QueryRow(ctx, `update channels set failure_count=failure_count+1,last_error=$2,last_checked_at=now(),updated_at=now() where id=$1 returning failure_count`, channelID, reason).Scan(&failureCount)
	if err == nil && failureCount == 3 {
		go s.testFailedChannel(channelID)
	}
}

// testFailedChannel verifies a newly unhealthy channel outside the client request.
// Used for legacy channels without channel_api_keys rows; multi-key channels go
// through testFailedChannelKey so only the failing key is retired.
func (s *Service) testFailedChannel(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()
	var baseURL, encrypted, provider, upstreamFormat string
	var enabled, autoDisable bool
	var uaPool []byte
	if err := s.db.QueryRow(ctx, `select c.base_url,c.api_key,c.provider,c.upstream_format,c.enabled,c.ua_pool,ss.auto_disable_failed_channels from channels c cross join site_settings ss where c.id=$1 and ss.id=true`, id).Scan(&baseURL, &encrypted, &provider, &upstreamFormat, &enabled, &uaPool, &autoDisable); err != nil || !enabled || !autoDisable {
		return
	}
	seed := sha256.Sum256([]byte(strconv.FormatInt(id, 10) + "test"))
	apiKey, _, err := s.selectChannelKey(ctx, id, encrypted, seed[:])
	if err != nil {
		s.disableFailedChannel(ctx, id, "", "credential_decryption_failed")
		return
	}
	s.testFailedCredential(ctx, id, "", baseURL, apiKey, provider, upstreamFormat, parsedUAPool(uaPool))
}

// testFailedChannelKey verifies a channel API key that failed repeatedly, so only
// that key is auto-disabled when the upstream really rejects it.
func (s *Service) testFailedChannelKey(channelID int64, keyID string) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()
	var baseURL, encrypted, provider, upstreamFormat string
	var enabled, autoDisable bool
	var uaPool []byte
	if err := s.db.QueryRow(ctx, `select c.base_url,k.key_encrypted,c.provider,c.upstream_format,c.enabled,c.ua_pool,ss.auto_disable_failed_channels from channels c join channel_api_keys k on k.channel_id=c.id cross join site_settings ss where c.id=$1 and k.id=$2 and ss.id=true`, channelID, keyID).Scan(&baseURL, &encrypted, &provider, &upstreamFormat, &enabled, &uaPool, &autoDisable); err != nil || !enabled || !autoDisable {
		return
	}
	apiKey, err := channelKeyValue(s.cfg.EncryptionKey, encrypted)
	if err != nil {
		s.disableFailedChannel(ctx, channelID, keyID, "credential_decryption_failed")
		return
	}
	s.testFailedCredential(ctx, channelID, keyID, baseURL, apiKey, provider, upstreamFormat, parsedUAPool(uaPool))
}

// testFailedCredential probes a channel credential with GET /v1/models three
// times. Success clears the failure bookkeeping for the channel or key; three
// failed attempts auto-disable the credential.
func (s *Service) testFailedCredential(ctx context.Context, channelID int64, keyID, baseURL, apiKey, provider, upstreamFormat string, uaPool []string) {
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
		if err != nil {
			s.disableFailedChannel(ctx, channelID, keyID, "invalid_test_request")
			return
		}
		if provider == "anthropic" || (provider == "custom" && upstreamFormat == "anthropic") {
			request.Header.Set("X-API-Key", apiKey)
			request.Header.Set("Anthropic-Version", "2023-06-01")
		} else {
			request.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if ua := randomUA(uaPool); ua != "" {
			request.Header.Set("User-Agent", ua)
		}
		response, err := s.httpClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				if keyID != "" {
					_, _ = s.db.Exec(ctx, `update channel_api_keys set failure_count=0,last_error=null,last_checked_at=now() where id=$1 and channel_id=$2 and enabled`, keyID, channelID)
				} else {
					_, _ = s.db.Exec(ctx, `update channels set failure_count=0,cooldown_until=null,last_error=null,last_checked_at=now(),updated_at=now() where id=$1 and enabled`, channelID)
				}
				return
			}
		}
	}
	s.disableFailedChannel(ctx, channelID, keyID, "system_test_failed")
}

// disableFailedChannel marks a channel or, when the failing key is known, just
// that channel API key as automatically disabled after repeated failures.
func (s *Service) disableFailedChannel(ctx context.Context, channelID int64, keyID, reason string) {
	if keyID != "" {
		result, err := s.db.Exec(ctx, `update channel_api_keys set enabled=false,failure_count=0,last_error=$1,last_checked_at=now() where id=$2 and channel_id=$3 and enabled and failure_count>=3 and exists(select 1 from channels c where c.id=channel_api_keys.channel_id and c.enabled and c.auto_disable)`, reason, keyID, channelID)
		if err != nil || result.RowsAffected() != 1 {
			return
		}
		s.invalidateChannels()
		s.syncChannelKeyType(ctx, strconv.FormatInt(channelID, 10))
		details, _ := json.Marshal(map[string]string{"reason": reason})
		auditID, _ := randomID()
		_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel_key.auto_disabled','system','channel_api_key',$2,$3,'SYSTEM','/system/channel-test')`, auditID, keyID, details)
		s.disableChannelIfKeyless(ctx, channelID, reason)
		return
	}
	result, err := s.db.Exec(ctx, `update channels set enabled=false,auto_disabled=true,disabled_reason=$1,last_error=$1,last_checked_at=now(),updated_at=now() where id=$2 and enabled and auto_disable and failure_count>=3`, reason, channelID)
	if err != nil || result.RowsAffected() != 1 {
		return
	}
	s.invalidateChannels()
	details, _ := json.Marshal(map[string]string{"reason": reason})
	auditID, _ := randomID()
	_, _ = s.db.Exec(ctx, `insert into audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path) values($1,'channel.auto_disabled','system','channel',$2,$3,'SYSTEM','/system/channel-test')`, auditID, channelID, details)
}
func retryableStatus(status int) bool {
	settings := defaultReliabilitySettings()
	return settings.retryable(status)
}

type streamCaptureWriter struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	flusher    http.Flusher
	wrote      bool
	headerSent bool
}

// newStreamCaptureWriter wraps w so the gateway can tell whether an SSE stream
// actually produced any bytes. When store is true the relayed body is buffered for
// the conversation cache; otherwise no copy is kept so long-lived streams stay cheap.
func newStreamCaptureWriter(w http.ResponseWriter, store bool) *streamCaptureWriter {
	c := &streamCaptureWriter{ResponseWriter: w}
	if store {
		c.buf = &bytes.Buffer{}
	}
	if f, ok := w.(http.Flusher); ok {
		c.flusher = f
	}
	return c
}

func (c *streamCaptureWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		c.wrote = true
	}
	if c.buf != nil {
		c.buf.Write(p)
	}
	return c.ResponseWriter.Write(p)
}

func (c *streamCaptureWriter) WriteHeader(code int) {
	if !c.headerSent {
		c.headerSent = true
		c.ResponseWriter.WriteHeader(code)
	}
}

func (c *streamCaptureWriter) Flush() {
	if c.flusher != nil {
		c.flusher.Flush()
	}
}

func (c *streamCaptureWriter) bytes() []byte {
	if c.buf == nil {
		return nil
	}
	return c.buf.Bytes()
}

// streamUsage collects token counts from the SSE chunks that pass through a
// stream. OpenAI streams carry usage in the final chunk's "usage" object (when
// stream_options.include_usage was requested); Anthropic streams carry
// input/cache tokens in the message_start event and output tokens in the
// message_delta event.
type sseUsage struct {
	Prompt              int `json:"prompt_tokens"`
	Completion          int `json:"completion_tokens"`
	Total               int `json:"total_tokens"`
	Input               int `json:"input_tokens"`
	Output              int `json:"output_tokens"`
	PromptTokensDetails struct {
		Cached int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

func parseSSEUsage(data []byte, st *streamStats) {
	var chunk struct {
		Usage   json.RawMessage `json:"usage"`
		Type    string          `json:"type"`
		Message struct {
			Usage json.RawMessage `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(data, &chunk) != nil {
		return
	}
	rawUsage := chunk.Usage
	isMessageStart := chunk.Type == "message_start"
	if isMessageStart {
		rawUsage = chunk.Message.Usage
	}
	if len(rawUsage) == 0 || string(rawUsage) == "null" {
		return
	}
	var u sseUsage
	if json.Unmarshal(rawUsage, &u) != nil {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawUsage, &fields) != nil {
		return
	}
	st.usageReported = true
	if _, ok := fields["prompt_tokens"]; ok {
		st.promptReported = true
	}
	if _, ok := fields["input_tokens"]; ok {
		st.promptReported = true
	}
	if !isMessageStart {
		if _, ok := fields["completion_tokens"]; ok {
			st.completionReported = true
		}
		if _, ok := fields["output_tokens"]; ok {
			st.completionReported = true
		}
	}
	if u.Prompt > st.prompt {
		st.prompt = u.Prompt
	}
	if u.Input > st.prompt {
		st.prompt = u.Input
	}
	if u.Completion > st.completion {
		st.completion = u.Completion
	}
	if u.Output > st.completion {
		st.completion = u.Output
	}
	if u.PromptTokensDetails.Cached > st.cached {
		st.cached = u.PromptTokensDetails.Cached
	}
	if u.CacheReadInputTokens > st.cached {
		st.cached = u.CacheReadInputTokens
	}
	st.usageComplete = st.promptReported && st.completionReported && (st.prompt > 0 || st.completion > 0)
}

func estimatedStreamUsage(body []byte, maxTokens int) (prompt, completion int) {
	completion, _ = resolveGatewayMaxTokens(maxTokens)
	prompt = len(body) / 3
	return prompt, completion
}

func (s *Service) streamResponse(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "internal_error", "streaming unsupported")
		return streamStats{}, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", contentType(resp.Header.Get("Content-Type")))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	var st streamStats
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := fmt.Fprintln(w, line); err != nil {
			return st, err
		}
		flusher.Flush()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data != "" && data != "[DONE]" {
				parseSSEUsage([]byte(data), &st)
			}
		}
	}
	return st, scanner.Err()
}

// logRequest writes the audit/quota trail. It stays synchronous because usage_records
// has a foreign key onto request_logs, but runs on a detached context so a client that
// hangs up mid-stream does not silently lose its log row (and its quota accounting).
// The api_keys.last_used_at stamp it used to duplicate is now handled by the api
// middleware.
type subscriptionCoveredKey struct{}

func (s *Service) logRequest(ctx context.Context, key keyContext, channelID int64, channelKeyID, model string, status, prompt, completion, total int, d time.Duration, errorCode, detail string) {
	if s.db == nil {
		return
	}
	id, _ := randomID()
	info := clientInfoFromContext(ctx)
	detail = sanitizeErrorDetail(detail)
	subscriptionAccess, _ := ctx.Value(subscriptionCoveredKey{}).(subscriptionAccess)
	logCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, err := s.db.Exec(logCtx, `insert into request_logs(id,request_id,user_id,api_key_id,channel_id,channel_key_id,group_id,model,status_code,prompt_tokens,completion_tokens,total_tokens,duration_ms,error_code,client_ip,user_agent,error_detail,subscription_covered) values($1::uuid,$2::text,$3::bigint,$4::uuid,nullif($5,0),nullif($6,'')::uuid,nullif($7,'')::uuid,$8,$9::int,$10::int,$11::int,$12::int,$13::int,nullif($14,''),$15,$16,$17,$18)`, id, requestID(ctx), key.userID, key.keyID, channelID, channelKeyID, key.groupID, model, status, prompt, completion, total, d.Milliseconds(), errorCode, info.ip, info.userAgent, detail, subscriptionAccess.Covered)
	if err != nil {
		log.Printf("logRequest failed: %v", err)
	}
}

var upstreamURLPattern = regexp.MustCompile(`https?://[^\s"'\])\}]+`)

// classifyContextError maps a context-cancellation or deadline-exceeded error
// (client hangup, RequestTimeout elapsed) to a stable error code and a clean,
// channel-agnostic message. The boolean reports whether err is such an error.
// These errors must not count against channel health: the channel itself is
// fine, only the request was cut short.
func classifyContextError(err error) (string, string, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout", "upstream request timed out", true
	}
	if errors.Is(err, context.Canceled) {
		return "client_canceled", "request canceled by client", true
	}
	return "", "", false
}

// noChannelAvailableDetail is the generic message relayed to clients when an
// upstream error matches an auto-disable keyword, so the upstream's specific
// account, quota, or token text is never shown verbatim.
const noChannelAvailableDetail = "no channel is currently available"

// clientUpstreamError rewrites an upstream error before it is relayed to the
// client. Errors matching an auto-disable keyword have their message replaced by
// the generic no-channel notice (the OpenAI-style JSON error shape is preserved
// when present, so clients that parse error.message keep working); URLs in any
// other error are swapped for the site's public origin so upstream endpoints are
// never exposed. The request log still records the caller's original detail.
func (s *Service) clientUpstreamError(ctx context.Context, detail string, reliability reliabilitySettings) string {
	if reliability.autoDisableKeyword(detail) {
		return rewriteErrorMessage(detail, noChannelAvailableDetail)
	}
	if upstreamURLPattern.MatchString(detail) {
		if base := s.loadPublicBaseURL(ctx); base != "" {
			detail = rewriteUpstreamURLs(detail, base)
		}
	}
	return detail
}

// rewriteErrorMessage replaces the "message" field of an OpenAI-style JSON error
// body; non-JSON bodies are replaced wholesale with the message itself.
func rewriteErrorMessage(body, message string) string {
	var payload map[string]any
	if json.Unmarshal([]byte(body), &payload) == nil {
		if errObj, ok := payload["error"].(map[string]any); ok {
			errObj["message"] = message
			if out, err := json.Marshal(payload); err == nil {
				return string(out)
			}
		}
	}
	return message
}

// rewriteUpstreamURLs swaps every URL in an upstream error text for the site's
// public origin. An empty publicBase leaves the text untouched.
func rewriteUpstreamURLs(detail, publicBase string) string {
	if publicBase == "" {
		return detail
	}
	return upstreamURLPattern.ReplaceAllString(detail, publicBase)
}

// sanitizeErrorDetail strips URLs (which would leak upstream endpoints) and bounds the
// length before an upstream error message is stored on the request log.
func sanitizeErrorDetail(detail string) string {
	if len(detail) > 4096 {
		detail = detail[:4096]
	}
	detail = upstreamURLPattern.ReplaceAllString(detail, "[url]")
	detail = strings.Join(strings.Fields(detail), " ")
	if r := []rune(detail); len(r) > 500 {
		detail = string(r[:500])
	}
	return detail
}
func usage(body []byte) (prompt, completion, total, cached int) {
	var v struct {
		Usage struct {
			Prompt              int `json:"prompt_tokens"`
			Completion          int `json:"completion_tokens"`
			Total               int `json:"total_tokens"`
			Input               int `json:"input_tokens"`
			Output              int `json:"output_tokens"`
			PromptTokensDetails struct {
				Cached int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &v) != nil {
		return 0, 0, 0, 0
	}
	prompt, completion, total = v.Usage.Prompt, v.Usage.Completion, v.Usage.Total
	if prompt == 0 && completion == 0 {
		prompt, completion = v.Usage.Input, v.Usage.Output
	}
	if total == 0 {
		total = prompt + completion
	}
	cached = v.Usage.PromptTokensDetails.Cached
	if cached == 0 && v.Usage.CacheReadInputTokens > 0 {
		cached = v.Usage.CacheReadInputTokens
	}
	return prompt, completion, total, cached
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
