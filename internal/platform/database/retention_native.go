package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Retain all exact periods, usage, cursors and financial references. Only
// expired service access and multiplier boundaries superseded below an already
// acknowledged cursor are disposable. Each maintenance pass deletes bounded work.
func pruneNativeMaintenanceTx(ctx context.Context, tx *sql.Tx, cutoff time.Time, counts map[string]int64) error {
	var err error
	counts["xnode_service_bindings"], err = deleteCount(ctx, tx, `DELETE FROM xnode_service_bindings WHERE rowid IN
		(SELECT rowid FROM xnode_service_bindings WHERE valid_until<? ORDER BY valid_until LIMIT 1000)`, stamp(cutoff))
	if err != nil {
		return fmt.Errorf("prune expired native service access: %w", err)
	}
	counts["xnode_node_rates"], err = deleteCount(ctx, tx, `DELETE FROM xnode_node_rates WHERE (agent_id,from_sequence) IN
		(SELECT rate.agent_id,rate.from_sequence FROM xnode_node_rates rate JOIN xnode_report_cursors cursor ON cursor.agent_id=rate.agent_id
		WHERE EXISTS (SELECT 1 FROM xnode_node_rates newer WHERE newer.agent_id=rate.agent_id
			AND newer.from_sequence>rate.from_sequence AND newer.from_sequence<=cursor.last_sequence)
		ORDER BY rate.agent_id,rate.from_sequence LIMIT 1000)`)
	if err != nil {
		return fmt.Errorf("prune acknowledged native rate boundaries: %w", err)
	}
	return nil
}
