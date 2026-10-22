package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type miniMaxBalanceTransport func(*http.Request) (*http.Response, error)

func (f miniMaxBalanceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestMiniMaxBalanceCredentialValidation(t *testing.T) {
	for _, value := range []string{"1", "2084086050715607726", strings.Repeat("9", 64)} {
		if !validMiniMaxBalanceGroupID(value) {
			t.Errorf("valid group rejected: %q", value)
		}
	}
	for _, value := range []string{"", " 1", "1\r\n", "１２", "-1", "1.0", strings.Repeat("1", 65)} {
		if validMiniMaxBalanceGroupID(value) {
			t.Errorf("invalid group accepted: %q", value)
		}
	}
	for _, value := range []string{"_token=fake-token", "other=value; _token=fake-token; minimax_group_id_v2=123;", "_token=fake-token; encoded=%7B%7D"} {
		if !validMiniMaxBalanceCookie(value) {
			t.Errorf("valid cookie rejected: %q", value)
		}
	}
	for _, value := range []string{"", "token=fake", "_token=", "_token=fake\r\nX-Injected: value", "_token=fake; garbage", "_token=first; _token=second", "_token=fake\x00", "_token=" + strings.Repeat("x", 16384)} {
		if validMiniMaxBalanceCookie(value) {
			t.Error("invalid cookie accepted")
		}
	}
}

func TestParseMiniMaxBalanceCreditsAndSensitiveFields(t *testing.T) {
	for _, tc := range []struct {
		body                   string
		total, used, remaining float64
	}{
		{`{"total_credits":0,"used_credits":0,"remaining_credits":0,"api_key":"fake-secret-api-key","balance_breakdown":{"total_balance":0,"buckets":[]},"token_plan_credit_fallback_enabled":true,"base_resp":{"status_code":0,"status_msg":"success"}}`, 0, 0, 0},
		{`{"total_credits":120.5,"used_credits":20.25,"remaining_credits":100.25,"api_key":"fake-secret-api-key","base_resp":{"status_code":0}}`, 120.5, 20.25, 100.25},
	} {
		got, err := parseMiniMaxBalance([]byte(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		if got.Total == nil || got.Used == nil || got.Balance == nil || *got.Total != tc.total || *got.Used != tc.used || *got.Balance != tc.remaining || got.Currency != "credits" {
			t.Fatalf("unexpected parsed balance: %+v", got)
		}
		encoded, err := json.Marshal(got)
		if err != nil || strings.Contains(string(encoded), "fake-secret") || strings.Contains(string(encoded), "api_key") {
			t.Fatal("balance contains upstream secret fields")
		}
	}
}

func TestParseMiniMaxBalanceRejectsBusinessErrorsAndInvalidCredits(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `[]`, `not json`,
		`{"total_credits":0,"used_credits":0,"remaining_credits":0}`,
		`{"total_credits":0,"used_credits":0,"remaining_credits":0,"base_resp":{}}`,
		`{"total_credits":0,"used_credits":0,"remaining_credits":0,"base_resp":{"status_code":1004,"status_msg":"fake-secret-token"},"api_key":"fake-secret-key"}`,
		`{"used_credits":0,"remaining_credits":0,"base_resp":{"status_code":0}}`,
		`{"total_credits":null,"used_credits":0,"remaining_credits":0,"base_resp":{"status_code":0}}`,
		`{"total_credits":1,"used_credits":0,"remaining_credits":-1,"base_resp":{"status_code":0}}`,
		`{"total_credits":1e999,"used_credits":0,"remaining_credits":0,"base_resp":{"status_code":0}}`,
		`{"total_credits":"fake-secret","used_credits":0,"remaining_credits":0,"base_resp":{"status_code":0}}`,
	} {
		_, err := parseMiniMaxBalance([]byte(body))
		if err == nil {
			t.Fatalf("invalid response accepted: %s", body)
		}
		if strings.Contains(err.Error(), "fake-secret") {
			t.Fatal("upstream response leaked into error")
		}
	}
}

func TestQueryMiniMaxBalanceRequest(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: miniMaxBalanceTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodGet || r.URL.String() != "https://www.minimax.cn/backend/account/token_plan_credit" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Cookie") != "_token=fake-cookie" || r.Header.Get("X-Group-Id") != "1234567890123456789" || r.Header.Get("Authorization") != "" || r.Header.Get("Origin") != "https://platform.minimax.cn" || r.Header.Get("Referer") != "https://platform.minimax.cn/" || r.Header.Get("Accept") != "application/json" {
			t.Error("incorrect balance authentication headers")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"total_credits":10,"used_credits":3,"remaining_credits":7,"base_resp":{"status_code":0}}`)), Request: r}, nil
	})}
	s := &Service{httpClient: client}
	got, err := s.queryMiniMaxBalance(context.Background(), "_token=fake-cookie", "1234567890123456789")
	if err != nil || got.Balance == nil || *got.Balance != 7 || calls != 1 {
		t.Fatalf("balance=%+v err=%v calls=%d", got, err, calls)
	}
	if client.Timeout != 0 || client.CheckRedirect != nil {
		t.Fatal("shared HTTP client was modified")
	}
	_, err = s.queryMiniMaxBalance(context.Background(), "_token=fake\n", "1")
	if err == nil || calls != 1 {
		t.Fatal("invalid credentials reached upstream")
	}
	_, err = s.queryUpstreamBalance(context.Background(), "https://untrusted.example", "fake-api-key", "minimax")
	if err == nil || calls != 1 {
		t.Fatal("MiniMax reached generic balance probes")
	}
}

func TestQueryMiniMaxBalanceRejectsRedirectsAndTransportErrors(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		body           string
		transportError bool
	}{
		{"redirect", http.StatusFound, "fake-secret-body", false},
		{"unauthorized", http.StatusUnauthorized, "fake-secret-body", false},
		{"business error", http.StatusOK, `{"base_resp":{"status_code":1004,"status_msg":"fake-secret-cookie"},"api_key":"fake-secret-key"}`, false},
		{"oversized response", http.StatusOK, strings.Repeat("x", (2<<20)+1), false},
		{"transport error", 0, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := &Service{httpClient: &http.Client{Transport: miniMaxBalanceTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if tc.transportError {
					return nil, fmt.Errorf("fake-secret-cookie")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": []string{"https://attacker.example/?token=fake-secret-cookie"}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})}}
			_, err := s.queryMiniMaxBalance(context.Background(), "_token=fake-cookie", "123")
			if err == nil || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
			if strings.Contains(err.Error(), "fake-secret") || strings.Contains(err.Error(), "attacker") {
				t.Fatal("sensitive upstream data leaked into error")
			}
		})
	}
}

func TestUpdateMiniMaxBalanceCredentialsValidatesBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{`, `null`, `{}`, `{"cookie":"_token=fake","group_id":123}`,
		`{"cookie":"_token=fake\r\nInjected: true","group_id":"123"}`,
		`{"cookie":"unrelated=fake","group_id":"123"}`,
		`{"cookie":"_token=fake","group_id":"1.2"}`,
	} {
		w := httptest.NewRecorder()
		(&Service{}).updateMiniMaxBalanceCredentials(w, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
