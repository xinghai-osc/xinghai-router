package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestFetchChannelModelsUpstreamFormats(t *testing.T) {
	const nativeResponse = `{"models":[{"name":" jev-latest ","description":"Latest model","release_date":"2026-09-15"},{"name":"jev-1","description":"Versioned model","release_date":"2026-09-15"},{"name":"jev-latest"},{"name":" "},{}],"data":[{"id":"not-native"}]}`
	const openAIResponse = `{"data":[{"id":" gpt-test "},{"id":"gpt-other"},{"id":"gpt-test"},{"id":" "},{}],"models":[{"name":"not-openai"}]}`
	tests := []struct {
		name           string
		provider       string
		upstreamFormat string
		omitFormat     bool
		response       string
		want           []string
	}{
		{name: "native jev", upstreamFormat: "jev", response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "jev provider auto", provider: "jev", omitFormat: true, response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "jev provider empty format", provider: "jev", response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "jev provider trimmed", provider: " jev ", response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "legacy custom jev", provider: "custom", upstreamFormat: "jev", response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "explicit format overrides provider", provider: "jev", upstreamFormat: "openai_chat", response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "trimmed jev format", upstreamFormat: " jev ", response: nativeResponse, want: []string{"jev-latest", "jev-1"}},
		{name: "legacy omitted format", omitFormat: true, response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "default empty format", response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "explicit openai", upstreamFormat: "openai", response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "openai chat", upstreamFormat: "openai_chat", response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "anthropic", upstreamFormat: "anthropic", response: openAIResponse, want: []string{"gpt-test", "gpt-other"}},
		{name: "empty native list", upstreamFormat: "jev", response: `{"models":[]}`, want: []string{}},
		{name: "empty openai list", upstreamFormat: "openai", response: `{"data":[]}`, want: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
					t.Errorf("upstream request = %s %s, want GET /v1/models", r.Method, r.URL.Path)
				}
				if resolveUpstreamFormat(strings.TrimSpace(test.provider), test.upstreamFormat) == "anthropic" {
					if r.Header.Get("X-API-Key") != "fake-models-key" || r.Header.Get("Anthropic-Version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
						t.Error("missing Anthropic authentication headers")
					}
				} else if got := r.Header.Get("Authorization"); got != "Bearer fake-models-key" {
					t.Errorf("Authorization = %q, want fake bearer key", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.response))
			}))
			defer upstream.Close()

			input := map[string]string{"base_url": upstream.URL + "/", "api_key": " fake-models-key "}
			if !test.omitFormat {
				input["upstream_format"] = test.upstreamFormat
			}
			if test.provider != "" {
				input["provider"] = test.provider
			}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/channels/models", bytes.NewReader(body))
			(&Service{httpClient: upstream.Client()}).fetchChannelModels(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var result struct {
				Models []string `json:"models"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Models, test.want) {
				t.Fatalf("models = %#v, want %#v", result.Models, test.want)
			}
		})
	}
}

func TestFetchChannelModelsRejectsInvalidUpstreamFormat(t *testing.T) {
	for _, format := range []string{"unknown", "typesafe", "JEV"} {
		t.Run(format, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(`{"base_url":"https://api.example.com","api_key":"fake-models-key","upstream_format":"`+format+`"}`))
			(&Service{}).fetchChannelModels(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			var result struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Error.Code != "invalid_request" {
				t.Fatalf("error code = %q, want invalid_request", result.Error.Code)
			}
		})
	}
}

func TestFetchChannelModelsRejectsInvalidProvider(t *testing.T) {
	for _, provider := range []string{"unknown", "JEV"} {
		t.Run(provider, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(`{"base_url":"https://api.example.com","api_key":"fake-models-key","provider":"`+provider+`"}`))
			(&Service{}).fetchChannelModels(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "unsupported provider") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestFetchChannelModelsRejectsMalformedUpstreamResponse(t *testing.T) {
	for _, format := range []string{"jev", "openai"} {
		t.Run(format, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"models":`))
			}))
			defer upstream.Close()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/admin/channels/models", strings.NewReader(`{"base_url":"`+upstream.URL+`","api_key":"fake-models-key","upstream_format":"`+format+`"}`))
			(&Service{httpClient: upstream.Client()}).fetchChannelModels(recorder, request)
			if recorder.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
			}
		})
	}
}
