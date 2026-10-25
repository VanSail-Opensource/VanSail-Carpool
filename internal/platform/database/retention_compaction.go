package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CompactAndPrune aggregates required facts and removes expired detail in one transaction.
func (s *Store) retryAttempt_CompactAndPrune(ctx context.Context, cutoff7Days, cutoff24Hours, now time.Time) (map[string]int64, error) {
	if !cutoff7Days.Before(now) || !cutoff24Hours.Before(now) {
		return nil, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin retention compaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	counts := make(map[string]int64)
	if err := pruneAbandonedImportsTx(ctx, tx, now, counts); err != nil {
		return nil, err
	}
	if err := pruneNativeMaintenanceTx(ctx, tx, cutoff7Days, counts); err != nil {
		return nil, err
	}
	if err := pruneNodeCompensationEventsTx(ctx, tx, now, counts); err != nil {
		return nil, err
	}
	if err := pruneProviderOperationsTx(ctx, tx, cutoff24Hours, now, counts); err != nil {
		return nil, err
	}
	if err := pruneFailedSynchronizationJobsTx(ctx, tx, now, counts); err != nil {
		return nil, err
	}
	if err := compactPaymentsTx(ctx, tx, cutoff7Days, now, counts); err != nil {
		return nil, err
	}
	if err := compactActivityTx(ctx, tx, cutoff7Days, cutoff24Hours, now, counts); err != nil {
		return nil, err
	}
	if err := compactPurchasesTx(ctx, tx, cutoff7Days, now, counts); err != nil {
		return nil, err
	}
	if counts["connection_scans"], err = deleteCount(ctx, tx,
		`DELETE FROM connection_scans WHERE expires_at<=? AND status IN ('succeeded','failed')`, stamp(now)); err != nil {
		return nil, fmt.Errorf("prune expired connection scans: %w", err)
	}
	if err := pruneHousekeepingTx(ctx, tx, cutoff7Days, cutoff24Hours, now, counts); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit retention compaction: %w", err)
	}
	return counts, nil
}

func deleteCount(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
