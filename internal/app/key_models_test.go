package app

import (
	"strings"
	"testing"
)

func TestNormalizeAllowedModels(t *testing.T) {
	models, err := normalizeAllowedModels([]string{" gpt-4o ", "", "gpt-4o", "claude-3-5-sonnet"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "claude-3-5-sonnet" {
		t.Fatalf("models = %v", models)
	}
	empty, err := normalizeAllowedModels(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("nil input = %v, %v", empty, err)
	}
	if _, err := normalizeAllowedModels([]string{"bad model"}); err == nil {
		t.Fatal("expected rejection of a model name with a space")
	}
	if _, err := normalizeAllowedModels([]string{strings.Repeat("a", maxAllowedModelName+1)}); err == nil {
		t.Fatal("expected rejection of an over-long model name")
	}
	tooMany := make([]string, maxAllowedModels+1)
	for i := range tooMany {
		tooMany[i] = "model-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	if _, err := normalizeAllowedModels(tooMany); err == nil {
		t.Fatal("expected rejection of too many models")
	}
}

func TestModelAllowed(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		model   string
		want    bool
	}{
		{"unrestricted key", nil, "gpt-4o", true},
		{"unrestricted key with empty list", []string{}, "gpt-4o", true},
		{"exact match", []string{"gpt-4o"}, "gpt-4o", true},
		{"case insensitive", []string{"GPT-4O"}, "gpt-4o", true},
		{"different model", []string{"gpt-4o"}, "claude-3-5-sonnet", false},
		{"prefix wildcard", []string{"gpt-4*"}, "gpt-4o-mini", true},
		{"prefix wildcard rejects others", []string{"gpt-4*"}, "gpt-5-turbo", false},
		{"suffix wildcard", []string{"*-mini"}, "gpt-4o-mini", true},
		{"suffix wildcard rejects others", []string{"*-mini"}, "gpt-4o", false},
		{"middle wildcard", []string{"gpt-*-mini"}, "gpt-4o-mini", true},
		{"middle wildcard rejects others", []string{"gpt-*-mini"}, "gpt-4o", false},
		{"star allows every model", []string{"*"}, "anything", true},
		{"one of several", []string{"gpt-4o", "claude-3-5-sonnet"}, "claude-3-5-sonnet", true},
		{"none of several", []string{"gpt-4o", "claude-3-5-sonnet"}, "gemini-2.5-pro", false},
		{"empty model name", []string{"gpt-4o"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := modelAllowed(tc.allowed, tc.model); got != tc.want {
				t.Fatalf("modelAllowed(%v, %q) = %v, want %v", tc.allowed, tc.model, got, tc.want)
			}
		})
	}
}
