package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const paygRetentionBatch = 4096

// PrunePAYGTransientRecords expires disposable samples independently of repairs.
// Every batch commits separately; a budget expiry leaves the backlog for next tick.
func (s *Store) PrunePAYGTransientRecords(ctx context.Context, now time.Time) (map[string]int64, error) {
	budget, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	counts := map[string]int64{}
	for budget.Err() == nil {
		batch, err := s.PrunePAYGTransientBatch(budget, now)
		if err != nil {
			if budget.Err() != nil && ctx.Err() == nil {
				return counts, nil
			}
			return counts, err
		}
		for key, count := range batch {
			counts[key] += count
		}
		if batch["xnode_payg_minutes"] < paygRetentionBatch && batch["xnode_payg_windows"] < paygRetentionBatch {
			return counts, nil
		}
	}
	return counts, ctx.Err()
}

// PrunePAYGTransientBatch is one retryable transaction, with no financial writes.
func (s *Store) retryAttempt_PrunePAYGTransientBatch(ctx context.Context, now time.Time) (map[string]int64, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	counts := map[string]int64{}
	if err := prunePAYGTransientTx(ctx, tx, now, counts); err != nil {
		return nil, err
	}
	return counts, tx.Commit()
}

func prunePAYGTransientTx(ctx context.Context, tx *sql.Tx, now time.Time, counts map[string]int64) error {
	var err error
	counts["xnode_payg_minutes"], err = deleteCount(ctx, tx, `DELETE FROM xnode_payg_minutes WHERE (user_id,group_id,minute) IN
		(SELECT user_id,group_id,minute FROM xnode_payg_minutes WHERE minute<? ORDER BY minute LIMIT 4096)`, stamp(now.UTC().Truncate(time.Minute).Add(-xnode.PAYGWindow)))
	if err != nil {
		return fmt.Errorf("prune PAYG minute summaries: %w", err)
	}
	counts["xnode_payg_windows"], err = deleteCount(ctx, tx, `DELETE FROM xnode_payg_windows WHERE (proxy_user_id,through) IN
		(SELECT proxy_user_id,through FROM xnode_payg_windows WHERE through<=? ORDER BY through LIMIT 4096)`, paygWindowStamp(now.Add(-xnode.PAYGWindow)))
	if err != nil {
		return fmt.Errorf("prune PAYG rolling samples: %w", err)
	}
	return nil
}
