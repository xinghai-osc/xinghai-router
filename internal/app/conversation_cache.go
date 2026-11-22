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
	"sync"
	"time"
)

type conversationCacheSettings struct {
	Enabled        bool `json:"conversation_cache_enabled"`
	Retain         bool `json:"conversation_cache_retain"`
	RetentionHours int  `json:"conversation_cache_retention_hours"`
}

const defaultConversationCacheRetentionHours = 24
const maxConversationCacheRetentionHours = 87600

func (s conversationCacheSettings) cutoff(now time.Time) time.Time {
	if s.Retain {
		return time.Time{}
	}
	hours := s.RetentionHours
	if hours < 1 || hours > maxConversationCacheRetentionHours {
		hours = defaultConversationCacheRetentionHours
	}
	return now.Add(-time.Duration(hours) * time.Hour)
}

func conversationCacheExpired(createdAt, cutoff time.Time) bool {
	return !cutoff.IsZero() && !createdAt.After(cutoff)
}

func filterConversationCacheEntries(entries []conversationCacheSummary, cutoff time.Time) []conversationCacheSummary {
	filtered := make([]conversationCacheSummary, 0, len(entries))
	for _, entry := range entries {
		if !conversationCacheExpired(entry.CreatedAt, cutoff) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
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
	FirstTokenMS *int            `json:"first_token_ms,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	RequestBody  json.RawMessage `json:"request_body"`
	ResponseBody json.RawMessage `json:"response_body"`
}

// conversationCacheSummary deliberately excludes request and response bodies.
// List requests only need metadata, so decoding every full cache file would
// retain all cached bodies until the scan completes.
type conversationCacheSummary struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	UserID       string    `json:"user_id"`
	APIKeyID     string    `json:"api_key_id"`
	Model        string    `json:"model"`
	StatusCode   int       `json:"status_code"`
	Stream       bool      `json:"stream"`
	DurationMS   int64     `json:"duration_ms"`
	FirstTokenMS *int      `json:"first_token_ms,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type conversationCacheIndex struct {
	mu       sync.Mutex
	entries  []conversationCacheSummary
	loaded   time.Time
	settings conversationCacheSettings
}

const conversationCacheIndexTTL = 15 * time.Second

func newConversationCacheIndex() *conversationCacheIndex {
	return &conversationCacheIndex{}
}

func (i *conversationCacheIndex) invalidate() {
	if i == nil {
		return
	}
	i.mu.Lock()
	i.loaded = time.Time{}
	i.mu.Unlock()
}

func (i *conversationCacheIndex) add(entry conversationCacheSummary) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, existing := range i.entries {
		if existing.ID == entry.ID {
			return
		}
	}
	i.entries = append(i.entries, entry)
}

func (i *conversationCacheIndex) get(dir string, now time.Time, settings conversationCacheSettings) ([]conversationCacheSummary, error) {
	if i == nil {
		return readConversationFiles(dir, now, settings)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.loaded.IsZero() && now.Sub(i.loaded) < conversationCacheIndexTTL && i.settings == settings {
		return filterConversationCacheEntries(i.entries, settings.cutoff(now)), nil
	}
	entries, err := readConversationFiles(dir, now, settings)
	if err != nil {
		return nil, err
	}
	i.entries = entries
	i.loaded = now
	i.settings = settings
	return filterConversationCacheEntries(i.entries, settings.cutoff(now)), nil
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
	settings, _ := s.readConversationCacheSettings(ctx)
	return settings
}

func (s *Service) readConversationCacheSettings(ctx context.Context) (conversationCacheSettings, error) {
	settings := conversationCacheSettings{RetentionHours: defaultConversationCacheRetentionHours}
	if s.db == nil {
		return settings, nil
	}
	err := s.db.QueryRow(ctx, `select conversation_cache_enabled, conversation_cache_retain, conversation_cache_retention_hours from site_settings where id=true`).Scan(&settings.Enabled, &settings.Retain, &settings.RetentionHours)
	return settings, err
}

func (s *Service) getConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.readConversationCacheSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load conversation cache settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Service) updateConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled        *bool `json:"conversation_cache_enabled"`
		Retain         *bool `json:"conversation_cache_retain"`
		RetentionHours *int  `json:"conversation_cache_retention_hours"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid conversation cache settings")
		return
	}
	if in.RetentionHours != nil && (*in.RetentionHours < 1 || *in.RetentionHours > maxConversationCacheRetentionHours) {
		writeError(w, http.StatusBadRequest, "invalid_request", "conversation cache retention must be between 1 and 87600 hours")
		return
	}
	current, err := s.readConversationCacheSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load conversation cache settings")
		return
	}
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	if in.Retain != nil {
		current.Retain = *in.Retain
	}
	if in.RetentionHours != nil {
		current.RetentionHours = *in.RetentionHours
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set conversation_cache_enabled=$1, conversation_cache_retain=$2, conversation_cache_retention_hours=$3, updated_at=now() where id=true`, current.Enabled, current.Retain, current.RetentionHours); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save conversation cache settings")
		return
	}
	if s.conversationCacheData != nil {
		s.conversationCacheData.clear()
	}
	if s.conversationCacheIndex != nil {
		s.conversationCacheIndex.invalidate()
	}
	s.audit(r, "conversation_cache.updated", "site_settings", "conversation_cache", map[string]any{"enabled": current.Enabled, "retain": current.Retain, "retention_hours": current.RetentionHours})
	writeJSON(w, http.StatusOK, current)
}

func (s *Service) storeConversationCache(ctx context.Context, key keyContext, model string, stream bool, request, response []byte, statusCode int, durationMs int64, firstTokenMs *int) {
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
		FirstTokenMS: firstTokenMs,
		CreatedAt:    time.Now(),
		RequestBody:  reqJSON,
		ResponseBody: respJSON,
	}
	go func() {
		if err := writeConversationFile(s.cfg.ConversationCacheDir, entry); err != nil {
			log.Printf("conversation cache write: %v", err)
			return
		}
		if s.conversationCacheIndex != nil {
			s.conversationCacheIndex.add(conversationCacheSummary{
				ID: entry.ID, RequestID: entry.RequestID, UserID: entry.UserID,
				APIKeyID: entry.APIKeyID, Model: entry.Model, StatusCode: entry.StatusCode,
				Stream: entry.Stream, DurationMS: entry.DurationMS, FirstTokenMS: entry.FirstTokenMS,
				CreatedAt: entry.CreatedAt,
			})
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

func secureConversationCacheDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

func writeConversationFile(dir string, entry conversationFile) error {
	if err := secureConversationCacheDir(dir); err != nil {
		return err
	}
	sub := filepath.Join(dir, entry.CreatedAt.Format("2006-01-02"))
	if err := os.MkdirAll(sub, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sub, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := filepath.Join(sub, entry.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Service) listConversationCache(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize, offset := listPage(r)
	userFilter := strings.TrimSpace(q.Get("user_id"))
	modelFilter := strings.TrimSpace(q.Get("model"))
	var start, end time.Time
	if v := q.Get("start"); v != "" {
		start, _ = time.Parse(time.RFC3339, v)
	}
	if v := q.Get("end"); v != "" {
		end, _ = time.Parse(time.RFC3339, v)
	}
	entries, err := s.conversationCacheIndex.get(s.conversationCacheDir(), time.Now(), s.conversationCacheSettings(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read conversation cache")
		return
	}
	filtered := make([]conversationCacheSummary, 0, len(entries))
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
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	total := len(filtered)
	if offset >= total {
		filtered = nil
	} else if total-offset <= pageSize {
		filtered = filtered[offset:]
	} else {
		filtered = filtered[offset : offset+pageSize]
	}
	data := make([]map[string]any, 0, len(filtered))
	for _, e := range filtered {
		data = append(data, conversationSummary(e))
	}
	writePaged(w, data, total, page, pageSize)
}

func validConversationCacheID(id string) bool {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return false
	}
	for _, char := range id {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func (s *Service) getConversationCacheDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validConversationCacheID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid conversation id")
		return
	}
	entry, err := findConversationFile(s.conversationCacheDir(), id, s.conversationCacheSettings(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":             entry.ID,
		"request_id":     entry.RequestID,
		"user_id":        entry.UserID,
		"api_key_id":     entry.APIKeyID,
		"model":          entry.Model,
		"status_code":    entry.StatusCode,
		"stream":         entry.Stream,
		"duration_ms":    entry.DurationMS,
		"first_token_ms": entry.FirstTokenMS,
		"request_body":   json.RawMessage(entry.RequestBody),
		"response_body":  json.RawMessage(entry.ResponseBody),
		"created_at":     entry.CreatedAt,
	})
}

func readConversationFiles(dir string, now time.Time, settings conversationCacheSettings) ([]conversationCacheSummary, error) {
	var entries []conversationCacheSummary
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	cutoff := settings.cutoff(now)
	for _, day := range dayDirs {
		if !day.IsDir() || !validConversationCacheDay(day.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, day.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.Type().IsRegular() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			file, err := os.Open(filepath.Join(dir, day.Name(), f.Name()))
			if err != nil {
				continue
			}
			var e conversationCacheSummary
			err = json.NewDecoder(file).Decode(&e)
			_ = file.Close()
			if err == nil && !conversationCacheExpired(e.CreatedAt, cutoff) {
				entries = append(entries, e)
			}
		}
	}
	return entries, nil
}

func validConversationCacheDay(name string) bool {
	_, err := time.Parse("2006-01-02", name)
	return err == nil
}

func findConversationFile(dir, id string, settings conversationCacheSettings) (conversationFile, error) {
	var zero conversationFile
	if !validConversationCacheID(id) {
		return zero, os.ErrNotExist
	}
	cutoff := settings.cutoff(time.Now())
	days, err := os.ReadDir(dir)
	if err != nil {
		return zero, err
	}
	for i := len(days) - 1; i >= 0; i-- {
		day := days[i]
		if !day.IsDir() || !validConversationCacheDay(day.Name()) {
			continue
		}
		path := filepath.Join(dir, day.Name(), id+".json")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var entry conversationFile
		if err := json.Unmarshal(data, &entry); err != nil {
			return zero, err
		}
		if conversationCacheExpired(entry.CreatedAt, cutoff) {
			return zero, os.ErrNotExist
		}
		return entry, nil
	}
	return zero, os.ErrNotExist
}

func (s *Service) startConversationCacheCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			s.cleanupConversationCache(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) cleanupConversationCache(ctx context.Context) {
	dir := s.conversationCacheDir()
	if dir == "" {
		return
	}
	settings, err := s.readConversationCacheSettings(ctx)
	if err != nil {
		// Deleting on an unknown retention setting could discard history the
		// operator asked to keep, so a failed read leaves the records alone.
		log.Printf("conversation cache cleanup: could not read settings, skipping: %v", err)
		return
	}
	if removed := cleanupConversationCacheDir(dir, time.Now(), settings); removed > 0 {
		s.conversationCacheIndex.invalidate()
		log.Printf("conversation cache cleanup: removed %d expired records in %s", removed, dir)
	}
}

func cleanupConversationCacheDir(dir string, now time.Time, settings conversationCacheSettings) int64 {
	if settings.Retain {
		return 0
	}
	cutoff := settings.cutoff(now)
	var removed int64
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("conversation cache cleanup: %v", err)
		}
		return 0
	}
	for _, day := range dayDirs {
		if !day.IsDir() || !validConversationCacheDay(day.Name()) {
			continue
		}
		dayPath := filepath.Join(dir, day.Name())
		files, err := os.ReadDir(dayPath)
		if err != nil {
			log.Printf("conversation cache cleanup: %v", err)
			continue
		}
		for _, file := range files {
			if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			path := filepath.Join(dayPath, file.Name())
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			var entry conversationCacheSummary
			err = json.NewDecoder(f).Decode(&entry)
			_ = f.Close()
			if err != nil || entry.CreatedAt.IsZero() || !conversationCacheExpired(entry.CreatedAt, cutoff) {
				continue
			}
			if err := os.Remove(path); err != nil {
				if !os.IsNotExist(err) {
					log.Printf("conversation cache cleanup: %v", err)
				}
				continue
			}
			removed++
		}
		_ = os.Remove(dayPath)
	}
	return removed
}

func conversationSummary(e conversationCacheSummary) map[string]any {
	return map[string]any{
		"id":             e.ID,
		"request_id":     e.RequestID,
		"user_id":        e.UserID,
		"api_key_id":     e.APIKeyID,
		"model":          e.Model,
		"status_code":    e.StatusCode,
		"stream":         e.Stream,
		"duration_ms":    e.DurationMS,
		"first_token_ms": e.FirstTokenMS,
		"created_at":     e.CreatedAt,
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
