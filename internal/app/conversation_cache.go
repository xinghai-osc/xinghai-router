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
	Enabled bool `json:"conversation_cache_enabled"`
	Retain  bool `json:"conversation_cache_retain"`
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
	mu      sync.Mutex
	entries []conversationCacheSummary
	loaded  time.Time
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

func (i *conversationCacheIndex) get(dir string, now time.Time, retain bool) ([]conversationCacheSummary, error) {
	if i == nil {
		return readConversationFiles(dir, now, retain)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.loaded.IsZero() && now.Sub(i.loaded) < conversationCacheIndexTTL {
		return i.entries, nil
	}
	entries, err := readConversationFiles(dir, now, retain)
	if err != nil {
		return nil, err
	}
	i.entries = entries
	i.loaded = now
	return i.entries, nil
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
	var settings conversationCacheSettings
	if s.db == nil {
		return settings, nil
	}
	err := s.db.QueryRow(ctx, `select conversation_cache_enabled, conversation_cache_retain from site_settings where id=true`).Scan(&settings.Enabled, &settings.Retain)
	return settings, err
}

func (s *Service) getConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.loadConversationCacheSettings(r.Context()))
}

func (s *Service) updateConversationCacheSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool `json:"conversation_cache_enabled"`
		Retain  *bool `json:"conversation_cache_retain"`
	}
	if decode(r, &in) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid conversation cache settings")
		return
	}
	current := s.loadConversationCacheSettings(r.Context())
	if in.Enabled != nil {
		current.Enabled = *in.Enabled
	}
	if in.Retain != nil {
		current.Retain = *in.Retain
	}
	if _, err := s.db.Exec(r.Context(), `update site_settings set conversation_cache_enabled=$1, conversation_cache_retain=$2, updated_at=now() where id=true`, current.Enabled, current.Retain); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save conversation cache settings")
		return
	}
	if s.conversationCacheData != nil {
		s.conversationCacheData.clear()
	}
	if s.conversationCacheIndex != nil {
		s.conversationCacheIndex.invalidate()
	}
	s.audit(r, "conversation_cache.updated", "site_settings", "conversation_cache", map[string]any{"enabled": current.Enabled, "retain": current.Retain})
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
	entries, err := s.conversationCacheIndex.get(s.conversationCacheDir(), time.Now(), s.conversationCacheSettings(r.Context()).Retain)
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
	entry, err := findConversationFile(s.conversationCacheDir(), id, s.conversationCacheSettings(r.Context()).Retain)
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

// readConversationFiles walks the cache directory and returns every record that
// is still within the retention window (today plus the immediately preceding two
// days, so deletions at midnight never race an in-flight write). When retain is
// on the cleanup keeps older days, so every dated directory is read.
func readConversationFiles(dir string, today time.Time, retain bool) ([]conversationCacheSummary, error) {
	var entries []conversationCacheSummary
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	keep := map[string]bool{}
	if retain {
		for _, day := range dayDirs {
			if day.IsDir() && validConversationCacheDay(day.Name()) {
				keep[day.Name()] = true
			}
		}
	} else {
		for offset := -2; offset <= 0; offset++ {
			keep[today.AddDate(0, 0, offset).Format("2006-01-02")] = true
		}
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
			file, err := os.Open(filepath.Join(dir, day.Name(), f.Name()))
			if err != nil {
				continue
			}
			var e conversationCacheSummary
			err = json.NewDecoder(file).Decode(&e)
			_ = file.Close()
			if err == nil {
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

func findConversationFile(dir, id string, retain bool) (conversationFile, error) {
	var zero conversationFile
	today := time.Now()
	days := make([]string, 0, 3)
	if retain {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && validConversationCacheDay(entry.Name()) {
					days = append(days, entry.Name())
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(days)))
		}
	} else {
		for offset := 0; offset >= -2; offset-- {
			days = append(days, today.AddDate(0, 0, offset).Format("2006-01-02"))
		}
	}
	var lastErr error
	for _, day := range days {
		data, err := os.ReadFile(filepath.Join(dir, day, id+".json"))
		if err != nil {
			lastErr = err
			continue
		}
		var e conversationFile
		if err := json.Unmarshal(data, &e); err != nil {
			return zero, err
		}
		return e, nil
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return zero, lastErr
}

// startConversationCacheCleanup deletes day directories that have aged past the
// retention window, scheduled to run at midnight. When the admin enabled history
// retention the cleanup keeps every record instead.
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
	settings, err := s.readConversationCacheSettings(ctx)
	if err != nil {
		// Deleting on an unknown retention setting could discard history the
		// operator asked to keep, so a failed read leaves the records alone.
		log.Printf("conversation cache cleanup: could not read settings, skipping: %v", err)
		return
	}
	if settings.Retain {
		log.Printf("conversation cache cleanup: retention enabled, keeping records in %s", dir)
	}
	if removed := cleanupConversationCacheDir(dir, time.Now(), settings.Retain); removed > 0 {
		log.Printf("conversation cache cleanup: removed %d stale day directories in %s", removed, dir)
	}
}

func cleanupConversationCacheDir(dir string, today time.Time, retain bool) int64 {
	if retain {
		return 0
	}
	var removed int64
	dayDirs, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("conversation cache cleanup: %v", err)
		}
		return 0
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
