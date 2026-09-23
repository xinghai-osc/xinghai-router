package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func channelCredentialDatabase(t *testing.T) *Service {
	t.Helper()
	dsn := os.Getenv("CHANNEL_CREDENTIAL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CHANNEL_CREDENTIAL_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	id, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "channel_credentials_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "create schema "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "drop schema "+quoted+" cascade"); err != nil {
			t.Error(err)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return &Service{db: db, cfg: Config{EncryptionKey: channelCredentialTestKey, ChannelCredentialStorage: "encrypted"}}
}

func channelCredentialInsert(t *testing.T, s *Service, name, stored string) string {
	t.Helper()
	var id string
	if err := s.db.QueryRow(context.Background(), `insert into channels(name,base_url,api_key,models) values($1,'https://example.test',$2,'["fake-model"]') returning id::text`, name, stored).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func channelCredentialRequest(t *testing.T, handler http.HandlerFunc, body string, paths map[string]string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/admin/channels", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), accountContextKey{}, accountContext{role: "admin"}))
	for key, value := range paths {
		r.SetPathValue(key, value)
	}
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
	return w
}

func channelCredentialAssertStored(t *testing.T, s *Service, channelID string, expected []string) {
	t.Helper()
	rows, err := s.db.Query(context.Background(), `select api_key from channels where id=$1 union all select key_encrypted from channel_api_keys where channel_id=$1`, channelID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]int{}
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored == "" {
			continue
		}
		if s.cfg.ChannelCredentialStorage == "encrypted" && !strings.HasPrefix(stored, channelCredentialPrefix) {
			t.Fatal("a credential write left plaintext or ambiguous legacy ciphertext")
		}
		plain, err := channelKeyValue(s.cfg.EncryptionKey, stored)
		if err != nil {
			t.Fatal(err)
		}
		seen[plain]++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, key := range expected {
		if seen[key] == 0 {
			t.Fatal("expected usable credential is missing")
		}
		seen[key]--
	}
	for _, count := range seen {
		if count != 0 {
			t.Fatal("unexpected credential stored")
		}
	}
}

func TestChannelCredentialDatabaseMigration(t *testing.T) {
	s := channelCredentialDatabase(t)
	ctx := context.Background()
	legacy, err := crypt(channelCredentialTestKey, "fake-legacy-key", false)
	if err != nil {
		t.Fatal(err)
	}
	channelID := channelCredentialInsert(t, s, "plaintext", "fake-plaintext-key")
	legacyID := channelCredentialInsert(t, s, "legacy", legacy)
	channelCredentialInsert(t, s, "empty", "")
	if _, err := s.db.Exec(ctx, `insert into channel_api_keys(channel_id,key_encrypted) values($1,$2),($1,$3)`, channelID, legacy, "fake-second-key"); err != nil {
		t.Fatal(err)
	}
	result, err := s.migrateChannelCredentialRows(ctx)
	if err != nil || result.ChannelsMigrated != 2 || result.KeysMigrated != 2 || result.AlreadyEncrypted != 0 {
		t.Fatalf("migration result=%+v err=%v", result, err)
	}
	channelCredentialAssertStored(t, s, channelID, []string{"fake-plaintext-key", "fake-legacy-key", "fake-second-key"})
	channelCredentialAssertStored(t, s, legacyID, []string{"fake-legacy-key"})
	result, err = s.migrateChannelCredentialRows(ctx)
	if err != nil || result.ChannelsMigrated != 0 || result.KeysMigrated != 0 || result.AlreadyEncrypted != 4 {
		t.Fatalf("repeat result=%+v err=%v", result, err)
	}
	response := channelCredentialRequest(t, s.migrateChannelCredentials, `{"confirm":true}`, nil, http.StatusOK)
	if strings.Contains(response.Body.String(), "fake-") || strings.Contains(response.Body.String(), channelCredentialPrefix) {
		t.Fatal("migration response exposed credentials")
	}
	s.cfg.EncryptionKey = "wrong-fake-key"
	if _, err := s.migrateChannelCredentialRows(ctx); err == nil {
		t.Fatal("migration must fail with incorrect encryption key")
	}
	s.cfg.EncryptionKey = channelCredentialTestKey
	if _, err := s.db.Exec(ctx, `update channels set api_key='fake-pending-key' where id=$1;`, channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `update channel_api_keys set key_encrypted=$1 where channel_id=$2`, channelCredentialPrefix+"broken", channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.migrateChannelCredentialRows(ctx); err == nil {
		t.Fatal("corrupt tagged key must roll back migration")
	}
	var stored string
	if err := s.db.QueryRow(ctx, `select api_key from channels where id=$1`, channelID).Scan(&stored); err != nil || stored != "fake-pending-key" {
		t.Fatal("failed migration committed a partial channel update")
	}
}

func TestChannelCredentialDatabaseWritePaths(t *testing.T) {
	s := channelCredentialDatabase(t)
	ctx := context.Background()
	for _, mode := range []string{"plaintext", "encrypted"} {
		t.Run(mode, func(t *testing.T) {
			s.cfg.ChannelCredentialStorage = mode
			w := channelCredentialRequest(t, s.createChannel, `{"name":"`+mode+`","base_url":"https://example.test","api_keys":"fake-first-key\nfake-second-key","key_type":"multi","models":["fake-model"]}`, nil, http.StatusCreated)
			var created struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			channelCredentialAssertStored(t, s, created.ID, []string{"fake-first-key", "fake-first-key", "fake-second-key"})
			if mode == "plaintext" {
				var raw string
				if err := s.db.QueryRow(ctx, `select api_key from channels where id=$1`, created.ID).Scan(&raw); err != nil || raw != "fake-first-key" {
					t.Fatal("default plaintext policy changed")
				}
			}
			paths := map[string]string{"id": created.ID}
			channelCredentialRequest(t, s.updateChannel, `{"name":"`+mode+`","base_url":"https://example.test","api_keys":"fake-replaced-key","key_type":"single","models":["fake-model"]}`, paths, http.StatusNoContent)
			channelCredentialAssertStored(t, s, created.ID, []string{"fake-replaced-key", "fake-replaced-key"})
			w = channelCredentialRequest(t, s.createChannelKey, `{"name":"extra","api_key":"fake-extra-key"}`, paths, http.StatusCreated)
			var key struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &key); err != nil {
				t.Fatal(err)
			}
			channelCredentialRequest(t, s.updateChannelKey, `{"api_key":"fake-updated-key"}`, map[string]string{"id": created.ID, "keyId": key.ID}, http.StatusNoContent)
			channelCredentialAssertStored(t, s, created.ID, []string{"fake-replaced-key", "fake-replaced-key", "fake-updated-key"})
			w = channelCredentialRequest(t, s.copyChannel, `{"name":"`+mode+`-copy"}`, paths, http.StatusCreated)
			var copied struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &copied); err != nil {
				t.Fatal(err)
			}
			channelCredentialAssertStored(t, s, copied.ID, []string{"fake-replaced-key", "fake-replaced-key", "fake-updated-key"})
			channelCredentialRequest(t, s.updateChannel, `{"name":"`+mode+`-copy","base_url":"https://example.test","api_keys":"fake-rolled-back-key","key_type":"single","models":["fake-model"]}`, paths, http.StatusConflict)
			channelCredentialAssertStored(t, s, created.ID, []string{"fake-replaced-key", "fake-replaced-key", "fake-updated-key"})
		})
	}
	s.cfg.ChannelCredentialStorage = "encrypted"
	legacyID := channelCredentialInsert(t, s, "legacy-source", "fake-legacy-copy-key")
	channelCredentialRequest(t, s.migrateChannelKeys, `{}`, map[string]string{"id": legacyID}, http.StatusOK)
	var stored string
	if err := s.db.QueryRow(ctx, `select key_encrypted from channel_api_keys where channel_id=$1`, legacyID).Scan(&stored); err != nil || !strings.HasPrefix(stored, channelCredentialPrefix) {
		t.Fatal("legacy key promotion bypassed encrypted storage")
	}
	w := channelCredentialRequest(t, s.copyChannel, `{"name":"legacy-copy"}`, map[string]string{"id": legacyID}, http.StatusCreated)
	var copied struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &copied); err != nil {
		t.Fatal(err)
	}
	channelCredentialAssertStored(t, s, copied.ID, []string{"fake-legacy-copy-key", "fake-legacy-copy-key"})
}

func TestChannelCredentialDatabaseMigrationWaitsForWriters(t *testing.T) {
	s := channelCredentialDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	channelID := channelCredentialInsert(t, s, "concurrent", "fake-initial-key")
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `insert into channel_api_keys(channel_id,key_encrypted) values($1,'fake-inflight-key')`, channelID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.migrateChannelCredentialRows(ctx)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("migration did not wait for an in-flight writer: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	channelCredentialAssertStored(t, s, channelID, []string{"fake-initial-key", "fake-inflight-key"})
}
