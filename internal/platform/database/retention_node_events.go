package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Expiration is creation-based for every status, including open and approved events.
// Purchased extensions and the operation's frozen user targets live independently.
func pruneNodeCompensationEventsTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	expired, err := expiredTimestampIDsTx(ctx, tx, `SELECT id,created_at FROM node_compensation_events WHERE created_at<$1`, now.Add(-7*24*time.Hour))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_expired_node_events(id TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	for _, id := range expired {
		if _, err := tx.ExecContext(ctx, `INSERT INTO maintenance_expired_node_events(id) VALUES($1)`, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE node_compensation_node_state SET open_event_id=NULL,updated_at=$1
		WHERE open_event_id IN (SELECT id FROM maintenance_expired_node_events)`, stamp(now)); err != nil {
		return fmt.Errorf("detach expired outage observations: %w", err)
	}
	count, err := deleteCount(ctx, tx, `DELETE FROM node_compensation_events WHERE id IN (SELECT id FROM maintenance_expired_node_events)`)
	if err != nil {
		return fmt.Errorf("prune expired node-down events: %w", err)
	}
	counts["node_compensation_events"] = count
	_, err = tx.ExecContext(ctx, `DROP TABLE maintenance_expired_node_events`)
	return err
}
