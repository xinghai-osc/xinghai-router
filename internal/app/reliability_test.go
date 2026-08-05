package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateReliabilitySettingsRejectsInvalidValues(t *testing.T) {
	for _, body := range []string{`{"retry_count":-1}`, `{"retry_count":11}`, `{"health_check_mode":"always"}`, `{"health_check_interval_minutes":0}`, `{"health_check_interval_minutes":1441}`, `{"auto_disable_slow_seconds":-1}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/reliability-settings", strings.NewReader(body))
		(&Service{}).updateReliabilitySettings(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want %d", body, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestParseStatusMatcher(t *testing.T) {
	m := parseStatusMatcher("100-199,300-407,409-503,505-523,525-599")
	cases := []struct {
		code int
		want bool
	}{
		{100, true}, {150, true}, {199, true}, {200, false}, {299, false},
		{300, true}, {407, true}, {408, false}, {409, true}, {429, true},
		{503, true}, {504, false}, {505, true}, {523, true}, {524, false},
		{525, true}, {599, true}, {600, false}, {99, false},
	}
	for _, c := range cases {
		if got := m.match(c.code); got != c.want {
			t.Errorf("match(%d) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestParseStatusMatcherSingleCodes(t *testing.T) {
	m := parseStatusMatcher("401, 429 ,503")
	for _, code := range []int{401, 429, 503} {
		if !m.match(code) {
			t.Errorf("match(%d) = false, want true", code)
		}
	}
	for _, code := range []int{400, 402, 500, 502} {
		if m.match(code) {
			t.Errorf("match(%d) = true, want false", code)
		}
	}
}

func TestParseStatusMatcherInvalid(t *testing.T) {
	m := parseStatusMatcher("abc,99-50,700,,1000")
	if m.match(500) || m.match(100) {
		t.Error("invalid spec should not match anything")
	}
}

func TestValidStatusCodeSpec(t *testing.T) {
	valid := []string{"401,429,503", "100-199,300-407", "200", "500-599"}
	for _, spec := range valid {
		if !validStatusCodeSpec(spec) {
			t.Errorf("validStatusCodeSpec(%q) = false, want true", spec)
		}
	}
	invalid := []string{"", "abc", "99", "600", "500-499", "100-700", "1-2-3", "401,"}
	for _, spec := range invalid {
		if validStatusCodeSpec(spec) {
			t.Errorf("validStatusCodeSpec(%q) = true, want false", spec)
		}
	}
}

func TestAutoDisableKeyword(t *testing.T) {
	s := defaultReliabilitySettings()
	if !s.autoDisableKeyword(`{"error":{"message":"Your credit balance is too low, please recharge"}}`) {
		t.Error("expected credit balance keyword to match")
	}
	if !s.autoDisableKeyword(`{"error":"TOO MANY REQUESTS"}`) {
		t.Error("expected case-insensitive keyword match")
	}
	if !s.autoDisableKeyword("错误：订阅额度不足或未配置订阅") {
		t.Error("expected Chinese keyword to match")
	}
	if s.autoDisableKeyword(`{"choices":[{"message":{"content":"hello"}}]}`) {
		t.Error("normal response should not match keywords")
	}
}

func TestMatchedKeyword(t *testing.T) {
	s := defaultReliabilitySettings()
	if got := s.matchedKeyword(`{"error":{"message":"Your credit balance is too low, please recharge"}}`); got != "your credit balance is too low" {
		t.Errorf("matchedKeyword = %q, want %q", got, "your credit balance is too low")
	}
	if got := s.matchedKeyword(`{"error":"TOO MANY REQUESTS"}`); got != "too many requests" {
		t.Errorf("matchedKeyword = %q, want %q", got, "too many requests")
	}
	if got := s.matchedKeyword(`{"choices":[{"message":{"content":"hello"}}]}`); got != "" {
		t.Errorf("matchedKeyword = %q, want empty", got)
	}
}

func TestSplitKeywordsTrimsAndSkipsEmpty(t *testing.T) {
	keywords := splitKeywords("  Foo Bar  \n\n Baz \n")
	if len(keywords) != 2 || keywords[0] != "foo bar" || keywords[1] != "baz" {
		t.Errorf("unexpected keywords: %v", keywords)
	}
}

func TestParseIDList(t *testing.T) {
	ids := parseIDList("69,75 27\n80,, 88")
	for _, id := range []string{"69", "75", "27", "80", "88"} {
		if !ids[id] {
			t.Errorf("expected %q in id list", id)
		}
	}
	if len(ids) != 5 {
		t.Errorf("expected 5 ids, got %d", len(ids))
	}
	if parseIDList("") != nil && len(parseIDList("")) != 0 {
		t.Error("empty input should produce empty set")
	}
}

func TestDefaultReliabilitySettings(t *testing.T) {
	s := defaultReliabilitySettings()
	if s.RetryCount != 3 {
		t.Errorf("default retry count = %d, want 3", s.RetryCount)
	}
	if !s.retryable(429) || !s.retryable(503) || s.retryable(200) || s.retryable(408) {
		t.Error("default retry codes mismatch")
	}
	if !s.autoDisableStatus(401) || !s.autoDisableStatus(429) || !s.autoDisableStatus(503) || s.autoDisableStatus(500) {
		t.Error("default auto-disable codes mismatch")
	}
	if s.HealthCheckMode != "off" {
		t.Errorf("default health check mode = %q, want off", s.HealthCheckMode)
	}
	if len(s.parsedKeywords) != 16 {
		t.Errorf("expected 16 default keywords, got %d", len(s.parsedKeywords))
	}
}

func TestRetryableStatusDefaultSpec(t *testing.T) {
	// The default spec covers every status except 2xx, 408, and 504.
	if !retryableStatus(429) || !retryableStatus(500) || !retryableStatus(301) {
		t.Error("expected retryable statuses to match default spec")
	}
	if retryableStatus(200) || retryableStatus(204) || retryableStatus(408) || retryableStatus(504) {
		t.Error("2xx, 408 and 504 must not be retryable by default")
	}
}
