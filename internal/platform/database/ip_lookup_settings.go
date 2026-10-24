package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SaveIPLookupSettings commits configuration, encrypted secrets, quotas and audit together.
func (s *Store) retryAttempt_SaveIPLookupSettings(ctx context.Context, actor string, config iplookup.Config, secrets map[string]string, quotas map[string]*int) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	for id, quota := range quotas {
		if quota != nil && (*quota < 0 || *quota > 1000000) {
			return &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_CONFIG"}
		}
		result, err := tx.ExecContext(ctx, `UPDATE combos SET ip_lookup_quota=?,updated_at=? WHERE id=?`, quota, stamp(now), id)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return ErrNotFound
		}
	}
	for id, value := range secrets {
		if err := putIPSettingTx(ctx, tx, iplookup.CredentialKey(id), value, true, actor, now); err != nil {
			return err
		}
	}
	if config.Enabled {
		var missing int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM combos WHERE ip_lookup_quota IS NULL`).Scan(&missing); err != nil {
			return err
		}
		if missing != 0 {
			return &iplookup.CodeError{Code: "IP_LOOKUP_CONFIGURATION_REQUIRED"}
		}
		for _, p := range config.Providers {
			if !p.Enabled {
				continue
			}
			var value string
			if err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=? AND encrypted=1`, iplookup.CredentialKey(p.ID)).Scan(&value); err == sql.ErrNoRows || (err == nil && value == "") {
				return &iplookup.CodeError{Code: "IP_LOOKUP_CREDENTIAL_REQUIRED"}
			} else if err != nil {
				return err
			}
		}
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := putIPSettingTx(ctx, tx, iplookup.SettingKey, string(raw), false, actor, now); err != nil {
		return err
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,?,'ip_lookup.settings','settings',?,?,?)`, id, actor, iplookup.SettingKey, string(raw), stamp(now))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func putIPSettingTx(ctx context.Context, tx *sql.Tx, key, value string, encrypted bool, actor string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by) VALUES(?,?,?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,encrypted=excluded.encrypted,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, key, value, boolInt(encrypted), stamp(now), actor)
	return err
}

func saveComboIPQuotaTx(ctx context.Context, tx *sql.Tx, input ComboInput, creating bool) error {
	if input.IPLookupQuota != nil && (*input.IPLookupQuota < 0 || *input.IPLookupQuota > 1000000) {
		return &iplookup.CodeError{Code: "IP_LOOKUP_INVALID_CONFIG"}
	}
	if creating && input.IPLookupQuota == nil {
		var raw string
		if err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, iplookup.SettingKey).Scan(&raw); err != nil && err != sql.ErrNoRows {
			return err
		}
		config, err := iplookup.DecodeConfig(raw)
		if err != nil {
			return err
		}
		if config.Enabled {
			return &iplookup.CodeError{Code: "IP_LOOKUP_CONFIGURATION_REQUIRED"}
		}
	}
	// Creation sets its quota after the combo row has been inserted.
	if !creating && input.IPLookupQuota != nil {
		_, err := tx.ExecContext(ctx, `UPDATE combos SET ip_lookup_quota=? WHERE id=?`, *input.IPLookupQuota, input.ID)
		return err
	}
	return nil
}
