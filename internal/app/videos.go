package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxVideoTaskBody = 1 << 20

func validVideoTaskID(id string) bool {
	if len(id) == 0 || len(id) > 256 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func parseVideoTask(body []byte) (string, string, error) {
	var task struct {
		ID     string          `json:"id"`
		Status string          `json:"status"`
		Error  json.RawMessage `json:"error"`
	}
	if len(body) > maxVideoTaskBody || json.Unmarshal(body, &task) != nil || !validVideoTaskID(task.ID) {
		return "", "", fmt.Errorf("invalid video task response")
	}
	switch task.Status {
	case "queued", "in_progress", "completed", "failed":
	default:
		return "", "", fmt.Errorf("invalid video task status")
	}
	if task.Status != "failed" && len(task.Error) > 0 && string(task.Error) != "null" {
		return "", "", fmt.Errorf("video task response contains an error")
	}
	return task.ID, task.Status, nil
}

func videoBaseURL(base string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	u, err := url.Parse(base)
	if err != nil || validUpstreamURL(base) != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return "", fmt.Errorf("invalid video upstream base URL")
	}
	return base, nil
}

func videoTaskResponse(body []byte, id string) ([]byte, error) {
	var task map[string]json.RawMessage
	if err := json.Unmarshal(body, &task); err != nil || task == nil {
		return nil, fmt.Errorf("invalid video task response")
	}
	task["id"], _ = json.Marshal(id)
	return json.Marshal(task)
}

func (s *Service) registerVideoTask(ctx context.Context, key keyContext, ch channel, model string, body []byte) ([]byte, error) {
	upstreamID, status, err := parseVideoTask(body)
	if err != nil {
		return nil, err
	}
	if status == "failed" {
		return nil, fmt.Errorf("upstream video task failed")
	}
	base, err := videoBaseURL(ch.baseURL)
	if err != nil {
		return nil, err
	}
	if key.userID == "" || key.keyID == "" || key.workspaceID == "" || ch.id <= 0 || ch.apiKey == "" || model == "" {
		return nil, fmt.Errorf("missing video task ownership or channel")
	}
	id, err := randomID()
	if err != nil {
		return nil, fmt.Errorf("generate video task id: %w", err)
	}
	id = "video_" + id
	response, err := videoTaskResponse(body, id)
	if err != nil {
		return nil, err
	}
	upstreamModel := ch.upstreamModel
	if upstreamModel == "" {
		upstreamModel = model
	}
	persistCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, err = s.db.Exec(persistCtx, `insert into video_tasks(id,upstream_id,user_id,api_key_id,workspace_id,group_id,channel_id,channel_key_id,model,upstream_model,upstream_base_url,status,credential_hash,upstream_provider) values($1,$2,$3::bigint,$4::uuid,$5::uuid,nullif($6,'')::uuid,$7,nullif($8,'')::uuid,$9,$10,$11,$12,$13,$14)`, id, upstreamID, key.userID, key.keyID, key.workspaceID, key.groupID, ch.id, ch.keyID, model, upstreamModel, base, status, hashSecret(ch.apiKey), ch.provider)
	if err != nil {
		return nil, fmt.Errorf("persist video task: %w", err)
	}
	return response, nil
}

func (s *Service) getVideo(w http.ResponseWriter, r *http.Request) { s.proxyVideoTask(w, r, false) }
func (s *Service) getVideoContent(w http.ResponseWriter, r *http.Request) {
	s.proxyVideoTask(w, r, true)
}

func videoContentQuery(raw string, content bool) (string, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", fmt.Errorf("invalid video query")
	}
	for name, variants := range values {
		if !content || name != "variant" || len(variants) != 1 {
			return "", fmt.Errorf("unsupported video query")
		}
		switch variants[0] {
		case "video", "thumbnail", "spritesheet":
		default:
			return "", fmt.Errorf("invalid video content variant")
		}
	}
	return values.Encode(), nil
}

func (s *Service) proxyVideoTask(w http.ResponseWriter, r *http.Request, content bool) {
	started := time.Now()
	key := r.Context().Value(contextKey{}).(keyContext)
	ctx := r.Context()
	id := r.PathValue("id")
	if !validVideoTaskID(id) {
		writeError(w, 404, "not_found", "video task not found")
		return
	}
	query, err := videoContentQuery(r.URL.RawQuery, content)
	if err != nil {
		writeError(w, 400, "invalid_request", err.Error())
		return
	}
	var ch channel
	var model, base, credentialHash, upstreamID string
	err = s.db.QueryRow(ctx, `select upstream_id,channel_id,coalesce(channel_key_id::text,''),model,upstream_base_url,credential_hash,upstream_provider from video_tasks where id=$1 and user_id=$2::bigint and api_key_id=$3::uuid and workspace_id=$4::uuid`, id, key.userID, key.keyID, key.workspaceID).Scan(&upstreamID, &ch.id, &ch.keyID, &model, &base, &credentialHash, &ch.provider)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "video task not found")
		return
	}
	if err != nil {
		writeError(w, 500, "internal_error", "could not load video task")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v2/") && ch.provider != "minimax" {
		writeError(w, 404, "not_found", "video task not found")
		return
	}
	status, code, detail := 500, "internal_error", "video retrieval failed"
	defer func() {
		s.logRequest(ctx, key, ch.id, ch.keyID, model, status, 0, 0, 0, time.Since(started), code, detail)
	}()
	fail := func(n int, c, message string) { status, code, detail = n, c, message; writeError(w, n, c, message) }
	var stored, currentBase string
	err = s.db.QueryRow(ctx, `select c.base_url,case when $2='' then c.api_key else k.key_encrypted end,coalesce(c.max_concurrency,0) from channels c left join channel_api_keys k on k.channel_id=c.id and k.id=nullif($2,'')::uuid and k.enabled where c.id=$1 and c.enabled and not c.auto_disabled and ($2='' or k.id is not null)`, ch.id, ch.keyID).Scan(&currentBase, &stored, &ch.maxConcurrency)
	if err != nil {
		ch.id, ch.keyID = 0, ""
		fail(503, "video_channel_unavailable", "original video channel or credential unavailable")
		return
	}
	currentBase, err = videoBaseURL(currentBase)
	if err != nil || currentBase != base {
		fail(503, "video_channel_unavailable", "original video upstream changed")
		return
	}
	ch.apiKey, err = channelKeyValue(s.cfg.EncryptionKey, stored)
	if err != nil || strings.TrimSpace(ch.apiKey) == "" || hashSecret(ch.apiKey) != credentialHash {
		fail(503, "credential_unavailable", "original video credential unavailable")
		return
	}
	leaseCtx, release, blocked, err := s.acquireRequestConcurrency(ctx, key)
	if err != nil {
		fail(503, "concurrency_unavailable", "could not enforce concurrency limits")
		return
	}
	if blocked != "" {
		fail(429, blocked+"_concurrency_exceeded", blocked+" concurrency limit exceeded")
		return
	}
	defer release()
	ctx = leaseCtx
	leaseCtx, releaseChannel, busy, err := s.acquireChannelConcurrency(ctx, ch)
	if err != nil {
		fail(503, "concurrency_unavailable", "could not enforce channel concurrency")
		return
	}
	if busy {
		fail(429, "channel_concurrency_exceeded", "original video channel concurrency limit exceeded")
		return
	}
	defer releaseChannel()
	timeout := 30 * time.Second
	if content {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(leaseCtx, timeout)
	defer cancel()
	endpoint := base + "/v1/videos/" + upstreamID
	if content {
		endpoint += "/content"
	}
	if query != "" {
		endpoint += "?" + query
	}
	method := http.MethodGet
	if ch.provider == "minimax" {
		endpoint = base + "/v2/query/video_generation/" + upstreamID
		if r.Method == http.MethodDelete {
			method = http.MethodDelete
			endpoint = base + minimaxVideoPath + "/" + upstreamID
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		fail(503, "video_channel_unavailable", "invalid original video upstream")
		return
	}
	req.Header.Set("Authorization", "Bearer "+ch.apiKey)
	req.Header.Set("Accept", "application/json")
	if content {
		req.Header.Set("Accept", "*/*")
		for _, name := range []string{"Range", "If-Range"} {
			if value := r.Header.Get(name); value != "" {
				req.Header.Set(name, value)
			}
		}
	}
	client := clientWithTimeout(s.streamClient, timeout)
	client.CheckRedirect = checkUpstreamRedirect
	resp, err := client.Do(req)
	if err != nil {
		if c, message, ok := classifyContextError(err); ok {
			fail(504, c, message)
		} else {
			fail(502, "upstream_error", "video upstream request failed")
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		status = resp.StatusCode
		if status < 400 {
			status = 502
		}
		code, detail = "upstream_error", adminErrorDetail(string(body), ch.apiKey)
		if readErr != nil {
			detail = "could not read video upstream error"
		}
		if content && status == http.StatusRequestedRangeNotSatisfiable {
			w.Header().Set("Content-Range", resp.Header.Get("Content-Range"))
		}
		writeError(w, status, code, "video upstream request failed")
		return
	}
	if ch.provider == "minimax" {
		status, err = s.writeMiniMaxVideoTask(w, r, resp, id, upstreamID, model, content)
		if err != nil {
			code, detail = "invalid_video_response", err.Error()
			if status == 0 {
				fail(502, code, "invalid MiniMax video task response")
			}
		} else {
			code, detail = "", ""
		}
		return
	}
	if !content {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxVideoTaskBody+1))
		taskID, _, parseErr := parseVideoTask(body)
		if readErr != nil || parseErr != nil || taskID != upstreamID {
			fail(502, "invalid_video_response", "invalid video task response")
			return
		}
		body, err = videoTaskResponse(body, id)
		if err != nil {
			fail(502, "invalid_video_response", "invalid video task response")
			return
		}
		status, code, detail = resp.StatusCode, "", ""
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "ETag", "Last-Modified"} {
		if value := resp.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	status, code, detail = resp.StatusCode, "", ""
	w.WriteHeader(status)
	if _, err := io.Copy(w, resp.Body); err != nil {
		status, code, detail = 502, "video_stream_error", "video content stream interrupted"
	}
}
