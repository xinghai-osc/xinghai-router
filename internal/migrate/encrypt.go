package migrate

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EncryptExistingChannelKeys(ctx context.Context, targetDSN, encryptionKey string, progress ProgressFunc) error {
	mode, err := channelCredentialStorage()
	if err != nil {
		return err
	}
	if mode != "encrypted" {
		return fmt.Errorf("CHANNEL_CREDENTIAL_STORAGE=encrypted is required for channel credential migration")
	}
	if len(encryptionKey) < 24 {
		return fmt.Errorf("ENCRYPTION_KEY must contain at least 24 characters")
	}
	if progress != nil {
		progress(Progress{Step: "connect", Detail: "Connecting to target database"})
	}
	target, err := pgxpool.New(ctx, targetDSN)
	if err != nil {
		return fmt.Errorf("connect target database: %w", err)
	}
	defer target.Close()
	tx, err := target.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `lock table channels,channel_api_keys in share row exclusive mode`); err != nil {
		return err
	}
	for _, table := range []struct{ step, query, update string }{
		{"channels", `select id::text,api_key from channels where api_key<>''`, `update channels set api_key=$1 where id=$2`},
		{"channel_api_keys", `select id::text,key_encrypted from channel_api_keys where key_encrypted<>''`, `update channel_api_keys set key_encrypted=$1 where id=$2`},
	} {
		if progress != nil {
			progress(Progress{Step: table.step, Detail: "Encrypting channel credentials"})
		}
		rows, err := tx.Query(ctx, table.query)
		if err != nil {
			return err
		}
		type credential struct{ id, stored string }
		var pending []credential
		for rows.Next() {
			var row credential
			if err := rows.Scan(&row.id, &row.stored); err != nil {
				rows.Close()
				return err
			}
			converted, err := encryptIfNeeded(encryptionKey, row.stored)
			if err != nil {
				rows.Close()
				return err
			}
			if converted != row.stored {
				row.stored = converted
				pending = append(pending, row)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, row := range pending {
			if _, err := tx.Exec(ctx, table.update, row.stored, row.id); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if progress != nil {
		progress(Progress{Step: "done", Detail: "Finished encrypting channel keys"})
	}
	return nil
}
