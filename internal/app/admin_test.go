package app

import (
	"encoding/json"
	"math"
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

func TestCreateChannelRejectsInvalidRequestBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":[]}`,
		`{"name":"channel","api_key":"sk","base_url":"http://api.example.com","models":["model"]}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"provider":"unknown"}`,
		`{"name":"","api_key":"sk","base_url":"https://api.example.com","models":["model"]}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
		(&Service{}).createChannel(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestUpsertPricingRejectsInvalidValuesBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"model":"","input_per_million":1,"cached_input_per_million":0,"output_per_million":1}`,
		`{"model":"m","input_per_million":-1,"cached_input_per_million":0,"output_per_million":1}`,
		`{"model":"m","input_per_million":1,"cached_input_per_million":0,"output_per_million":1,"multiplier":-1}`,
		`{"model":"m","input_per_million":1,"cached_input_per_million":0,"output_per_million":1,"multiplier":"nan"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/pricing", strings.NewReader(body))
		(&Service{}).upsertPricing(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestCreateAndUpdateGroupRejectInvalidMultipliers(t *testing.T) {
	for _, body := range []string{
		`{"name":"g","multiplier":-1}`,
		`{"name":"g","multiplier":"nan"}`,
		`{"name":"g","multiplier":"Inf"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/groups", strings.NewReader(body))
		(&Service{}).createGroup(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("createGroup body %s status = %d", body, recorder.Code)
		}
	}
	for _, body := range []string{
		`{"multiplier":-1}`,
		`{"multiplier":"nan"}`,
		`{"multiplier":"-Inf"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/groups/group-id", strings.NewReader(body))
		(&Service{}).updateGroup(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("updateGroup body %s status = %d", body, recorder.Code)
		}
	}
}

func TestSetUserRoleRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{`{}`, `{"role":"owner"}`, `{"role":""}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/users/1/role", strings.NewReader(body))
		(&Service{}).setUserRole(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, recorder.Code)
		}
	}
}

func TestValidFiniteHelpers(t *testing.T) {
	if !validNonNegativeFinite(0) || !validNonNegativeFinite(1.5) {
		t.Fatal("expected non-negative finite values accepted")
	}
	if validNonNegativeFinite(-0.1) || validNonNegativeFinite(math.NaN()) || validNonNegativeFinite(math.Inf(1)) {
		t.Fatal("expected invalid non-negative values rejected")
	}
	if !validPositiveFinite(0.01) || validPositiveFinite(0) || validPositiveFinite(math.Inf(-1)) {
		t.Fatal("unexpected positive finite validation")
	}
}

func TestMethodEnabled(t *testing.T) {
	methods := []paymentMethod{{Code: "alipay", Enabled: true}, {Code: "wxpay", Enabled: false}}
	if !methodEnabled(methods, "alipay") {
		t.Fatal("enabled method should match")
	}
	if methodEnabled(methods, "wxpay") || methodEnabled(methods, "missing") {
		t.Fatal("disabled or missing method must not match")
	}
}

func TestMaskName(t *testing.T) {
	if got := maskName("  Alice  "); got != "A***" {
		t.Fatalf("maskName = %q", got)
	}
	if got := maskName(""); got != "***" {
		t.Fatalf("empty maskName = %q", got)
	}
	if got := maskName("张三"); got != "张***" {
		t.Fatalf("unicode maskName = %q", got)
	}
}

func TestAdjustBalanceRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"user_id":"1","amount":0,"note":"x"}`,
		`{"user_id":"1","amount":1,"note":""}`,
		`{"user_id":"","amount":1,"note":"x"}`,
		`{"user_id":"1","amount":"NaN","note":"x"}`,
		`{"user_id":"1","amount":"Inf","note":"x"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/wallets/adjustments", strings.NewReader(body))
		(&Service{}).adjustBalance(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestRunMigrationRejectsEmptyDSN(t *testing.T) {
	for _, body := range []string{`{}`, `{"source_dsn":""}`, `{"source_driver":"mysql"}`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/migrate", strings.NewReader(body))
		(&Service{}).runMigration(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
	}
}

func TestRedactDSN(t *testing.T) {
	got := redactDSN("postgres://router:s3cret@postgres:5432/router?sslmode=disable")
	if strings.Contains(got, "s3cret") {
		t.Fatalf("password not redacted: %q", got)
	}
	if !strings.Contains(got, "***") && !strings.Contains(got, "%2A%2A%2A") {
		t.Fatalf("expected redaction marker: %q", got)
	}
	got = redactDSN("user:pass@tcp(127.0.0.1:3306)/db")
	if strings.Contains(got, ":pass@") {
		t.Fatalf("mysql-style dsn not redacted: %q", got)
	}
	if redactDSN("") != "" {
		t.Fatal("empty dsn should stay empty")
	}
}
