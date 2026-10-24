package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) retryAttempt_DiscardCouponGrant(ctx context.Context, userID, grantID string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin coupon discard: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var existingGrantID string
	err = tx.QueryRowContext(ctx, `SELECT grant_id FROM coupon_grant_discards WHERE grant_id=? AND user_id=?`, grantID, userID).Scan(&existingGrantID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load coupon discard: %w", err)
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=? AND user_id=?`, grantID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load coupon grant for discard: %w", err)
	}
	if status != "active" {
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO coupon_grant_discards(grant_id,user_id,discarded_at) VALUES(?,?,?)`, grantID, userID, stamp(now.UTC())); err != nil {
		if isUniqueConstraint(err) {
			return nil
		}
		return fmt.Errorf("record coupon discard: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit coupon discard: %w", err)
	}
	return nil
}
