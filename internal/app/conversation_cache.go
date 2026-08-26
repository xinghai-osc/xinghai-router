package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type conversationCacheSettings struct {
	Enabled bool `json:"conversation_cache_enabled"`
}

// conversationFile is the on-disk JSON shape of one cached conversation.
type conversationFile struct {
	ID           string          `json:"id"`
	RequestID    string          `json:"request_id"`
	UserID       string          `json:"user_id"`
	APIKeyID     string          `json:"api_key_id"`
	Model        string          `json:"model"`
	StatusCode   int             `json:"status_code"`
	Stream       bool            `json:"stream"`
	DurationMS   int64           `json:"duration_ms"`
	CreatedAt    time.Time       `json:"created_at"`
	RequestBody  json.RawMessage `json:"request_body"`
	ResponseBody json.RawMessage `json:"response_body"`
}

func (s *Service) conversationCacheDir() string {
	return s.cfg.ConversationCacheDir
}

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
	if !settings.Enabled || !key.dataUsageEnabled {
		return
	}
	if len(request) == 0 {
		return
	}
	var reqJSON, respJSON json.RawMessage
	if json.Valid(request) {
		reqJSON = json.RawMessage(request)
	} else if len(request) > 0 {
		reqJSON, _ = json.Marshal(string(request))
	}
	if json.Valid(response) {
		respJSON = json.RawMessage(response)
	} else if len(response) > 0 {
		respJSON, _ = json.Marshal(string(response))
	}
	entry := conversationFile{
		ID:           randomIDString(),
		RequestID:    requestID(ctx),
		UserID:       key.userID,
		APIKeyID:     key.keyID,
		Model:        model,
		StatusCode:   statusCode,
		Stream:       stream,
		DurationMS:   durationMs,
		CreatedAt:    time.Now(),
		RequestBody:  reqJSON,
		ResponseBody: respJSON,
	}
	go func() {
		if err := writeConversationFile(s.cfg.ConversationCacheDir, entry); err != nil {
			log.Printf("conversation cache write: %v", err)
		}
	}()
}

func randomIDString() string {
	id, _ := randomID()
	if id == "" {
		return time.Now().Format("20060102150405.999999999")
	}
	return id
}

func writeConversationFile(dir string, entry conversationFile) error {
	sub := filepath.Join(dir, entry.CreatedAt.Format("2006-01-02"))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := filepath.Join(sub, entry.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
	userFilter := strings.TrimSpace(q.Get("user_id"))
	modelFilter := strings.TrimSpace(q.Get("model"))
	var start, end time.Time
	if v := q.Get("start"); v != "" {
		start, _ = time.Parse(time.RFC3339, v)
	}
	if v := q.Get("end"); v != "" {
		end, _ = time.Parse(time.RFC3339, v)
	}
	entries, err := readConversationFiles(s.conversationCacheDir(), time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read conversation cache")
		return
	}
	filtered := entries[:0]
	for _, e := range entries {
		if userFilter != "" && e.UserID != userFilter {
			continue
		}
		if modelFilter != "" && e.Model != modelFilter {
			continue
		}
		if !start.IsZero() && e.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && !e.CreatedAt.Before(end) {
			continue
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
	total := len(filtered)
	offset := (page - 1) * pageSize
	limit := pageSize
	if offset >= total {
		filtered = nil
	} else if offset+limit > total {
		filtered = filtered[offset:]
	} else {
		filtered = filtered[offset : offset+limit]
	}
	data := make([]map[string]any, 0, len(filtered))
	for _, e := range filtered {
		data = append(data, conversationSummary(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "page": page, "page_size": pageSize})
}

func (s *Service) getConversationCacheDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "id is required")
		return
	}
	entry, err := findConversationFile(s.conversationCacheDir(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":            entry.ID,
		"request_id":    entry.RequestID,
		"user_id":       entry.UserID,
		"api_key_id":    entry.APIKeyID,
		"model":         entry.Model,
		"status_code":   entry.StatusCode,
		"stream":        entry.Stream,
		"duration_ms":   entry.DurationMS,
		"request_body":  json.RawMessage(entry.RequestBody),
		"response_body": json.RawMessage(entry.ResponseBody),
		"created_at":    entry.CreatedAt,
	})
}

// readConversationFiles walks the cache directory and returns every record that
// is still within the retention window (today plus the immediately preceding day,
// so deletions at midnight never race an in-flight write).
func readConversationFiles(dir string, today time.Time) ([]conversationFile, error) {
	var entries []conversationFile
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	keep := map[string]bool{}
	for offset := -2; offset <= 0; offset++ {
		keep[today.AddDate(0, 0, offset).Format("2006-01-02")] = true
	}
	for _, day := range dayDirs {
		if !day.IsDir() || !keep[day.Name()] {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, day.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, day.Name(), f.Name()))
			if err != nil {
				continue
			}
			var e conversationFile
			if json.Unmarshal(data, &e) == nil {
				entries = append(entries, e)
			}
		}
	}
	return entries, nil
}

func findConversationFile(dir, id string) (conversationFile, error) {
	var zero conversationFile
	today := time.Now()
	var lastErr error
	for offset := 0; offset >= -2; offset-- {
		dayDir := filepath.Join(dir, today.AddDate(0, 0, offset).Format("2006-01-02"))
		data, err := os.ReadFile(filepath.Join(dayDir, id+".json"))
		if err == nil {
			var e conversationFile
			if json.Unmarshal(data, &e) == nil {
				return e, nil
			}
			return zero, err
		}
		lastErr = err
	}
	return zero, lastErr
}

// startConversationCacheCleanup deletes day directories that have aged past the
// retention window, scheduled to run at midnight.
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
	dir := s.conversationCacheDir()
	if dir == "" {
		return
	}
	today := time.Now()
	var removed int64
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("conversation cache cleanup: %v", err)
		}
		return
	}
	for _, day := range dayDirs {
		if !day.IsDir() {
			continue
		}
		when, err := time.Parse("2006-01-02", day.Name())
		if err != nil || when.AddDate(0, 0, 1).Before(today) {
			// Directory is unparseable or older than one full day: remove it.
			full := filepath.Join(dir, day.Name())
			os.RemoveAll(full)
			removed++
		}
	}
	if removed > 0 {
		log.Printf("conversation cache cleanup: removed %d stale day directories in %s", removed, dir)
	}
}

// conversationEntry is used both for disk persistence and for in-memory scans.
type conversationEntry = conversationFile

func conversationSummary(e conversationEntry) map[string]any {
	return map[string]any{
		"id":          e.ID,
		"request_id":  e.RequestID,
		"user_id":     e.UserID,
		"api_key_id":  e.APIKeyID,
		"model":       e.Model,
		"status_code": e.StatusCode,
		"stream":      e.Stream,
		"duration_ms": e.DurationMS,
		"created_at":  e.CreatedAt,
	}
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
