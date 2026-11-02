package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Existing false positives are released by their detecting IP. Shared source
// ranges remain compact; exact kernel exclusions protect managed addresses.
func (s *Store) ExcludeManagedFirewallIPs(ctx context.Context, ips []string, now time.Time) error {
	raw, err := json.Marshal(ips)
	if err != nil {
		return err
	}
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var found bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_bans WHERE released_version=0 AND ip IN (SELECT value FROM jsonb_array_elements_text(($1)::jsonb)))`, string(raw)).Scan(&found); err != nil {
			return err
		}
		if !found {
			return nil
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_bans SET released_version=$1,released_at=$2 WHERE released_version=0 AND ip IN (SELECT value FROM jsonb_array_elements_text(($3)::jsonb))`, version, stamp(now), string(raw)); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
}
