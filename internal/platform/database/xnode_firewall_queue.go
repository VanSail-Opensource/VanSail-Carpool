package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func queueFirewallTx(ctx context.Context, tx *sql.Tx, node string, now time.Time) error {
	return queueFirewallMembersTx(ctx, tx, node, now)
}

func queueFirewallFleetTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	return queueFirewallMembersTx(ctx, tx, "", now)
}

// Read desired revisions once and queue only the banks that need delivery.
// Existing pending retries and processing leases retain their executor pacing.
// Failed receipts remain historical; the active-payload index coalesces recovery.
func queueFirewallMembersTx(ctx context.Context, tx *sql.Tx, node string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT m.node_id,
 (m.force_sync<>0 OR m.applied<c.version),
 (m.force_sync<>0 OR m.source_applied<c.source_version)
 FROM xnode_firewall_members m CROSS JOIN xnode_firewall_clock c
 WHERE m.active=1 AND ($1='' OR m.node_id=$2)
 AND (m.force_sync<>0 OR m.applied<c.version OR m.source_applied<c.source_version)
 ORDER BY m.node_id`, node, node)
	if err != nil {
		return err
	}
	type intent struct {
		node             string
		dynamic, sources bool
	}
	var intents []intent
	for rows.Next() {
		var v intent
		if err = rows.Scan(&v.node, &v.dynamic, &v.sources); err != nil {
			break
		}
		intents = append(intents, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range intents {
		for _, bank := range []struct {
			name   string
			needed bool
		}{{"dynamic", v.dynamic}, {"sources", v.sources}} {
			if !bank.needed {
				continue
			}
			raw, err := json.Marshal(map[string]string{"nodeId": v.node, "bank": bank.name})
			if err != nil {
				return err
			}
			if err = insertOutboxTx(ctx, tx, xnode.FirewallJob, string(raw), now, now); err != nil {
				return err
			}
		}
	}
	return nil
}
