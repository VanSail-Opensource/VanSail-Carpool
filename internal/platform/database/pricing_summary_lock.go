package database

import (
	"context"
	"database/sql"
)

// Source writers hold a shared transaction lock. Analytics never queue an
// exclusive lock behind financial transactions that might depend on each other.
func lockPricingSummaryTx(ctx context.Context, tx *sql.Tx, kind string) (bool, error) {
	var acquired bool
	err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))`, "pricing."+kind).Scan(&acquired)
	return acquired, err
}

func lockPricingValidationSourceTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended('pricing.validation',0))`)
	return err
}

func (s *Store) beginPricingSummary(ctx context.Context, kind string) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	acquired, err := lockPricingSummaryTx(ctx, tx, kind)
	if err != nil || !acquired {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}
