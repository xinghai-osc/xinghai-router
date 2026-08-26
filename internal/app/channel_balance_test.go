package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryUpstreamBalanceUsesCreditGrants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dashboard/billing/credit_grants" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_granted":100,"total_used":35,"total_available":65,"currency":"USD"}`))
	}))
	defer server.Close()

	s := &Service{httpClient: server.Client()}
	got, err := s.queryUpstreamBalance(context.Background(), server.URL, "test-key", "openai")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance == nil || *got.Balance != 65 || got.Used == nil || *got.Used != 35 || got.Total == nil || *got.Total != 100 || got.Currency != "USD" {
		t.Fatalf("unexpected balance: %+v", got)
	}
}

func TestQueryUpstreamBalanceFallsBackToSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dashboard/billing/subscription" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"hard_limit_usd":20,"total_usage":7}`))
	}))
	defer server.Close()

	s := &Service{httpClient: server.Client()}
	got, err := s.queryUpstreamBalance(context.Background(), server.URL, "test-key", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance == nil || *got.Balance != 13 || got.Total == nil || *got.Total != 20 {
		t.Fatalf("unexpected fallback balance: %+v", got)
	}
}

func TestQueryUpstreamBalanceSupportsNewAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota":1000,"used_quota":275}}`))
	}))
	defer server.Close()

	s := &Service{httpClient: server.Client()}
	got, err := s.queryUpstreamBalance(context.Background(), server.URL, "test-key", "custom")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance == nil || *got.Balance != 725 || got.Total == nil || *got.Total != 1000 || got.Currency != "quota" {
		t.Fatalf("unexpected New API balance: %+v", got)
	}
}

func TestQueryUpstreamBalanceSupportsDeepSeek(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/balance" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"110.00","granted_balance":"110.00","topped_up_balance":"0.00"}]}}`))
	}))
	defer server.Close()

	s := &Service{httpClient: server.Client()}
	got, err := s.queryUpstreamBalance(context.Background(), server.URL, "test-key", "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance == nil || *got.Balance != 110 || got.Currency != "CNY" {
		t.Fatalf("unexpected DeepSeek balance: %+v", got)
	}
}

func TestQueryUpstreamBalanceSupportsOpenCodeGoUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/go/v1/usage" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":18,"resetsAt":"2026-08-13T18:26:39.281Z"},"weekly":{"status":"ok","percent":32,"resetsAt":"2026-08-17T00:00:00.281Z"},"monthly":{"status":"ok","percent":5,"resetsAt":"2026-09-01T00:00:00.000Z"}}}`))
	}))
	defer server.Close()

	s := &Service{httpClient: server.Client()}
	got, err := s.queryUpstreamBalance(context.Background(), server.URL+"/zen/go/v1", "test-key", "opencode_go")
	if err != nil {
		t.Fatal(err)
	}
	if !got.UsageSupported || len(got.UsageWindows) != 3 {
		t.Fatalf("unexpected OpenCode Go usage: %+v", got)
	}
	if got.UsageWindows[0].Window != "rolling" || got.UsageWindows[0].Percent == nil || *got.UsageWindows[0].Percent != 18 {
		t.Fatalf("unexpected rolling window: %+v", got.UsageWindows[0])
	}
	if got.UsageWindows[0].ResetAt != "2026-08-13T18:26:39Z" {
		t.Fatalf("reset timestamp = %q", got.UsageWindows[0].ResetAt)
	}
}

func TestParseOpenCodeGoUsageRejectsMalformedOrEmptyWindows(t *testing.T) {
	if _, err := parseOpenCodeGoUsage([]byte(`{"usage":{"rolling":{"percent":120}}}`)); err == nil {
		t.Fatal("expected out-of-range usage window to be rejected")
	}
	if _, err := parseOpenCodeGoUsage([]byte(`{"usage":{"rolling":{"status":"ok"}}}`)); err != nil {
		t.Fatalf("missing optional fields should be accepted: %v", err)
	}
	if _, err := parseOpenCodeGoUsage([]byte(`{"usage":{"rolling":"bad"}}`)); err == nil {
		t.Fatal("expected malformed window to be rejected")
	}
}

func TestQueryUpstreamBalanceUnsupportedProvider(t *testing.T) {
	s := &Service{httpClient: &http.Client{Timeout: time.Second}}
	if _, err := s.queryUpstreamBalance(context.Background(), "https://example.com", "test-key", "anthropic"); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
