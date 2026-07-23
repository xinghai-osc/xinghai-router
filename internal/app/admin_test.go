package app

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		{name: "long password", body: `{"password":"` + strings.Repeat("a", 73) + `"}`},
		{name: "invalid permission", body: `{"permissions":["unknown"]}`},
		{name: "negative balance", body: `{"balance":-1}`},
		{name: "oversized balance", body: `{"balance":1000000000.01}`},
		{name: "note without balance", body: `{"note":"adjustment"}`},
		{name: "oversized note", body: `{"balance":1,"note":"` + strings.Repeat("n", 501) + `"}`},
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
		`{"name":"channel","api_key":"sk","base_url":"https://169.254.169.254","models":["model"]}`,
		`{"name":"channel","api_key":"sk","base_url":"https://10.0.0.8","models":["model"]}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"priority":10001}`,
		`{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":["model"],"priority":-10001}`,
		`{"name":"channel","api_key":"","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","api_key":"` + strings.Repeat("k", 4097) + `","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","api_key":"sk","base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","models":["model"]}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
		(&Service{}).createChannel(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestUpdateChannelRejectsInvalidPriorityBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{"name":"channel","base_url":"https://api.example.com","models":["m"],"priority":10001}`,
		`{"name":"channel","base_url":"https://api.example.com","models":["m"],"priority":-10001}`,
		`{"name":"channel","base_url":"https://api.example.com","models":["m"],"provider":"unknown"}`,
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

func TestSanitizeChannelModels(t *testing.T) {
	got, ok := sanitizeChannelModels([]string{" gpt-4 ", "", "gpt-4", "claude-3"})
	if !ok || len(got) != 2 || got[0] != "gpt-4" || got[1] != "claude-3" {
		t.Fatalf("sanitize = %#v ok=%v", got, ok)
	}
	if _, ok := sanitizeChannelModels(nil); ok {
		t.Fatal("empty models must fail")
	}
	if _, ok := sanitizeChannelModels([]string{" ", ""}); ok {
		t.Fatal("whitespace-only models must fail")
	}
	if _, ok := sanitizeChannelModels([]string{strings.Repeat("m", 201)}); ok {
		t.Fatal("overlong model name must fail")
	}
}

func TestCreateChannelRejectsEmptyModelsAfterSanitize(t *testing.T) {
	body := `{"name":"channel","api_key":"sk","base_url":"https://api.example.com","models":[" ","\t"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
	(&Service{}).createChannel(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateChannelRejectsPrivateHTTPSBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{"name":"channel","base_url":"https://169.254.169.254","models":["m"]}`,
		`{"name":"channel","base_url":"https://192.168.0.1","models":["m"]}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/channels/channel-id", strings.NewReader(body))
		(&Service{}).updateChannel(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestSyncNewAPIPricingRejectsPrivateBaseURL(t *testing.T) {
	body := `{"base_url":"https://10.0.0.1","price_per_quota_unit":1}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/pricing/newapi/sync", strings.NewReader(body))
	(&Service{}).syncNewAPIPricing(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestUpsertPricingRejectsInvalidValuesBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"model":"","input_per_million":1,"cached_input_per_million":0,"output_per_million":1}`,
		`{"model":"m","input_per_million":-1,"cached_input_per_million":0,"output_per_million":1}`,
		`{"model":"m","input_per_million":1,"cached_input_per_million":0,"output_per_million":1,"multiplier":-1}`,
		`{"model":"m","input_per_million":1,"cached_input_per_million":0,"output_per_million":1,"multiplier":"nan"}`,
		`{"model":"m","input_per_million":1,"cached_input_per_million":0,"output_per_million":1,"multiplier":1000.01}`,
		`{"model":"m","input_per_million":1000000.01,"cached_input_per_million":0,"output_per_million":1}`,
		`{"model":"` + strings.Repeat("m", 201) + `","input_per_million":1,"cached_input_per_million":0,"output_per_million":1}`,
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
		`{"name":"g","multiplier":1000.01}`,
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
		`{"multiplier":1001}`,
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
	if !validGroupMultiplier(0) || !validGroupMultiplier(maxGroupMultiplier) {
		t.Fatal("boundary group multipliers must be valid")
	}
	if validGroupMultiplier(-0.01) || validGroupMultiplier(maxGroupMultiplier+0.01) || validGroupMultiplier(math.NaN()) {
		t.Fatal("out-of-range group multipliers must be invalid")
	}
	if !validPricingMultiplier(0.01) || !validPricingMultiplier(maxPricingMultiplier) {
		t.Fatal("boundary pricing multipliers must be valid")
	}
	if validPricingMultiplier(0) || validPricingMultiplier(maxPricingMultiplier+0.01) {
		t.Fatal("out-of-range pricing multipliers must be invalid")
	}
	if !validPricingRate(0) || !validPricingRate(maxPricingRate) || validPricingRate(maxPricingRate+1) {
		t.Fatal("pricing rate bounds unexpected")
	}
	if !validPricingModel("m") || validPricingModel("") || validPricingModel(strings.Repeat("m", 201)) {
		t.Fatal("pricing model bounds unexpected")
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
		`{"user_id":"1","amount":1000000000.01,"note":"x"}`,
		`{"user_id":"1","amount":-1000000000.01,"note":"x"}`,
		`{"user_id":"1","amount":1,"note":"` + strings.Repeat("n", 501) + `"}`,
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

func TestRedactMigrationError(t *testing.T) {
	source := "user:s3cret@tcp(127.0.0.1:3306)/newapi"
	target := "postgres://router:t4rget@postgres:5432/router?sslmode=disable"
	msg := fmt.Sprintf("ping source database: dial error using %s; target was %s password=s3cret", source, target)
	got := redactMigrationError(msg, source, target)
	if strings.Contains(got, "s3cret") || strings.Contains(got, "t4rget") {
		t.Fatalf("secret still present: %q", got)
	}
	if !strings.Contains(got, "ping source database") {
		t.Fatalf("non-secret context lost: %q", got)
	}
	if redactMigrationError("") != "" {
		t.Fatal("empty error should stay empty")
	}
	plain := "migrate users: relation does not exist"
	if got := redactMigrationError(plain, source, target); got != plain {
		t.Fatalf("plain error changed: %q", got)
	}
}
func TestSetUserRoleRejectsInvalidRoleBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{`{}`, `{"role":"owner"}`, `{"role":""}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/users/1/role", strings.NewReader(body))
		(&Service{}).setUserRole(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestCreateModelRouteRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	long := strings.Repeat("m", 201)
	for _, body := range []string{
		`{}`,
		`{"public_model":"m","upstream_model":"u"}`,
		`{"public_model":" ","upstream_model":"u","channel_id":"c"}`,
		`{"public_model":"m","upstream_model":"u","channel_id":"c","weight":-1}`,
		`{"public_model":"m","upstream_model":"u","channel_id":"c","weight":10001}`,
		`{"public_model":"m","upstream_model":"u","channel_id":"c","priority":10001}`,
		`{"public_model":"` + long + `","upstream_model":"u","channel_id":"c"}`,
		`{"public_model":"m","upstream_model":"` + long + `","channel_id":"c"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/model-routes", strings.NewReader(body))
		(&Service{}).createModelRoute(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestValidModelName(t *testing.T) {
	if !validModelName("m") || !validModelName(strings.Repeat("m", 200)) {
		t.Fatal("boundary model names must be valid")
	}
	if validModelName("") || validModelName(strings.Repeat("m", 201)) {
		t.Fatal("out-of-range model names must be invalid")
	}
}

func TestUpsertQuotaRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"user_id":"1","window":"hour","max_requests":10}`,
		`{"window":"day","max_requests":10}`,
		`{"user_id":"1","window":"day"}`,
		`{"user_id":"1","window":"day","max_requests":-1}`,
		`{"api_key_id":"k","window":"month","max_tokens":-5}`,
		`{"user_id":"1","window":"day","max_requests":1000000000001}`,
		`{"user_id":"1","window":"day","max_tokens":1000000000001}`,
		`{"user_id":"1","window":"day","max_requests":1,"model":"` + strings.Repeat("m", 201) + `"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/quota-limits", strings.NewReader(body))
		(&Service{}).upsertQuota(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestValidWalletAndQuotaHelpers(t *testing.T) {
	if !validWalletAdjustAmount(1) || !validWalletAdjustAmount(-maxWalletAdjustAmount) || !validWalletAdjustAmount(maxWalletAdjustAmount) {
		t.Fatal("boundary wallet amounts must be valid")
	}
	if validWalletAdjustAmount(0) || validWalletAdjustAmount(maxWalletAdjustAmount+1) {
		t.Fatal("out-of-range wallet amounts must be invalid")
	}
	if !validUserBalance(0) || !validUserBalance(maxWalletAdjustAmount) || validUserBalance(-0.01) || validUserBalance(maxWalletAdjustAmount+1) {
		t.Fatal("user balance bounds unexpected")
	}
	var ok int64 = maxQuotaLimit
	var over int64 = maxQuotaLimit + 1
	var neg int64 = -1
	if !validQuotaLimit(nil) || !validQuotaLimit(&ok) || validQuotaLimit(&over) || validQuotaLimit(&neg) {
		t.Fatal("quota limit bounds unexpected")
	}
}

func TestCreateKeyRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"user_id":"1"}`,
		`{"name":"k"}`,
		`{"user_id":"1","name":"k","expires_at":"not-a-date"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/keys", strings.NewReader(body))
		(&Service{}).createKey(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestValidGroupNameAndProviderSlug(t *testing.T) {
	if !validGroupName("default") || !validGroupName(strings.Repeat("a", 100)) {
		t.Fatal("expected valid group names")
	}
	if validGroupName("") || validGroupName(strings.Repeat("a", 101)) {
		t.Fatal("expected invalid group names")
	}
	for _, slug := range []string{"openai", "open-ai", "claude3", "a1"} {
		if !validProviderSlug(slug) {
			t.Fatalf("expected valid slug %q", slug)
		}
	}
	for _, slug := range []string{"", "OpenAI", "-bad", "bad-", "has_under", "has space", strings.Repeat("a", 65)} {
		if validProviderSlug(slug) {
			t.Fatalf("expected invalid slug %q", slug)
		}
	}
}

func TestCreateGroupRejectsInvalidNameBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":" "}`,
		`{"name":"` + strings.Repeat("n", 101) + `"}`,
		`{"name":"ok","multiplier":-1}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/groups", strings.NewReader(body))
		(&Service{}).createGroup(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
	}
}


func TestSaveProviderRejectsInvalidBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"name":"OpenAI","slug":"open_ai","prefixes":["gpt-"]}`,
		`{"name":"OpenAI","slug":"-openai","prefixes":["gpt-"]}`,
		`{"name":"OpenAI","slug":"openai","prefixes":[]}`,
		`{"name":"OpenAI","slug":"openai","prefixes":["gpt-"],"priority":-1}`,
		`{"name":"OpenAI","slug":"openai","prefixes":["gpt-"],"priority":10001}`,
		`{"name":"","slug":"openai","prefixes":["gpt-"]}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(body))
		(&Service{}).saveProvider(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
	}
}

func TestValidChannelName(t *testing.T) {
	if !validChannelName("openai") || !validChannelName(strings.Repeat("c", 100)) {
		t.Fatal("expected valid channel names")
	}
	if validChannelName("") || validChannelName(strings.Repeat("c", 101)) {
		t.Fatal("expected invalid channel names")
	}
}

func TestCreateChannelRejectsOverlongNameBeforeDatabase(t *testing.T) {
	body := `{"name":"` + strings.Repeat("n", 101) + `","api_key":"sk","base_url":"https://api.example.com","models":["m"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
	(&Service{}).createChannel(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateChannelRejectsOverlongNameBeforeDatabase(t *testing.T) {
	body := `{"name":"` + strings.Repeat("n", 101) + `","base_url":"https://api.example.com","models":["m"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/channels/channel-id", strings.NewReader(body))
	(&Service{}).updateChannel(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestValidAPIKeyName(t *testing.T) {
	if !validAPIKeyName("default") || !validAPIKeyName(strings.Repeat("k", 100)) {
		t.Fatal("expected valid API key names")
	}
	if validAPIKeyName("") || validAPIKeyName(strings.Repeat("k", 101)) {
		t.Fatal("expected invalid API key names")
	}
}

func TestCreateKeyRejectsInvalidNameBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{"user_id":"1","name":" "}`,
		`{"user_id":"1","name":"` + strings.Repeat("n", 101) + `"}`,
		`{"user_id":"","name":"ok"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/keys", strings.NewReader(body))
		(&Service{}).createKey(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
	}
}


func TestValidChannelAPIKeyAndBaseURL(t *testing.T) {
	if !validChannelAPIKey("sk") || !validChannelAPIKey(strings.Repeat("k", maxChannelAPIKeyLen)) {
		t.Fatal("boundary api keys must be valid")
	}
	if validChannelAPIKey("") || validChannelAPIKey(strings.Repeat("k", maxChannelAPIKeyLen+1)) {
		t.Fatal("out-of-range api keys must be invalid")
	}
	if !validChannelBaseURL("https://api.example.com") {
		t.Fatal("public https base_url must be valid")
	}
	if validChannelBaseURL("") || validChannelBaseURL("http://api.example.com") || validChannelBaseURL("https://"+strings.Repeat("a", 2040)+".example.com") {
		t.Fatal("invalid base_url must be rejected")
	}
}

func TestSanitizeChannelModelsCapsCount(t *testing.T) {
	models := make([]string, maxChannelModels+1)
	for i := range models {
		models[i] = "model-" + strconv.Itoa(i)
	}
	if _, ok := sanitizeChannelModels(models); ok {
		t.Fatal("expected more than maxChannelModels to be rejected")
	}
	okModels := models[:maxChannelModels]
	if out, ok := sanitizeChannelModels(okModels); !ok || len(out) != maxChannelModels {
		t.Fatalf("expected %d models accepted, got ok=%v len=%d", maxChannelModels, ok, len(out))
	}
}

func TestImportGroupsRejectsTooManyBeforeDatabase(t *testing.T) {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i <= maxGroupImportCount; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"g`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`":1`)
	}
	b.WriteByte('}')
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/groups/import", strings.NewReader(b.String()))
	(&Service{}).importGroups(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
