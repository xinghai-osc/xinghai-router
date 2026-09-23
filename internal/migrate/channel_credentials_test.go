package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const fakeEncryptionKey = "fake-import-encryption-key-2026"

func TestChannelCredentialImportMode(t *testing.T) {
	for _, mode := range []string{"", "plaintext", "encrypted", "invalid"} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Setenv("CHANNEL_CREDENTIAL_STORAGE", mode)
			parsed, err := channelCredentialStorage()
			if mode == "invalid" {
				if err == nil {
					t.Fatal("invalid mode accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			stored, err := importChannelCredential(fakeEncryptionKey, "abcdefghijklmnop", parsed)
			if err != nil {
				t.Fatal(err)
			}
			if parsed == "encrypted" && !strings.HasPrefix(stored, channelCredentialPrefix) {
				t.Fatal("base64-shaped plaintext must be encrypted")
			}
			if parsed == "plaintext" && stored != "abcdefghijklmnop" {
				t.Fatal("plaintext mode changed the key")
			}
		})
	}
	legacy, err := crypt(fakeEncryptionKey, "fake-legacy-key", false)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := encryptIfNeeded(fakeEncryptionKey, legacy)
	if err != nil || !strings.HasPrefix(stored, channelCredentialPrefix) {
		t.Fatal("legacy ciphertext was not upgraded")
	}
	plain, err := channelCredentialValue(fakeEncryptionKey, stored)
	if err != nil || plain != "fake-legacy-key" {
		t.Fatal("legacy key value changed")
	}
	repeat, err := encryptIfNeeded(fakeEncryptionKey, stored)
	if err != nil || repeat != stored {
		t.Fatal("migration is not idempotent")
	}
	for _, bad := range []string{channelCredentialPrefix + "broken", channelCredentialFormat + "v2:unknown"} {
		if _, err := encryptIfNeeded(fakeEncryptionKey, bad); err == nil {
			t.Fatal("malformed tagged value was accepted")
		}
	}
	if _, err := encryptIfNeeded("wrong-fake-encryption-key", stored); err == nil {
		t.Fatal("wrong encryption key was accepted")
	}
}

func importCredentialDatabase(t *testing.T) (*pgxpool.Pool, string) {
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
	id, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "import_credentials_" + strings.ReplaceAll(id, "-", "")
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
	if _, err := db.Exec(ctx, `create table channels(id bigserial primary key,name text unique,base_url text,api_key text,models jsonb,enabled bool,priority int,weight int,provider text,key_type text,created_at timestamptz,updated_at timestamptz); create table channel_api_keys(id uuid default gen_random_uuid() primary key,channel_id bigint references channels(id),name text,key_encrypted text,enabled bool);`); err != nil {
		t.Fatal(err)
	}
	return db, dsn + "&search_path=" + schema
}

type credentialSourceConnector struct{}
type credentialSourceDriver struct{}
type credentialSourceConn struct{}
type credentialSourceRows struct{ done bool }

func (credentialSourceConnector) Connect(context.Context) (driver.Conn, error) {
	return credentialSourceConn{}, nil
}
func (credentialSourceConnector) Driver() driver.Driver { return credentialSourceDriver{} }
func (credentialSourceDriver) Open(string) (driver.Conn, error) {
	return credentialSourceConn{}, nil
}
func (credentialSourceConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (credentialSourceConn) Close() error                        { return nil }
func (credentialSourceConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (credentialSourceConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &credentialSourceRows{}, nil
}
func (*credentialSourceRows) Columns() []string {
	return []string{"id", "type", "key", "name", "status", "base_url", "models", "group", "priority", "weight", "created_time", "model_mapping"}
}
func (*credentialSourceRows) Close() error { return nil }
func (r *credentialSourceRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(dest, []driver.Value{int64(1), int64(1), "fake-first-key\nfake-second-key", "fake-import", int64(1), "https://example.test", "fake-model", "", int64(100), int64(100), int64(1700000000), nil})
	return nil
}

func TestChannelCredentialImportDatabase(t *testing.T) {
	db, dsn := importCredentialDatabase(t)
	ctx := context.Background()
	source := sql.OpenDB(credentialSourceConnector{})
	defer source.Close()
	for _, mode := range []string{"plaintext", "encrypted"} {
		if _, err := db.Exec(ctx, `truncate channels,channel_api_keys restart identity`); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			mapping, err := migrateChannels(ctx, source, db, nil, fakeEncryptionKey, mode)
			if err != nil || len(mapping) != 1 {
				t.Fatalf("import mode=%s mapping=%v err=%v", mode, mapping, err)
			}
		}
		var count int
		if err := db.QueryRow(ctx, `select count(*) from channel_api_keys`).Scan(&count); err != nil || count != 2 {
			t.Fatal("repeated encrypted import duplicated channel keys")
		}
		var stored string
		if err := db.QueryRow(ctx, `select api_key from channels`).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if (mode == "encrypted") != strings.HasPrefix(stored, channelCredentialPrefix) {
			t.Fatal("import ignored storage mode")
		}
	}
	t.Setenv("CHANNEL_CREDENTIAL_STORAGE", "encrypted")
	if _, err := db.Exec(ctx, `update channels set api_key='fake-pending-key'; update channel_api_keys set key_encrypted='xh-credential:v1:broken'`); err != nil {
		t.Fatal(err)
	}
	if err := EncryptExistingChannelKeys(ctx, dsn, fakeEncryptionKey, nil); err == nil {
		t.Fatal("corrupt tagged value did not abort migration")
	}
	var stored string
	if err := db.QueryRow(ctx, `select api_key from channels`).Scan(&stored); err != nil || stored != "fake-pending-key" {
		t.Fatal("failed migration partially committed")
	}
	if _, err := db.Exec(ctx, `update channel_api_keys set key_encrypted='fake-repaired-key'`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := EncryptExistingChannelKeys(ctx, dsn, fakeEncryptionKey, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow(ctx, `select api_key from channels`).Scan(&stored); err != nil || !strings.HasPrefix(stored, channelCredentialPrefix) {
		t.Fatal("explicit migration did not encrypt")
	}
	t.Setenv("CHANNEL_CREDENTIAL_STORAGE", "plaintext")
	if err := EncryptExistingChannelKeys(ctx, "invalid-dsn", fakeEncryptionKey, nil); err == nil || !strings.Contains(err.Error(), "CHANNEL_CREDENTIAL_STORAGE") {
		t.Fatal("migration must reject plaintext mode before opening database")
	}
}
