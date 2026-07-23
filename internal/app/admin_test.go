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

func TestValidChannelAPIKeyAndBaseURLLength(t *testing.T) {
	if !validChannelAPIKey("sk") || !validChannelAPIKey(strings.Repeat("k", maxChannelAPIKeyLen)) {
		t.Fatal("boundary api keys must be valid")
	}
	if validChannelAPIKey("") || validChannelAPIKey(strings.Repeat("k", maxChannelAPIKeyLen+1)) {
		t.Fatal("out-of-range api keys must be invalid")
	}
	if !validChannelBaseURLLength("https://api.example.com") || !validChannelBaseURLLength("http://127.0.0.1:11434") {
		t.Fatal("expected valid base URLs")
	}
	if validChannelBaseURLLength("") || validChannelBaseURLLength("http://api.example.com") || validChannelBaseURLLength("https://"+strings.Repeat("a", 2040)+".example.com") {
		t.Fatal("expected invalid base URLs")
	}
}

func TestValidGatewayModelName(t *testing.T) {
	if !validGatewayModelName("m") || !validGatewayModelName(strings.Repeat("m", maxGatewayModelLen)) {
		t.Fatal("boundary model names must be valid")
	}
	if validGatewayModelName("") || validGatewayModelName(strings.Repeat("m", maxGatewayModelLen+1)) {
		t.Fatal("out-of-range model names must be invalid")
	}
}

func TestCreateChannelRejectsOverlongSecretsBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{"name":"c","api_key":"","base_url":"https://api.example.com","models":["m"]}`,
		`{"name":"c","api_key":"` + strings.Repeat("k", 4097) + `","base_url":"https://api.example.com","models":["m"]}`,
		`{"name":"c","api_key":"sk","base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","models":["m"]}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
		(&Service{}).createChannel(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	}
}

func TestChatCompletionsRejectsLongModel(t *testing.T) {
	body := `{"model":"` + strings.Repeat("m", 201) + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	(&Service{}).chatCompletions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
