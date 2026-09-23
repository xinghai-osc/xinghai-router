package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

const channelCredentialFormat = "xh-credential:"
const channelCredentialPrefix = channelCredentialFormat + "v1:"

func (s *Service) storeChannelCredential(plain string) (string, error) {
	switch s.cfg.ChannelCredentialStorage {
	case "", "plaintext":
		if strings.HasPrefix(plain, channelCredentialFormat) {
			return "", fmt.Errorf("channel credential uses a reserved storage prefix")
		}
		return plain, nil
	case "encrypted":
		if plain == "" {
			return "", nil
		}
		encrypted, err := crypt(s.cfg.EncryptionKey, plain, false)
		if err != nil {
			return "", fmt.Errorf("encrypt channel credential: %w", err)
		}
		return channelCredentialPrefix + encrypted, nil
	default:
		return "", fmt.Errorf("invalid channel credential storage mode")
	}
}

func (s *Service) copyChannelCredential(stored string) (string, error) {
	if stored == "" {
		return s.storeChannelCredential("")
	}
	plain, err := channelKeyValue(s.cfg.EncryptionKey, stored)
	if err != nil {
		return "", err
	}
	switch s.cfg.ChannelCredentialStorage {
	case "", "plaintext":
		return stored, nil
	case "encrypted":
		if strings.HasPrefix(stored, channelCredentialPrefix) {
			return stored, nil
		}
		return s.storeChannelCredential(plain)
	default:
		return "", fmt.Errorf("invalid channel credential storage mode")
	}
}

func (s *Service) copyChannelAPIKeys(ctx context.Context, tx pgx.Tx, targetID, sourceID string) error {
	rows, err := tx.Query(ctx, `select name,key_encrypted,enabled,priority from channel_api_keys where channel_id=$1 order by priority desc nulls last,created_at`, sourceID)
	if err != nil {
		return err
	}
	type keyRow struct {
		name, stored string
		enabled      bool
		priority     int
	}
	var keys []keyRow
	for rows.Next() {
		var key keyRow
		if err := rows.Scan(&key.name, &key.stored, &key.enabled, &key.priority); err != nil {
			rows.Close()
			return err
		}
		key.stored, err = s.copyChannelCredential(key.stored)
		if err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := tx.Exec(ctx, `insert into channel_api_keys(channel_id,name,key_encrypted,enabled,priority) values($1,$2,$3,$4,$5)`, targetID, key.name, key.stored, key.enabled, key.priority); err != nil {
			return err
		}
	}
	return nil
}

type channelCredentialMigrationResult struct {
	ChannelsMigrated int `json:"channels_migrated"`
	KeysMigrated     int `json:"keys_migrated"`
	AlreadyEncrypted int `json:"already_encrypted"`
}

func (s *Service) migrateChannelCredentialRows(ctx context.Context) (channelCredentialMigrationResult, error) {
	var result channelCredentialMigrationResult
	if s.cfg.ChannelCredentialStorage != "encrypted" {
		return result, fmt.Errorf("channel credential migration requires encrypted storage mode")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `lock table channels,channel_api_keys in share row exclusive mode`); err != nil {
		return result, err
	}
	for _, target := range []struct {
		query  string
		update string
		count  *int
	}{
		{`select id::text,api_key from channels where api_key<>'' order by id`, `update channels set api_key=$1 where id=$2`, &result.ChannelsMigrated},
		{`select id::text,key_encrypted from channel_api_keys where key_encrypted<>'' order by id`, `update channel_api_keys set key_encrypted=$1 where id=$2`, &result.KeysMigrated},
	} {
		rows, err := tx.Query(ctx, target.query)
		if err != nil {
			return channelCredentialMigrationResult{}, err
		}
		type credentialRow struct {
			id, stored string
		}
		var pending []credentialRow
		for rows.Next() {
			var row credentialRow
			if err := rows.Scan(&row.id, &row.stored); err != nil {
				rows.Close()
				return channelCredentialMigrationResult{}, err
			}
			converted, err := s.copyChannelCredential(row.stored)
			if err != nil {
				rows.Close()
				return channelCredentialMigrationResult{}, err
			}
			if converted == row.stored {
				result.AlreadyEncrypted++
				continue
			}
			row.stored = converted
			pending = append(pending, row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return channelCredentialMigrationResult{}, err
		}
		for _, row := range pending {
			if _, err := tx.Exec(ctx, target.update, row.stored, row.id); err != nil {
				return channelCredentialMigrationResult{}, err
			}
			*target.count++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return channelCredentialMigrationResult{}, err
	}
	s.invalidateChannels()
	return result, nil
}

func (s *Service) migrateChannelCredentials(w http.ResponseWriter, r *http.Request) {
	account, ok := r.Context().Value(accountContextKey{}).(accountContext)
	if !ok || account.role != "admin" {
		writeError(w, http.StatusForbidden, "forbidden", "administrator role required")
		return
	}
	if s.cfg.ChannelCredentialStorage != "encrypted" {
		writeError(w, http.StatusConflict, "encrypted_storage_required", "set CHANNEL_CREDENTIAL_STORAGE=encrypted before migrating channel credentials")
		return
	}
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if decode(r, &in) != nil || !in.Confirm {
		writeError(w, http.StatusBadRequest, "confirmation_required", "confirm=true is required to migrate channel credentials")
		return
	}
	result, err := s.migrateChannelCredentialRows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_migration_failed", "channel credential migration failed; no credentials were changed")
		return
	}
	s.audit(r, "channel.credentials_migrated", "channel", "all", map[string]any{"channels_migrated": result.ChannelsMigrated, "keys_migrated": result.KeysMigrated, "already_encrypted": result.AlreadyEncrypted})
	writeJSON(w, http.StatusOK, result)
}
