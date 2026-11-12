package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// QueueXNodeGeoIP coalesces country preparation into the existing durable outbox.
// A fresh job recovers failed delivery without rewriting history or active leases.
func (s *Store) QueueXNodeGeoIP(ctx context.Context, node string) error {
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		raw, err := json.Marshal(map[string]string{"nodeId": node, "bank": "geoip"})
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		return insertOutboxTx(ctx, tx, xnode.FirewallJob, string(raw), now, now)
	})
}
