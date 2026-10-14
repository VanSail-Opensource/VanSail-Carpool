package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// DueAutoRenewal identifies a currently enabled source term due for renewal.
type DueAutoRenewal struct {
	PurchaseID string
	UserID     string
}

// HasEnabledAutoRenewal reports whether an active entitlement chain blocks catalog checkout.
func (s *Store) HasEnabledAutoRenewal(ctx context.Context, userID string, _ time.Time) (bool, error) {
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases
		WHERE user_id=? AND auto_renew_enabled=1 AND status IN ('active','activating','queued'))`, userID).Scan(&enabled)
	if err != nil {
		return false, fmt.Errorf("check automatic renewal: %w", err)
	}
	return enabled == 1, nil
}

// SetAutoRenewal stores the member's explicit choice for an active, activating,
// or settled expired source term. Callers must validate eligibility first.
func (s *Store) SetAutoRenewal(ctx context.Context, userID, purchaseID string, enabled bool, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic renewal update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=?,auto_renew_failure_reason=CASE WHEN ?=1 THEN '' ELSE auto_renew_failure_reason END,
		auto_renew_failed_at=CASE WHEN ?=1 THEN NULL ELSE auto_renew_failed_at END,updated_at=?
		WHERE id=? AND user_id=? AND status IN ('active','activating','expired')`, boolInt(enabled), boolInt(enabled), boolInt(enabled), stamp(now), purchaseID, userID)
	if err != nil {
		return fmt.Errorf("set automatic renewal: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect automatic renewal update: %w", err)
	}
	if affected == 1 {
		if !enabled {
			if _, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,updated_at=?
				WHERE auto_renew_source_purchase_id=? AND status='queued'`, stamp(now), purchaseID); err != nil {
				return fmt.Errorf("disable queued automatic renewal successor: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit automatic renewal update: %w", err)
		}
		return nil
	}
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("close automatic renewal update: %w", err)
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE id=? AND user_id=?)`, purchaseID, userID).Scan(&exists); err != nil {
		return fmt.Errorf("find automatic renewal purchase: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	return ErrConflict
}

// DueAutoRenewals lists expired terms with a persisted rollover calculation.
func (s *Store) DueAutoRenewals(ctx context.Context, now time.Time) ([]DueAutoRenewal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id FROM purchases WHERE auto_renew_enabled=1
		AND valid_until<=? AND ((status IN ('active','activating')
			AND EXISTS (SELECT 1 FROM purchase_rollovers rollover WHERE rollover.purchase_id=purchases.id AND rollover.status='calculated'))
			OR (status='expired' AND EXISTS (SELECT 1 FROM purchase_rollovers rollover WHERE rollover.purchase_id=purchases.id AND rollover.status='zero')))
		AND NOT EXISTS (SELECT 1 FROM purchases successor WHERE successor.auto_renew_source_purchase_id=purchases.id
			AND successor.status IN ('queued','activating','active'))
		ORDER BY valid_until,id`, stamp(now.UTC()))
	if err != nil {
		return nil, fmt.Errorf("list due automatic renewals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]DueAutoRenewal, 0)
	for rows.Next() {
		var item DueAutoRenewal
		if err := rows.Scan(&item.PurchaseID, &item.UserID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// RenewalAwaitingRollover reports whether a due term's rollover settlement has
// not completed yet. An insufficient-balance renewal should then be retried
// after the rollover credit instead of being permanently disabled.
func (s *Store) RenewalAwaitingRollover(ctx context.Context, purchaseID string) (bool, error) {
	var awaiting int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases purchase WHERE purchase.id=?
		AND (purchase.status IN ('active','activating')
			OR EXISTS (SELECT 1 FROM purchase_rollovers rollover WHERE rollover.purchase_id=purchase.id
				AND rollover.status IN ('pending','processing'))))`, purchaseID).Scan(&awaiting)
	if err != nil {
		return false, fmt.Errorf("check pending rollover settlement: %w", err)
	}
	return awaiting == 1, nil
}

// MarkAutoRenewalFailed disables a due source without charging its owner.
func (s *Store) MarkAutoRenewalFailed(ctx context.Context, purchaseID, reason string, now time.Time) error {
	if reason == "" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic renewal failure: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var userID string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM purchases WHERE id=?`, purchaseID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load automatic renewal failure owner: %w", err)
	}
	settled, err := s.failCalculatedAutoRenewalTx(ctx, tx, purchaseID, userID, reason, now.UTC())
	if err != nil {
		return err
	}
	if !settled {
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=0,auto_renew_failure_reason=?,auto_renew_failed_at=?,updated_at=?
			WHERE id=? AND auto_renew_enabled=1 AND status IN ('active','activating','expired') AND valid_until<=?`, reason, stamp(now), stamp(now), purchaseID, stamp(now.UTC()))
		if err != nil {
			return fmt.Errorf("record automatic renewal failure: %w", err)
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return rowsErr
		} else if affected == 1 {
			if err := s.insertAutoRenewalFailureNoticeTx(ctx, tx, purchaseID, userID, reason, "", now); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit automatic renewal failure: %w", err)
	}
	return nil
}

// AutoRenewalFailure returns the latest member-visible automatic-renewal failure.
func (s *Store) AutoRenewalFailure(ctx context.Context, userID string) (*model.AutoRenewalFailure, error) {
	var failure model.AutoRenewalFailure
	var failedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,auto_renew_failure_reason,auto_renew_failed_at FROM purchases
		WHERE user_id=? AND auto_renew_failure_reason<>'' AND auto_renew_failed_at IS NOT NULL ORDER BY auto_renew_failed_at DESC LIMIT 1`, userID).Scan(&failure.PurchaseID, &failure.Reason, &failedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load automatic renewal failure: %w", err)
	}
	failed, err := parseStamp(failedAt.String)
	if err != nil {
		return nil, err
	}
	failure.FailedAt = failed
	return &failure, nil
}
