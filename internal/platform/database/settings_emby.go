package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const EmbyURLSetting = "emby.base_url"
const EmbyTokenSetting = "emby.api_token"

// EmbyConnectionRecord keeps the two existing encrypted settings in one snapshot.
type EmbyConnectionRecord struct {
	BaseURL Setting
	Token   Setting
}

// Version is an opaque fingerprint of the stored pair, never of plaintext secrets.
func (r EmbyConnectionRecord) Version() string {
	encoded, _ := json.Marshal([]any{r.BaseURL.Value, r.BaseURL.Encrypted, r.Token.Value, r.Token.Encrypted})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

type embySettingsReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readEmbyConnection(ctx context.Context, reader embySettingsReader) (EmbyConnectionRecord, error) {
	result := EmbyConnectionRecord{BaseURL: Setting{Key: EmbyURLSetting}, Token: Setting{Key: EmbyTokenSetting}}
	rows, err := reader.QueryContext(ctx, `SELECT key,value,encrypted FROM settings WHERE key IN ($1,$2)`, EmbyURLSetting, EmbyTokenSetting)
	if err != nil {
		return result, fmt.Errorf("read Emby connection: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value Setting
		var encrypted int
		if err := rows.Scan(&value.Key, &value.Value, &encrypted); err != nil {
			return result, err
		}
		value.Encrypted = encrypted == 1
		if value.Key == EmbyURLSetting {
			result.BaseURL = value
		} else {
			result.Token = value
		}
	}
	return result, rows.Err()
}

func (s *Store) EmbyConnection(ctx context.Context) (EmbyConnectionRecord, error) {
	return readEmbyConnection(ctx, s.db)
}

// SaveEmbyConnection atomically replaces the pair, including first-time setup.
// The advisory lock also serializes absent rows; CAS rejects stale administrator drafts.
func (s *Store) retryAttempt_SaveEmbyConnection(ctx context.Context, expected string, next EmbyConnectionRecord, actorID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('settings:emby.connection',0))`); err != nil {
		return err
	}
	current, err := readEmbyConnection(ctx, tx)
	if err != nil {
		return err
	}
	if expected == "" || current.Version() != expected {
		return ErrConflict
	}
	if next.BaseURL.Value == "" || next.Token.Value == "" || !next.BaseURL.Encrypted || !next.Token.Encrypted {
		return ErrConflict
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by)
		VALUES($1,$2,1,$5,$6),($3,$4,1,$5,$6) ON CONFLICT(key) DO UPDATE
		SET value=excluded.value,encrypted=1,updated_at=excluded.updated_at,updated_by=excluded.updated_by`,
		EmbyURLSetting, next.BaseURL.Value, EmbyTokenSetting, next.Token.Value, stamp(now), actorID)
	if err != nil {
		return fmt.Errorf("save Emby connection: %w", err)
	}
	if err := catalogAdminAuditTx(ctx, tx, actorID, "emby.connection.update", "setting", "emby.connection", map[string]bool{"secret": true}, now); err != nil {
		return err
	}
	return tx.Commit()
}
