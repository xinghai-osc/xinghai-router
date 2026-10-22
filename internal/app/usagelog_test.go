package app

import (
	"strings"
	"testing"
)

// decodePostgresEString returns the value of the first E'...' literal in sql
// following PostgreSQL escape-string rules: a backslash consumes the next
// character and '' is an escaped quote. It reports false when the literal is
// never terminated, which is what PostgreSQL rejects as an unterminated
// quoted string.
func decodePostgresEString(sql string) (string, bool) {
	start := strings.Index(sql, "E'")
	if start < 0 {
		return "", false
	}
	var value strings.Builder
	for i := start + 2; i < len(sql); i++ {
		switch sql[i] {
		case '\\':
			if i+1 >= len(sql) {
				return "", false
			}
			i++
			value.WriteByte(sql[i])
		case '\'':
			if i+1 < len(sql) && sql[i+1] == '\'' {
				i++
				value.WriteByte('\'')
				continue
			}
			return value.String(), true
		default:
			value.WriteByte(sql[i])
		}
	}
	return "", false
}

func TestLikeEscapeClauseIsWellFormed(t *testing.T) {
	escape, ok := decodePostgresEString(likeEscapeClause)
	if !ok {
		t.Fatalf("escape clause %q is not a terminated PostgreSQL E-string", likeEscapeClause)
	}
	if escape != `\` {
		t.Fatalf("escape clause %q decodes to %q, want a single backslash", likeEscapeClause, escape)
	}
}

func TestEscapeLikePatternMatchesBackslashEscape(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"plain-model", "plain-model"},
		{"with%percent", `with\%percent`},
		{"with_underscore", `with\_underscore`},
		{`with\backslash`, `with\\backslash`},
		{"", ""},
	} {
		if got := escapeLikePattern(tc.value); got != tc.want {
			t.Fatalf("escapeLikePattern(%q)=%q, want %q", tc.value, got, tc.want)
		}
	}
}
