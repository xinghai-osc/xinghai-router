package app

import (
	"os"
	"strings"
	"testing"
	"unicode"
)

func TestBootstrapAdminEmailDefaultAndOverride(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "")
	if got := bootstrapAdminEmail(); got != defaultBootstrapAdminEmail {
		t.Fatalf("default email = %q, want %q", got, defaultBootstrapAdminEmail)
	}
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", " Ops@Example.COM ")
	if got := bootstrapAdminEmail(); got != "ops@example.com" {
		t.Fatalf("override email = %q, want ops@example.com", got)
	}
}

func TestBootstrapAdminEmailWhitespaceOnlyUsesDefault(t *testing.T) {
	for _, value := range []string{" ", "   ", "\t", "\n", " \t \n "} {
		t.Setenv("BOOTSTRAP_ADMIN_EMAIL", value)
		if got := bootstrapAdminEmail(); got != defaultBootstrapAdminEmail {
			t.Fatalf("whitespace-only email env %q = %q, want %q", value, got, defaultBootstrapAdminEmail)
		}
	}
}

func TestBootstrapAdminNameDefaultAndOverride(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_NAME", "")
	if got := bootstrapAdminName(); got != defaultBootstrapAdminName {
		t.Fatalf("default name = %q, want %q", got, defaultBootstrapAdminName)
	}
	t.Setenv("BOOTSTRAP_ADMIN_NAME", " Site Ops ")
	if got := bootstrapAdminName(); got != "Site Ops" {
		t.Fatalf("override name = %q, want Site Ops", got)
	}
}

func TestBootstrapAdminNameWhitespaceOnlyUsesDefault(t *testing.T) {
	for _, value := range []string{" ", "   ", "\t", "\n", " \t \n "} {
		t.Setenv("BOOTSTRAP_ADMIN_NAME", value)
		if got := bootstrapAdminName(); got != defaultBootstrapAdminName {
			t.Fatalf("whitespace-only name env %q = %q, want %q", value, got, defaultBootstrapAdminName)
		}
	}
}

func TestRandomPasswordLengthAndCharset(t *testing.T) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*-_"
	password, err := randomPassword(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != 24 {
		t.Fatalf("len = %d, want 24", len(password))
	}
	for _, r := range password {
		if !strings.ContainsRune(alphabet, r) {
			t.Fatalf("unexpected character %q in password", r)
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("!@#$%^&*-_", r)) {
			t.Fatalf("unexpected character class %q in password", r)
		}
	}
	other, err := randomPassword(24)
	if err != nil {
		t.Fatal(err)
	}
	if password == other {
		t.Fatal("expected distinct random passwords")
	}
}

func TestRandomPasswordMinimumLength(t *testing.T) {
	for _, length := range []int{0, 1, 4, 7, 8} {
		password, err := randomPassword(length)
		if err != nil {
			t.Fatal(err)
		}
		want := length
		if want < 8 {
			want = 8
		}
		if len(password) != want {
			t.Fatalf("randomPassword(%d) len = %d, want %d", length, len(password), want)
		}
	}
	password, err := randomPassword(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != 32 {
		t.Fatalf("len = %d, want 32", len(password))
	}
}

func TestBootstrapAdminEnvIsolation(t *testing.T) {
	if err := os.Unsetenv("BOOTSTRAP_ADMIN_EMAIL"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("BOOTSTRAP_ADMIN_NAME"); err != nil {
		t.Fatal(err)
	}
	if bootstrapAdminEmail() != defaultBootstrapAdminEmail {
		t.Fatal("expected default email after unset")
	}
	if bootstrapAdminName() != defaultBootstrapAdminName {
		t.Fatal("expected default name after unset")
	}
}
func TestBootstrapConflictPolicy(t *testing.T) {
	if got := bootstrapConflictPolicy("user"); got != bootstrapConflictRefusePromote {
		t.Fatalf("role=user => %q, want %q", got, bootstrapConflictRefusePromote)
	}
	if got := bootstrapConflictPolicy("admin"); got != bootstrapConflictAlreadyAdmin {
		t.Fatalf("role=admin => %q, want %q", got, bootstrapConflictAlreadyAdmin)
	}
	for _, role := range []string{"", "moderator", "operator"} {
		if got := bootstrapConflictPolicy(role); got != bootstrapConflictRefusePromote {
			t.Fatalf("role=%q => %q, want %q", role, got, bootstrapConflictRefusePromote)
		}
	}
}
