package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

type target struct {
	Table  string
	IDCol  string
	Column string
	Where  string
}

var targets = []target{
	{Table: "api_keys", IDCol: "id", Column: "secret_encrypted", Where: "secret_encrypted<>''"},
	{Table: "payment_settings", IDCol: "provider", Column: "merchant_key_encrypted", Where: "merchant_key_encrypted<>''"},
	{Table: "oauth_providers", IDCol: "id", Column: "client_secret_encrypted", Where: "client_secret_encrypted<>''"},
	{Table: "invoice_settings", IDCol: "id", Column: "client_secret_encrypted", Where: "client_secret_encrypted<>''"},
	{Table: "site_settings", IDCol: "id", Column: "geetest_captcha_key_encrypted", Where: "geetest_captcha_key_encrypted<>''"},
	{Table: "site_settings", IDCol: "id", Column: "corptcha_secret_encrypted", Where: "corptcha_secret_encrypted<>''"},
	{Table: "site_settings", IDCol: "id", Column: "smtp_password_encrypted", Where: "smtp_password_encrypted<>''"},
	{Table: "channels", IDCol: "id", Column: "api_key", Where: "api_key<>''"},
	{Table: "channel_api_keys", IDCol: "id", Column: "key_encrypted", Where: "key_encrypted<>''"},
}

func main() {
	var dsn, oldKey, newKey string
	var dryRun bool
	flag.StringVar(&dsn, "database-url", "", "PostgreSQL database URL")
	flag.StringVar(&oldKey, "old-key", "", "old ENCRYPTION_KEY")
	flag.StringVar(&newKey, "new-key", "", "new ENCRYPTION_KEY")
	flag.BoolVar(&dryRun, "dry-run", false, "verify decryption without changing the database")
	flag.Parse()

	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if oldKey == "" {
		oldKey = os.Getenv("OLD_ENCRYPTION_KEY")
	}
	if newKey == "" {
		newKey = os.Getenv("NEW_ENCRYPTION_KEY")
	}
	if dsn == "" || oldKey == "" || newKey == "" {
		log.Fatal("DATABASE_URL, OLD_ENCRYPTION_KEY, and NEW_ENCRYPTION_KEY are required")
	}
	if oldKey == newKey {
		log.Fatal("old and new keys must differ")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var totalRotated, totalSkipped int
	for _, t := range targets {
		rotated, skipped, err := rotateTarget(ctx, tx, oldKey, newKey, t, dryRun)
		if err != nil {
			log.Fatalf("rotate %s.%s: %v", t.Table, t.Column, err)
		}
		totalRotated += rotated
		totalSkipped += skipped
		fmt.Printf("%s.%s rotated=%d skipped=%d\n", t.Table, t.Column, rotated, skipped)
	}
	if dryRun {
		fmt.Printf("dry-run complete rotated=%d skipped=%d\n", totalRotated, totalSkipped)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit transaction: %v", err)
	}
	fmt.Printf("done rotated=%d skipped=%d\n", totalRotated, totalSkipped)
}

func rotateTarget(ctx context.Context, tx pgx.Tx, oldKey, newKey string, t target, dryRun bool) (int, int, error) {
	query := fmt.Sprintf("select %s::text,%s from %s where %s for update", t.IDCol, t.Column, t.Table, t.Where)
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	type row struct {
		id    string
		value string
		plain string
	}
	var pending []row
	skipped := 0
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.value); err != nil {
			return 0, 0, err
		}
		plain, err := crypt(oldKey, r.value, true)
		if err != nil {
			skipped++
			continue
		}
		r.plain = plain
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	update := fmt.Sprintf("update %s set %s=$1 where %s::text=$2", t.Table, t.Column, t.IDCol)
	if dryRun {
		return len(pending), skipped, nil
	}
	for _, r := range pending {
		encrypted, err := crypt(newKey, r.plain, false)
		if err != nil {
			return 0, 0, err
		}
		if _, err := tx.Exec(ctx, update, encrypted, r.id); err != nil {
			return 0, 0, err
		}
	}
	return len(pending), skipped, nil
}

func crypt(key, value string, decrypt bool) (string, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if decrypt {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return "", err
		}
		if len(raw) < gcm.NonceSize() {
			return "", fmt.Errorf("invalid encrypted value")
		}
		plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
		return string(plain), err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(out), nil
}
