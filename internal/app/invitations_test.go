package app

import "testing"

func TestNewInvitationCode(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		code, err := newInvitationCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 10 {
			t.Fatalf("unexpected code length: %q", code)
		}
		if seen[code] {
			t.Fatalf("duplicate invitation code: %q", code)
		}
		seen[code] = true
		for _, char := range code {
			if !containsRune(invitationAlphabet, char) {
				t.Fatalf("code contains invalid character: %q", code)
			}
		}
	}
}

func TestMaskInvitationEmail(t *testing.T) {
	tests := map[string]string{
		"alice@example.com": "a***@example.com",
		"x@example.com":     "x***@example.com",
		"invalid":           "***",
		"@example.com":      "***",
	}
	for input, want := range tests {
		if got := maskInvitationEmail(input); got != want {
			t.Errorf("maskInvitationEmail(%q) = %q, want %q", input, got, want)
		}
	}
}

func containsRune(value string, target rune) bool {
	for _, char := range value {
		if char == target {
			return true
		}
	}
	return false
}
