package database

import (
	"context"
	"time"
)

// WaitXNodeRemoval reschedules only the idempotent removal lifecycle. It never
// releases or replays a native mutation command whose outcome is uncertain.
func (s *Store) WaitXNodeRemoval(ctx context.Context, id string, attempt int, code string, now time.Time) error {
	for retry := 0; ; retry++ {
		err := s.waitXNodeRemoval(ctx, id, attempt, code, now)
		if !s.retryTransaction(ctx, err, retry) {
			return err
		}
	}
}

func (s *Store) waitXNodeRemoval(ctx context.Context, id string, attempt int, code string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='queued',attempt_started_at=NULL,error_code=$1,updated_at=$2 WHERE id=$3 AND kind='xnode_command' AND status='processing' AND attempts=$4`, operationCode(code), stamp(now), id, attempt)
	if err != nil {
		return err
	}
	if err = requireAccessRow(result); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,error_code=$1,updated_at=$2 WHERE operation_id=$3 AND status IN ('queued','processing')`, operationCode(code), stamp(now), id); err != nil {
		return err
	}
	return tx.Commit()
}
