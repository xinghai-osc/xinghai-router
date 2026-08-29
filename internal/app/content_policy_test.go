package app

import "testing"

func TestExtractPolicyTextFindsUserContentAndTools(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello blocked phrase"}],"tools":[{"type":"function","function":{"name":"lookup","description":"safe tool"}}]}`)
	text := extractPolicyText(body)
	if text == "" || normalizePolicyText(text, false) == "" {
		t.Fatal("expected request text to be extracted")
	}
}

func TestEvaluateContentPolicyModes(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: "test encryption key"}}
	rule := contentPolicyRule{ID: "00000000-0000-4000-8000-000000000001", Name: "blocked", Term: "secret phrase", Action: "block", Enabled: true}
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"This is a secret phrase."}]}`)
	for _, test := range []struct {
		mode     string
		decision string
	}{
		{"off", "allow"},
		{"audit", "audit"},
		{"block", "block"},
	} {
		result := s.evaluateContentPolicy(contentPolicySnapshot{Settings: contentPolicySettings{Mode: test.mode, StoreMode: "hash"}, Rules: []contentPolicyRule{rule}}, body)
		if result.Decision != test.decision {
			t.Fatalf("mode %s decision = %s, want %s", test.mode, result.Decision, test.decision)
		}
		if test.mode != "off" && len(result.MatchedRuleIDs) != 1 {
			t.Fatalf("mode %s matched %d rules, want 1", test.mode, len(result.MatchedRuleIDs))
		}
	}
}

func TestRedactPolicyTerm(t *testing.T) {
	if got := redactPolicyTerm("secret phrase and secret phrase", "secret phrase", false); got != "[redacted] and [redacted]" {
		t.Fatalf("redaction = %q", got)
	}
}

func TestParsePolicyTermsSkipsBlankCommentsAndDuplicates(t *testing.T) {
	terms := parsePolicyTerms("foo\n\n  bar  \r\n# comment\nfoo\n\t\nbaz")
	want := []string{"foo", "bar", "baz"}
	if len(terms) != len(want) {
		t.Fatalf("parsed %d terms, want %d: %v", len(terms), len(want), terms)
	}
	for i := range want {
		if terms[i] != want[i] {
			t.Fatalf("term[%d] = %q, want %q", i, terms[i], want[i])
		}
	}
}
