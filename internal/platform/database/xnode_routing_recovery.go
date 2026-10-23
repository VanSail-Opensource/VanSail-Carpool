package database

import (
	"context"
	"encoding/json"
	"time"
)

// A reconnect is new external evidence after bounded queue retries. Resume the
// same durable operation rather than creating a competing retry scheduler.
func (s *Store) WakeXNodeRouting(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=0,available_at=?,updated_at=? WHERE kind='provider_operation' AND (status='failed' OR status='pending' AND attempts>0) AND json_extract(payload,'$.operationId') IN(SELECT p.operation_id FROM xnode_routing_plans p JOIN provider_operations o ON o.id=p.operation_id WHERE p.phase IN('revoking','applying','enabling') AND o.status IN('queued','processing'))`, stamp(now), stamp(now))
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT p.operation_id FROM xnode_routing_plans p JOIN provider_operations o ON o.id=p.operation_id WHERE p.phase IN('revoking','applying','enabling') AND o.status IN('queued','processing') AND o.kind IN('xnode_routing','xnode_diy') AND NOT EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='provider_operation' AND json_extract(j.payload,'$.operationId')=p.operation_id)`)
	if err != nil {
		return err
	}
	var missing []string
	for rows.Next() {
		var operation string
		if err = rows.Scan(&operation); err != nil {
			rows.Close()
			return err
		}
		missing = append(missing, operation)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, operation := range missing {
		raw, err := json.Marshal(map[string]string{"operationId": operation})
		if err != nil {
			return err
		}
		if err = insertOutboxTx(ctx, tx, "provider_operation", string(raw), now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
