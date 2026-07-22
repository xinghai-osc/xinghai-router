package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSyncNewAPIPricingRejectsInvalidSourceBeforeNetworkOrDatabaseAccess(t *testing.T) {
	for _, body := range []string{"{}", `{"base_url":"http://example.com","price_per_quota_unit":1}`, `{"base_url":"https://example.com","price_per_quota_unit":-1}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/pricing/newapi/sync", strings.NewReader(body))
		(&Service{}).syncNewAPIPricing(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestNewAPIPricingDecodesExpectedFields(t *testing.T) {
	var pricing newAPIPricing
	if err := json.Unmarshal([]byte(`{"model_name":"kimi-k3","quota_type":0,"model_ratio":0.075,"completion_ratio":4,"cache_ratio":0.5}`), &pricing); err != nil {
		t.Fatal(err)
	}
	if pricing.ModelName != "kimi-k3" || pricing.ModelRatio != 0.075 || pricing.CompletionRatio != 4 || pricing.CacheRatio == nil || *pricing.CacheRatio != 0.5 {
		t.Fatalf("unexpected pricing: %+v", pricing)
	}
}

func TestNewAPIPricePerMillionUsesQuotaPerUnit(t *testing.T) {
	if actual := newAPIPricePerMillion(0.15, 1, 500000); actual != 0.3 {
		t.Fatalf("price = %v, want 0.3", actual)
	}
}

func TestUpdateUserRejectsInvalidPartialUpdatesBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty update", body: `{}`},
		{name: "unknown field", body: `{"id":"user-id"}`},
		{name: "empty name", body: `{"name":"  "}`},
		{name: "invalid email", body: `{"email":"invalid"}`},
		{name: "invalid role", body: `{"role":"owner"}`},
		{name: "short password", body: `{"password":"short"}`},
		{name: "invalid permission", body: `{"permissions":["unknown"]}`},
		{name: "negative balance", body: `{"balance":-1}`},
		{name: "note without balance", body: `{"note":"adjustment"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/admin/users/user-id", strings.NewReader(test.body))

			(&Service{}).updateUser(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestUpdateChannelRejectsInvalidRequestBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"channel","base_url":"https://api.example.com","models":[]}`,
		`{"name":"channel","base_url":"http://api.example.com","models":["model"]}`,
		`{"name":"channel","base_url":"https://api.example.com","models":["model"],"provider":"unknown"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/channels/channel-id", strings.NewReader(body))

		(&Service{}).updateChannel(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestCreateChannelRejectsInvalidPriorityBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"priority":10001}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"priority":-10001}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"provider":"unknown"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
		(&Service{}).createChannel(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestUpdateChannelRejectsInvalidPriorityBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{"name":"channel","base_url":"https://api.example.com","models":["m"],"priority":10001}`,
		`{"name":"channel","base_url":"https://api.example.com","models":["m"],"priority":-10001}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin/channels/channel-id", strings.NewReader(body))
		(&Service{}).updateChannel(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestValidChannelProviderAndPriority(t *testing.T) {
	for _, p := range []string{"openai", "ollama", "kimi", "opencode_go", "anthropic"} {
		if !validChannelProvider(p) {
			t.Fatalf("expected provider %q valid", p)
		}
	}
	if validChannelProvider("azure") || validChannelProvider("") {
		t.Fatal("unknown provider must be invalid")
	}
	if !validChannelPriority(0) || !validChannelPriority(-10000) || !validChannelPriority(10000) {
		t.Fatal("boundary priorities must be valid")
	}
	if validChannelPriority(-10001) || validChannelPriority(10001) {
		t.Fatal("out-of-range priority must be invalid")
	}
}
