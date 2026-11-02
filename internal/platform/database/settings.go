package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Setting is a stored application option. Encrypted values remain encrypted at this layer.
type Setting struct {
	Key       string
	Value     string
	Encrypted bool
	UpdatedAt time.Time
}

// PutSetting inserts or replaces a dashboard-managed setting.
func (s *Store) PutSetting(ctx context.Context, key, value string, encrypted bool, actorID *string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(key) DO UPDATE SET value=excluded.value,encrypted=excluded.encrypted,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, key, value, boolInt(encrypted), stamp(time.Now().UTC()), actorID)
	if err != nil {
		return fmt.Errorf("put setting %q: %w", key, err)
	}
	return nil
}

// GetSetting returns one setting.
func (s *Store) GetSetting(ctx context.Context, key string) (Setting, error) {
	var setting Setting
	var encrypted int
	var updated string
	err := s.db.QueryRowContext(ctx, `SELECT key,value,encrypted,updated_at FROM settings WHERE key=$1`, key).Scan(&setting.Key, &setting.Value, &encrypted, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Setting{}, ErrNotFound
		}
		return Setting{}, fmt.Errorf("get setting %q: %w", key, err)
	}
	setting.Encrypted = encrypted == 1
	setting.UpdatedAt, err = parseStamp(updated)
	if err != nil {
		return Setting{}, fmt.Errorf("parse setting timestamp: %w", err)
	}
	return setting, nil
}

// ListSettings returns all stored settings ordered by key.
func (s *Store) ListSettings(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key,value,encrypted,updated_at FROM settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()
	settings := make([]Setting, 0)
	for rows.Next() {
		var setting Setting
		var encrypted int
		var updated string
		if err := rows.Scan(&setting.Key, &setting.Value, &encrypted, &updated); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		setting.Encrypted = encrypted == 1
		setting.UpdatedAt, err = parseStamp(updated)
		if err != nil {
			return nil, fmt.Errorf("parse setting timestamp: %w", err)
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return settings, nil
}
