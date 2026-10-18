package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// QueueXNodeGeoIP coalesces country preparation into the existing durable outbox.
func (s *Store) QueueXNodeGeoIP(ctx context.Context, node string) error {
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		raw, err := json.Marshal(map[string]string{"nodeId": node, "bank": "geoip"})
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=0,available_at=?,updated_at=? WHERE kind=? AND payload=? AND status='failed'`, stamp(now), stamp(now), xnode.FirewallJob, string(raw)); err != nil {
			return err
		}
		return insertOutboxTx(ctx, tx, xnode.FirewallJob, string(raw), now, now)
	})
}
