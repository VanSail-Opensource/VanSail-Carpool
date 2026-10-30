package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// DeferXNodeCommand releases only a definitely rejected attempt. The durable
// claim still owns the node; uncertain/executed commands never enter this path.
func (s *Store) DeferXNodeCommand(ctx context.Context, id string, attempt int, at time.Time) error {
	for retry := 0; ; retry++ {
		err := s.deferXNodeCommand(ctx, id, attempt, at)
		if !s.retryTransaction(ctx, err, retry) {
			return err
		}
	}
}

func (s *Store) deferXNodeCommand(ctx context.Context, id string, attempt int, at time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE provider_operations SET status='queued',attempts=MAX(0,attempts-1),attempt_started_at=NULL,error_code='XNODE_COMMAND_BUSY',updated_at=? WHERE id=? AND kind=? AND status='processing' AND attempts=?`, stamp(at), id, xnode.OperationKind, attempt)
	if err != nil {
		return err
	}
	if err = requireAccessRow(result); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='queued',attempt_started_at=NULL,error_code='XNODE_COMMAND_BUSY',updated_at=? WHERE operation_id=? AND status='processing'`, stamp(at), id); err != nil {
		return err
	}
	return tx.Commit()
}
