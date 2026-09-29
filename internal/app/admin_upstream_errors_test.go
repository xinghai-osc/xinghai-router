package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminErrorDetailPreservesReasonAndRedactsCredentials(t *testing.T) {
	const secret = "fake-provider-credential"
	body := `{"error":{"message":"credit balance is too low", "api_key":"` + secret + `", "authorization":"Bearer another-credential", "detail":"https://upstream.example/private sk-other-secret"}}`
	got := adminErrorDetail(body, secret)
	if !strings.Contains(got, "credit balance is too low") {
		t.Fatalf("lost actionable reason: %s", got)
	}
	for _, forbidden := range []string{secret, "another-credential", "sk-other-secret", "upstream.example"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("secret or upstream URL leaked: %s", got)
		}
	}
	if len([]rune(adminErrorDetail(strings.Repeat("错", 2000)))) > 500 {
		t.Fatal("admin error is not bounded")
	}
}

func TestAdminUpstreamFailure(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
	}{
		{"openai", `{"error":{"message":"no usable channel supports this model"}}`, "no usable channel supports this model"},
		{"string error", `{"error":"invalid model"}`, "invalid model"},
		{"message", `{"message":"quota exhausted"}`, "quota exhausted"},
		{"detail", `{"detail":[{"msg":"model not found"}]}`, "model not found"},
		{"plain", "service unavailable", "service unavailable"},
		{"empty", "", "upstream_status_503"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := adminUpstreamFailure("", 503, []byte(tt.body), nil, "fake-key")
			if !strings.Contains(got, "upstream_status_503") || !strings.Contains(got, tt.want) {
				t.Fatalf("failure = %s", got)
			}
		})
	}
	got := adminUpstreamFailure("health_check_keyword_match", 401, []byte(`{"error":{"message":"quota exceeded for fake-key"}}`), nil, "fake-key")
	if !strings.Contains(got, "health_check_keyword_match: quota exceeded") || strings.Contains(got, "fake-key") {
		t.Fatalf("test failure = %s", got)
	}
	got = adminUpstreamFailure("", 0, nil, errors.New("dial https://upstream.example: connection refused"), "")
	if !strings.Contains(got, "connection refused") || strings.Contains(got, "upstream.example") {
		t.Fatalf("network failure = %s", got)
	}
}

func TestFetchChannelModelsReportsUpstreamErrorsWithoutAuthStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"unauthorized", 401, `{"error":{"message":"invalid API key fake-models-key"}}`, "invalid API key"},
		{"forbidden", 403, `{"message":"quota exhausted"}`, "quota exhausted"},
		{"plain", 503, "no usable channel supports this model", "no usable channel supports this model"},
		{"detail", 422, `{"detail":[{"msg":"unsupported model"}]}`, "unsupported model"},
		{"empty", 500, "", "upstream_status_500"},
		{"error with success status", 200, `{"error":{"message":"quota exhausted"}}`, "quota exhausted"},
		{"missing list", 200, `{}`, "invalid models response"},
		{"null list", 200, `{"data":null}`, "invalid models response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(`{"base_url":"`+upstream.URL+`","api_key":"fake-models-key"}`))
			(&Service{httpClient: upstream.Client()}).fetchChannelModels(w, r)
			var result struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusBadGateway || result.Error.Code != "upstream_error" || !strings.Contains(result.Error.Message, tt.want) || strings.Contains(result.Error.Message, "fake-models-key") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestFetchChannelModelsProviderRequest(t *testing.T) {
	for _, provider := range []string{"anthropic", "commandcode", "opencode_go"} {
		t.Run(provider, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := "/prefix/v1/models"
				if provider == "commandcode" {
					path = "/prefix" + commandCodeModelsPath
				}
				if r.URL.Path != path {
					t.Errorf("path = %s, want %s", r.URL.Path, path)
				}
				if provider == "anthropic" {
					if r.Header.Get("X-API-Key") != "fake-models-key" || r.Header.Get("Anthropic-Version") == "" {
						t.Error("missing Anthropic authentication")
					}
				} else if r.Header.Get("Authorization") != "Bearer fake-models-key" {
					t.Error("missing Bearer authentication")
				}
				if provider == "opencode_go" && r.Header.Get("x-opencode-session") == "" {
					t.Error("missing OpenCode session")
				}
				_, _ = w.Write([]byte(`{"data":[{"id":"model"}]}`))
			}))
			defer upstream.Close()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(`{"base_url":"`+upstream.URL+`/prefix/","api_key":"fake-models-key","provider":"`+provider+`"}`))
			(&Service{httpClient: upstream.Client()}).fetchChannelModels(w, r)
			if w.Code != 200 {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
