package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// MigrateGlobalFirewallDetection runs after vault initialization and preserves
// the encryption and audit ownership of an existing global configuration.
func (s *Store) MigrateGlobalFirewallDetection(ctx context.Context, vault *secret.Vault) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	var encrypted bool
	err = tx.QueryRowContext(ctx, `SELECT value,encrypted FROM settings WHERE key=?`, xnode.GlobalSettingKey).Scan(&raw, &encrypted)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if encrypted {
		raw, err = vault.Decrypt(xnode.GlobalSettingKey, raw)
		if err != nil {
			return err
		}
	}
	var fields map[string]json.RawMessage
	if raw != "" {
		if err = json.Unmarshal([]byte(raw), &fields); err != nil {
			return err
		}
	}
	if len(fields["firewallDetection"]) > 0 && string(fields["firewallDetection"]) != "null" {
		return nil
	}
	config, err := xnode.DecodeGlobal(raw)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,settings_cipher FROM xnode_nodes ORDER BY id`)
	if err != nil {
		return err
	}
	var policies []wire.FirewallPolicy
	for rows.Next() {
		var id, cipher string
		if err = rows.Scan(&id, &cipher); err != nil {
			rows.Close()
			return err
		}
		plain, e := vault.Decrypt("xnode.node."+id, cipher)
		if e != nil {
			rows.Close()
			return e
		}
		var settings xnode.NodeSettings
		if err = json.Unmarshal([]byte(plain), &settings); err != nil {
			rows.Close()
			return err
		}
		policies = append(policies, settings.Firewall)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	config.FirewallDetection = xnode.MigrateFirewallDetection(policies)
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	sealed, err := vault.Encrypt(xnode.GlobalSettingKey, string(encoded))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES(?,?,1,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,encrypted=1,updated_at=excluded.updated_at`, xnode.GlobalSettingKey, sealed, stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	return tx.Commit()
}
