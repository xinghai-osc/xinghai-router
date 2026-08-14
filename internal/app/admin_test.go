package app

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSyncNewAPIPricingRejectsInvalidSourceBeforeNetworkOrDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"base_url":"https://example.com","price_per_quota_unit":-1}`,
		`{"base_url":"https://example.com","price_per_quota_unit":"nan"}`,
		`{"base_url":"https://example.com","price_per_quota_unit":"inf"}`,
		`{"base_url":"https://example.com","price_per_quota_unit":1000000.01}`,
		`{"base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","price_per_quota_unit":1}`,
		`{"base_url":"https://example.com","api_key":"` + strings.Repeat("k", 4097) + `","price_per_quota_unit":1}`,
		`{"base_url":"","price_per_quota_unit":1}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/pricing/newapi/sync", strings.NewReader(body))
		(&Service{}).syncNewAPIPricing(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestFetchChannelModelsRejectsInvalidRequestBeforeNetworkAccess(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"base_url":"","api_key":"sk"}`,
		`{"base_url":"https://api.example.com","api_key":""}`,
		`{"base_url":"https://api.example.com","api_key":"` + strings.Repeat("k", 4097) + `"}`,
		`{"base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","api_key":"sk"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(body))
		(&Service{}).fetchChannelModels(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestValidPricePerQuotaUnit(t *testing.T) {
	if !validPricePerQuotaUnit(0) || !validPricePerQuotaUnit(maxPricePerQuotaUnit) {
		t.Fatal("boundary price_per_quota_unit must be valid")
	}
	if validPricePerQuotaUnit(-0.01) || validPricePerQuotaUnit(maxPricePerQuotaUnit+0.01) {
		t.Fatal("out-of-range price_per_quota_unit must be invalid")
	}
	if validPricePerQuotaUnit(math.NaN()) || validPricePerQuotaUnit(math.Inf(1)) || validPricePerQuotaUnit(math.Inf(-1)) {
		t.Fatal("non-finite price_per_quota_unit must be invalid")
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
		{name: "non-numeric id", body: `{"id":"user-id"}`},
		{name: "zero id", body: `{"id":0}`},
		{name: "negative id", body: `{"id":-5}`},
		{name: "oversized id", body: `{"id":9007199254740992}`},
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
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":[]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"provider":"unknown"}`,
		`{"name":"","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"priority":10001}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"priority":-10001}`,
		`{"name":"channel","key_type":"single","api_keys":"","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"single","api_keys":"` + strings.Repeat("k", 4097) + `","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"multi","api_keys":"sk1\nsk2","base_url":"https://api.example.com","models":["model"],"provider":"unknown"}`,
		`{"name":"channel","key_type":"single","api_keys":"sk1\nsk2","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"unknown","api_keys":"sk","base_url":"https://api.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://` + strings.Repeat("a", 2040) + `.example.com","models":["model"]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"request_overrides":{"delete":[""]}}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"request_overrides":{"delete":["a","a"]}}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"request_overrides":{"set":{"":"x"}}}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"request_overrides":{"delete":["` + strings.Repeat("a", 101) + `"]}}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"ua_pool":[""]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"ua_pool":["   "]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"ua_pool":["` + strings.Repeat("a", 513) + `"]}`,
		`{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["model"],"ua_pool":` + tooManyUAPoolJSON() + `}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
		(&Service{}).createChannel(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func tooManyUAPoolJSON() string {
	entries := make([]string, maxUAPoolEntries+1)
	for i := range entries {
		entries[i] = `"ua` + strconv.Itoa(i) + `"`
	}
	return "[" + strings.Join(entries, ",") + "]"
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
	for _, p := range []string{"openai", "ollama", "kimi", "opencode_go", "anthropic", "deepseek", "custom"} {
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
	if !validChannelKeyType("single") || !validChannelKeyType("multi") {
		t.Fatal("expected valid channel key types")
	}
	if validChannelKeyType("unknown") || validChannelKeyType("") {
		t.Fatal("unknown key type must be invalid")
	}
	keys := parseChannelAPIKeys("sk1\nsk2\n\nsk1")
	if len(keys) != 2 || keys[0] != "sk1" || keys[1] != "sk2" {
		t.Fatalf("parseChannelAPIKeys = %#v", keys)
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
	body := `{"name":"channel","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":[" ","\t"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
	(&Service{}).createChannel(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestNewAPIPricePerMillionRejectsNonFiniteResults(t *testing.T) {
	input := newAPIPricePerMillion(math.Inf(1), 1, 500000)
	if validPricingRate(input) {
		t.Fatal("infinite converted price must fail validPricingRate")
	}
	input = newAPIPricePerMillion(math.NaN(), 1, 500000)
	if validPricingRate(input) {
		t.Fatal("NaN converted price must fail validPricingRate")
	}
	input = newAPIPricePerMillion(maxPricingRate, 2, 1)
	if validPricingRate(input) {
		t.Fatal("oversized converted price must fail validPricingRate")
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

func TestBatchUpdateGroupsRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	for _, body := range []string{
		`{"groups":[]}`,
		`{"groups":[{"id":"g1","multiplier":-1}]}`,
		`{"groups":[{"id":"g1","multiplier":"nan"}]}`,
		`{"groups":[{"id":"g1","multiplier":1001}]}`,
		`{"groups":[{"id":"","multiplier":1}]}`,
		`{"groups":[{"id":"g1","multiplier":1,"max_concurrency":-1}]}`,
		`{"groups":[{"id":"g1","multiplier":1,"display_name":"` + strings.Repeat("a", 101) + `"}]}`,
		`{"groups":[{"id":"g1","multiplier":1,"description":"` + strings.Repeat("a", 501) + `"}]}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/admin/groups/batch-update", strings.NewReader(body))
		(&Service{}).batchUpdateGroups(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("batchUpdateGroups body %s status = %d", body, recorder.Code)
		}
	}
	var groups []map[string]any
	for i := 0; i < 101; i++ {
		groups = append(groups, map[string]any{"id": "g", "multiplier": 1})
	}
	body, _ := json.Marshal(map[string]any{"groups": groups})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/groups/batch-update", strings.NewReader(string(body)))
	(&Service{}).batchUpdateGroups(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("batchUpdateGroups with 101 groups status = %d", recorder.Code)
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

func TestValidTimeWindow(t *testing.T) {
	if !validTimeWindow(0, 1440) || !validTimeWindow(360, 720) || !validTimeWindow(1320, 360) {
		t.Fatal("valid time windows rejected")
	}
	if validTimeWindow(0, 0) || validTimeWindow(-1, 100) || validTimeWindow(0, 1441) || validTimeWindow(1440, 100) {
		t.Fatal("invalid time windows accepted")
	}
}

func TestValidWeekdays(t *testing.T) {
	if !validWeekdays("1111111") || !validWeekdays("0000010") || !validWeekdays("1010100") {
		t.Fatal("valid weekday strings rejected")
	}
	if validWeekdays("") || validWeekdays("111111") || validWeekdays("11111111") || validWeekdays("2111111") {
		t.Fatal("invalid weekday strings accepted")
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

func TestValidateSourceDSN(t *testing.T) {
	allowed := []string{
		"user:pass@tcp(localhost:3306)/db",
		"user:pass@tcp(127.0.0.1:3306)/db",
		"user:pass@127.0.0.1:3306/db",
		"user:pass@tcp([::1]:3306)/db",
		"mysql://user:pass@db.example.com:3306/db",
		"postgres://user:pass@104.26.7.174:5432/db",
		"postgres://user:pass@db.example.com:5432/db",
		"user:pass@unix(/var/run/mysqld/mysqld.sock)/db",
		"user:pass@/db",
		"/tmp/migrate.db",
		"host=db.example.com port=5432 dbname=mydb user=app",
		"host=/var/run/postgresql dbname=mydb",
	}
	for _, dsn := range allowed {
		if err := validateSourceDSN(dsn); err != nil {
			t.Fatalf("validateSourceDSN(%q) = %v, want nil", dsn, err)
		}
	}
	blocked := []string{
		"user:pass@tcp(10.0.0.5:3306)/db",
		"user:pass@172.16.1.10:3306/db",
		"user:pass@192.168.1.5:3306/db",
		"user:pass@tcp(169.254.169.254:3306)/db",
		"mysql://user:pass@10.1.2.3:3306/db",
		"postgres://user:pass@192.168.0.4:5432/db",
		"user:pass@100.64.0.10:3306/db",
		"user:pass@198.18.0.1:3306/db",
		"user:pass@tcp(fc00::1:3306)/db",
	}
	for _, dsn := range blocked {
		if err := validateSourceDSN(dsn); err == nil {
			t.Fatalf("validateSourceDSN(%q) = nil, want error", dsn)
		}
	}
}

func TestRedactMigrationError(t *testing.T) {
	cases := []struct{ in, out string }{
		{"", ""},
		{"ping source database: dial tcp 10.0.0.5:3306: connect: connection refused", "ping source database: dial tcp 10.0.0.5:3306: connect: connection refused"},
		{`invalid DSN "user:pass@tcp(127.0.0.1:3306)/db" ...`, `invalid DSN "user:***@tcp(127.0.0.1:3306)/db" ...`},
	}
	for _, c := range cases {
		if got := redactMigrationError(c.in, ""); got != c.out {
			t.Fatalf("redactMigrationError(%q) = %q, want %q", c.in, got, c.out)
		}
	}
	if got := redactMigrationError("failed to open user:sek:ret@mysql:3306/db", "user:sek:ret@mysql:3306/db"); strings.Contains(got, "sek:ret") {
		t.Fatalf("exact DSN not stripped: %q", got)
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
	body := `{"name":"` + strings.Repeat("n", 101) + `","key_type":"single","api_keys":"sk","base_url":"https://api.example.com","models":["m"]}`
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
	if !validChannelBaseURL("https://api.example.com") || !validChannelBaseURL("http://api.example.com") || !validChannelBaseURL("http://10.0.0.5:8080") {
		t.Fatal("http and https base_url must be valid")
	}
	if validChannelBaseURL("") || validChannelBaseURL("ftp://api.example.com") || validChannelBaseURL("https://"+strings.Repeat("a", 2040)+".example.com") {
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
