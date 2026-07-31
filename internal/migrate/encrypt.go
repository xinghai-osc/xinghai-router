package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EncryptExistingChannelKeys encrypts any plaintext channel keys stored in the
// target database. It is idempotent: already-encrypted values are skipped.
func EncryptExistingChannelKeys(ctx context.Context, targetDSN, encryptionKey string, progress ProgressFunc) error {
	if progress != nil {
		progress(Progress{Step: "connect", Detail: "Connecting to target database"})
	}
	if encryptionKey == "" {
		return fmt.Errorf("encryption key is required")
	}

	target, err := pgxpool.New(ctx, targetDSN)
	if err != nil {
		return fmt.Errorf("connect target database: %w", err)
	}
	defer target.Close()

	if err := target.Ping(ctx); err != nil {
		return fmt.Errorf("ping target database: %w", err)
	}

	if progress != nil {
		progress(Progress{Step: "channels", Detail: "Encrypting channel primary keys"})
	}
	rows, err := target.Query(ctx, `select id, api_key from channels where api_key <> ''`)
	if err != nil {
		return fmt.Errorf("query channels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			continue
		}
		enc, err := encryptIfNeeded(encryptionKey, key)
		if err != nil {
			return fmt.Errorf("encrypt channel %s key: %w", id, err)
		}
		if enc == key {
			continue
		}
		if _, err := target.Exec(ctx, `update channels set api_key=$1 where id=$2`, enc, id); err != nil {
			return fmt.Errorf("update channel %s: %w", id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("channel rows: %w", err)
	}

	if progress != nil {
		progress(Progress{Step: "channel_api_keys", Detail: "Encrypting channel alternate keys"})
	}
	krows, err := target.Query(ctx, `select id, key_encrypted from channel_api_keys where key_encrypted <> ''`)
	if err != nil {
		return fmt.Errorf("query channel api keys: %w", err)
	}
	defer krows.Close()
	for krows.Next() {
		var id, key string
		if err := krows.Scan(&id, &key); err != nil {
			continue
		}
		enc, err := encryptIfNeeded(encryptionKey, key)
		if err != nil {
			return fmt.Errorf("encrypt channel api key %s: %w", id, err)
		}
		if enc == key {
			continue
		}
		if _, err := target.Exec(ctx, `update channel_api_keys set key_encrypted=$1 where id=$2`, enc, id); err != nil {
			return fmt.Errorf("update channel api key %s: %w", id, err)
		}
	}
	if err := krows.Err(); err != nil {
		return fmt.Errorf("channel api key rows: %w", err)
	}

	if progress != nil {
		progress(Progress{Step: "done", Detail: "Finished encrypting channel keys"})
	}
	return nil
}
