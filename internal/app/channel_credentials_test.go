package app

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const channelCredentialTestKey = "fake-channel-encryption-test-key-2026"

func TestChannelCredentialStorageModes(t *testing.T) {
	for _, mode := range []string{"", "plaintext", "encrypted"} {
		t.Run("mode="+mode, func(t *testing.T) {
			s := &Service{cfg: Config{EncryptionKey: channelCredentialTestKey, ChannelCredentialStorage: mode}}
			stored, err := s.storeChannelCredential("fake-provider-key")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "encrypted" {
				if !strings.HasPrefix(stored, channelCredentialPrefix) || strings.Contains(stored, "fake-provider-key") {
					t.Fatal("encrypted mode did not store a tagged ciphertext")
				}
				second, err := s.storeChannelCredential("fake-provider-key")
				if err != nil || stored == second {
					t.Fatal("encryption must use a fresh nonce")
				}
			} else if stored != "fake-provider-key" {
				t.Fatal("plaintext compatibility mode changed the stored key")
			}
			plain, err := channelKeyValue(channelCredentialTestKey, stored)
			if err != nil || plain != "fake-provider-key" {
				t.Fatal("credential round trip failed")
			}
			empty, err := s.storeChannelCredential("")
			if err != nil || empty != "" {
				t.Fatal("empty legacy fallback must remain empty")
			}
		})
	}
	s := &Service{cfg: Config{ChannelCredentialStorage: "unknown"}}
	if _, err := s.storeChannelCredential("fake-key"); err == nil {
		t.Fatal("invalid storage mode must not silently write plaintext")
	}
}

func TestChannelCredentialReadsFailClosed(t *testing.T) {
	s := &Service{cfg: Config{EncryptionKey: channelCredentialTestKey, ChannelCredentialStorage: "encrypted"}}
	stored, err := s.storeChannelCredential("fake-provider-key")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, channelCredentialPrefix))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	tampered := channelCredentialPrefix + base64.RawURLEncoding.EncodeToString(raw)
	for _, tc := range []struct{ key, stored string }{
		{"wrong-fake-encryption-key", stored},
		{channelCredentialTestKey, tampered},
		{channelCredentialTestKey, channelCredentialPrefix},
		{channelCredentialTestKey, channelCredentialPrefix + "bad-value"},
		{channelCredentialTestKey, channelCredentialFormat + "v2:fake-value"},
		{channelCredentialTestKey, ""},
	} {
		plain, err := channelKeyValue(tc.key, tc.stored)
		if err == nil || plain != "" {
			t.Fatal("invalid tagged credential must not be forwarded as plaintext")
		}
		if strings.Contains(err.Error(), "fake-provider-key") || strings.Contains(err.Error(), stored) {
			t.Fatal("credential error exposed sensitive data")
		}
	}
}

func TestChannelCredentialLegacyCompatibilityAndCopy(t *testing.T) {
	legacy, err := crypt(channelCredentialTestKey, "fake-legacy-key", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ key, stored, want string }{
		{channelCredentialTestKey, "fake-plaintext-key", "fake-plaintext-key"},
		{channelCredentialTestKey, legacy, "fake-legacy-key"},
		{"wrong-fake-encryption-key", legacy, legacy},
	} {
		plain, err := channelKeyValue(tc.key, tc.stored)
		if err != nil || plain != tc.want {
			t.Fatal("unprefixed legacy compatibility changed")
		}
	}
	for _, mode := range []string{"", "plaintext", "encrypted"} {
		s := &Service{cfg: Config{EncryptionKey: channelCredentialTestKey, ChannelCredentialStorage: mode}}
		for _, source := range []string{"fake-plaintext-key", legacy} {
			copied, err := s.copyChannelCredential(source)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "encrypted" && !strings.HasPrefix(copied, channelCredentialPrefix) {
				t.Fatal("copy must encrypt legacy source in encrypted mode")
			}
			if mode != "encrypted" && copied != source {
				t.Fatal("plaintext copy should preserve existing storage")
			}
			plain, err := channelKeyValue(channelCredentialTestKey, copied)
			want, _ := channelKeyValue(channelCredentialTestKey, source)
			if err != nil || plain != want {
				t.Fatal("copy changed usable key")
			}
			repeated, err := s.copyChannelCredential(copied)
			if err != nil || repeated != copied {
				t.Fatal("migration of already converted credentials must be idempotent")
			}
		}
		if _, err := s.copyChannelCredential(channelCredentialPrefix + "broken"); err == nil {
			t.Fatal("copy must reject corrupt tagged ciphertext")
		}
	}
}

func TestChannelCredentialMigrationRequiresAdminModeAndConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name, role, mode, body string
		status                 int
	}{
		{"unauthenticated", "", "encrypted", `{"confirm":true}`, http.StatusForbidden},
		{"operator", "operator", "encrypted", `{"confirm":true}`, http.StatusForbidden},
		{"plaintext", "admin", "plaintext", `{"confirm":true}`, http.StatusConflict},
		{"default", "admin", "", `{"confirm":true}`, http.StatusConflict},
		{"missing confirmation", "admin", "encrypted", `{}`, http.StatusBadRequest},
		{"false confirmation", "admin", "encrypted", `{"confirm":false}`, http.StatusBadRequest},
		{"invalid confirmation", "admin", "encrypted", `{"confirm":"true"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/admin/channels/credentials/migrate", strings.NewReader(tc.body))
			if tc.role != "" {
				r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{role: tc.role, permissions: map[string]bool{"channels.manage": true}}))
			}
			w := httptest.NewRecorder()
			s := &Service{cfg: Config{ChannelCredentialStorage: tc.mode}}
			s.migrateChannelCredentials(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d", w.Code, tc.status)
			}
		})
	}
}
