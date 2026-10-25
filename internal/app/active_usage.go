package app

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

type activeUsageEntry struct {
	RequestID   string    `json:"request_id"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"-"`
}

type activeUsageTracker struct {
	mu      sync.Mutex
	entries map[string]activeUsageEntry
}

func (s *Service) trackActiveUsage(requestID, workspaceID, userID, model string) func() {
	if requestID == "" || workspaceID == "" {
		return func() {}
	}
	tracker := &s.activeUsage
	tracker.mu.Lock()
	if tracker.entries == nil {
		tracker.entries = make(map[string]activeUsageEntry)
	}
	tracker.entries[requestID] = activeUsageEntry{RequestID: requestID, Model: model, CreatedAt: time.Now(), UserID: userID, WorkspaceID: workspaceID}
	tracker.mu.Unlock()

	return func() {
		tracker.mu.Lock()
		delete(tracker.entries, requestID)
		tracker.mu.Unlock()
	}
}

func (s *Service) accountActiveUsage(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	writeJSON(w, http.StatusOK, map[string]any{"data": s.activeUsageSnapshot(account.workspaceID)})
}

func (s *Service) adminActiveUsage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.activeUsageSnapshot("")})
}

func (s *Service) activeUsageSnapshot(workspaceID string) []activeUsageEntry {
	tracker := &s.activeUsage
	tracker.mu.Lock()
	data := make([]activeUsageEntry, 0, len(tracker.entries))
	for _, entry := range tracker.entries {
		if workspaceID == "" || entry.WorkspaceID == workspaceID {
			data = append(data, entry)
		}
	}
	tracker.mu.Unlock()
	sort.Slice(data, func(i, j int) bool { return data[i].CreatedAt.After(data[j].CreatedAt) })
	return data
}
