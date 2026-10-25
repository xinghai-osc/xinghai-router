package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestAccountActiveUsageScopesByWorkspace(t *testing.T) {
	service := &Service{}
	stop := service.trackActiveUsage("request-a", "workspace-a", "user-a", "model-a")
	defer stop()
	service.trackActiveUsage("request-b", "workspace-b", "user-b", "model-b")()

	req := httptest.NewRequest("GET", "/account/usage/active", nil)
	req = req.WithContext(context.WithValue(req.Context(), accountContextKey{}, accountContext{workspaceID: "workspace-a"}))
	response := httptest.NewRecorder()
	service.accountActiveUsage(response, req)

	var payload struct {
		Data []activeUsageEntry `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].RequestID != "request-a" || payload.Data[0].Model != "model-a" {
		t.Fatalf("unexpected active usage: %+v", payload.Data)
	}
}

func TestTrackActiveUsageRemovesRequest(t *testing.T) {
	service := &Service{}
	stop := service.trackActiveUsage("request-a", "workspace-a", "user-a", "model-a")
	stop()

	service.activeUsage.mu.Lock()
	defer service.activeUsage.mu.Unlock()
	if len(service.activeUsage.entries) != 0 {
		t.Fatalf("active entry was not removed: %+v", service.activeUsage.entries)
	}
}
