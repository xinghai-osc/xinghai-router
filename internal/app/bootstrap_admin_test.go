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

func TestRandomPasswordLengthAndCharset(t *testing.T) {
	password, err := randomPassword(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != 24 {
		t.Fatalf("len = %d, want 24", len(password))
	}
	for _, r := range password {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("!@#$%^&*-_", r)) {
			t.Fatalf("unexpected character %q in password", r)
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
	password, err := randomPassword(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) < 8 {
		t.Fatalf("len = %d, want at least 8", len(password))
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
