package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type conversationCacheSettings struct {
	Enabled bool `json:"conversation_cache_enabled"`
}

const conversationCacheRetentionSQL = "24 hours"

func (s *Service) conversationCacheSettings(ctx context.Context) conversationCacheSettings {
	if s.conversationCacheData != nil {
		settings, err := s.conversationCacheData.get(ctx, struct{}{}, func(ctx context.Context) (conversationCacheSettings, error) {
			return s.loadConversationCacheSettings(ctx), nil
		})
		if err == nil {
			return settings
		}
	}
	return s.loadConversationCacheSettings(ctx)
}

func (s *Service) loadConversationCacheSettings(ctx context.Context) conversationCacheSettings {
	var enabled bool
	err := s.db.QueryRow(ctx, `select conversation_cache_enabled from site_settings where id=true`).Scan(&enabled)
	if err != nil {
		return conversationCacheSettings{}
	}
	return conversationCacheSettings{Enabled: enabled}
}

func (s *Service) getConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.loadConversationCacheSettings(r.Context()))
}

func (s *Service) updateConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool `json:"conversation_cache_enabled"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid conversation cache settings")
		return
	}
	current := s.loadConversationCacheSettings(r.Context())
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set conversation_cache_enabled=$1, updated_at=now() where id=true`, current.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save conversation cache settings")
		return
	}
	if s.conversationCacheData != nil {
		s.conversationCacheData.clear()
	}
	s.audit(r, "conversation_cache.updated", "site_settings", "conversation_cache", map[string]any{"enabled": current.Enabled})
	writeJSON(w, http.StatusOK, current)
}

func (s *Service) storeConversationCache(ctx context.Context, key keyContext, model string, stream bool, request, response []byte, statusCode int, durationMs int64) {
	settings := s.conversationCacheSettings(ctx)
	if !settings.Enabled {
		return
	}
	if len(request) == 0 {
		return
	}
	var reqJSON, respJSON json.RawMessage
	if json.Valid(request) {
		reqJSON = json.RawMessage(request)
	} else {
		reqJSON = json.RawMessage("null")
	}
	if len(response) > 0 && json.Valid(response) {
		respJSON = json.RawMessage(response)
	}
	id, _ := randomID()
	cacheCtx, cancel := detach(ctx, settlementTimeout)
	defer cancel()
	_, err := s.db.Exec(cacheCtx, `insert into conversation_cache(id,request_id,user_id,api_key_id,model,status_code,stream,request_body,response_body,duration_ms) values($1,$2,nullif($3,'')::bigint,$4::uuid,$5,$6,$7,$8,$9,$10)`,
		id, requestID(ctx), key.userID, nullIfEmpty(key.keyID), model, statusCode, stream, []byte(reqJSON), []byte(respJSON), durationMs)
	if err != nil {
		log.Printf("conversation cache insert: %v", err)
	}
}

func nullIfEmpty(s string) string {
	if s == "" {
		return ""
	}
	return s
}

func (s *Service) listConversationCache(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiOrDefault(q.Get("page"), 1)
	pageSize := atoiOrDefault(q.Get("page_size"), 50)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	where := "where 1=1"
	args := []any{}
	idx := 1
	if v := q.Get("user_id"); v != "" {
		where += " and user_id::text = $" + itoa(idx)
		args = append(args, v)
		idx++
	}
	if v := q.Get("model"); v != "" {
		where += " and model = $" + itoa(idx)
		args = append(args, v)
		idx++
	}
	if v := q.Get("start"); v != "" {
		where += " and created_at >= $" + itoa(idx)
		args = append(args, v)
		idx++
	}
	if v := q.Get("end"); v != "" {
		where += " and created_at < $" + itoa(idx)
		args = append(args, v)
		idx++
	}
	var total int
	countErr := s.db.QueryRow(r.Context(), `select count(*) from conversation_cache `+where, args...).Scan(&total)
	if countErr != nil {
		total = 0
	}
	offset := (page - 1) * pageSize
	queryArgs := append(args, pageSize, offset)
	rows, err := s.db.Query(r.Context(), `select id,request_id,coalesce(user_id::text,''),coalesce(api_key_id::text,''),model,status_code,stream,duration_ms,created_at from conversation_cache `+where+` order by created_at desc limit $`+itoa(idx)+` offset $`+itoa(idx+1), queryArgs...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, requestID, userID, apiKeyID, model string
		var statusCode, durationMs int
		var stream bool
		var createdAt any
		if rows.Scan(&id, &requestID, &userID, &apiKeyID, &model, &statusCode, &stream, &durationMs, &createdAt) == nil {
			data = append(data, map[string]any{"id": id, "request_id": requestID, "user_id": userID, "api_key_id": apiKeyID, "model": model, "status_code": statusCode, "stream": stream, "duration_ms": durationMs, "created_at": createdAt})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func (s *Service) getConversationCacheDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "id is required")
		return
	}
	var requestID, userID, apiKeyID, model string
	var statusCode, durationMs int
	var stream bool
	var requestBody, responseBody []byte
	var createdAt any
	err := s.db.QueryRow(r.Context(), `select request_id,coalesce(user_id::text,''),coalesce(api_key_id::text,''),model,status_code,stream,duration_ms,request_body,coalesce(response_body,'[]'::jsonb),created_at from conversation_cache where id=$1`, id).Scan(&requestID, &userID, &apiKeyID, &model, &statusCode, &stream, &durationMs, &requestBody, &responseBody, &createdAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "request_id": requestID, "user_id": userID, "api_key_id": apiKeyID, "model": model, "status_code": statusCode, "stream": stream, "duration_ms": durationMs, "request_body": json.RawMessage(requestBody), "response_body": json.RawMessage(responseBody), "created_at": createdAt})
}

func atoiOrDefault(s string, fallback int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	if s == "" {
		return fallback
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (s *Service) startConversationCacheCleanup(ctx context.Context) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				s.cleanupConversationCache(ctx)
			}
		}
	}()
}

func (s *Service) cleanupConversationCache(ctx context.Context) {
	if s.db == nil {
		return
	}
	tag, err := s.db.Exec(ctx, `delete from conversation_cache where created_at < now() - $1::interval`, conversationCacheRetentionSQL)
	if err != nil {
		log.Printf("conversation cache cleanup: %v", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		log.Printf("conversation cache cleanup: removed %d rows older than %s", n, conversationCacheRetentionSQL)
	}
}
