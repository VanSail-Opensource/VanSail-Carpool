package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	rolloverpkg "github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

// RolloverEligible reports whether expiry may calculate rollover for the term.
func (s *Store) RolloverEligible(ctx context.Context, purchaseID string) (bool, error) {
	var eligible int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases source
		WHERE source.id=? AND source.status IN ('active','activating') AND source.auto_renew_enabled=1
		AND NOT EXISTS (SELECT 1 FROM purchases queued
			WHERE queued.user_id=source.user_id AND queued.status='queued' AND queued.id<>source.id))`, purchaseID).Scan(&eligible)
	if err != nil {
		return false, fmt.Errorf("check rollover eligibility: %w", err)
	}
	return eligible == 1, nil
}

// RecordRolloverCalculation stores the aggregate used to settle automatic renewal.
func (s *Store) RecordRolloverCalculation(ctx context.Context, purchaseID string, summary model.RolloverUsageSummary, now time.Time) (model.PurchaseRollover, error) {
	if err := validateRolloverSummary(summary); err != nil {
		return model.PurchaseRollover{}, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PurchaseRollover{}, fmt.Errorf("begin rollover calculation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rollover, err := scanRollover(tx.QueryRowContext(ctx, rolloverSelect+` WHERE purchase_id=?`, purchaseID))
	if err != nil {
		return model.PurchaseRollover{}, err
	}
	if rollover.Status == "calculated" {
		if err := tx.Rollback(); err != nil {
			return model.PurchaseRollover{}, err
		}
		return rollover, nil
	}
	if rollover.Status != "processing" {
		return model.PurchaseRollover{}, ErrConflict
	}
	summary.EligibleUnusedBytes = rolloverEligibleForSummary(summary, rollover.MinimumRemainingBPS)
	summary.AlgorithmVersion = rolloverpkg.UsageAlgorithmVersion
	remaining := summary.AllocatedBytes - summary.UsedBytes
	result, err := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status='calculated',allocated_traffic_bytes=?,used_traffic_bytes=?,
		eligible_unused_bytes=?,remaining_traffic_bytes=?,credited_txb_minor=0,exception_code='',algorithm_version=?,updated_at=?
		WHERE purchase_id=? AND status='processing'`, summary.AllocatedBytes, summary.UsedBytes, summary.EligibleUnusedBytes,
		remaining, summary.AlgorithmVersion, stamp(now.UTC()), purchaseID)
	if err != nil {
		return model.PurchaseRollover{}, fmt.Errorf("record rollover calculation: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.PurchaseRollover{}, rowsErr
	} else if affected != 1 {
		return model.PurchaseRollover{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return model.PurchaseRollover{}, fmt.Errorf("commit rollover calculation: %w", err)
	}
	return s.RolloverByPurchase(ctx, purchaseID)
}

func rolloverEligibleForSummary(summary model.RolloverUsageSummary, threshold int) int64 {
	return rolloverpkg.EligibleUnused(summary.AllocatedBytes, summary.UsedBytes, threshold)
}

func validateRolloverSummary(summary model.RolloverUsageSummary) error {
	if summary.AllocatedBytes < 0 || summary.UsedBytes < 0 || summary.UsedBytes > summary.AllocatedBytes ||
		summary.EligibleUnusedBytes < 0 || summary.EligibleUnusedBytes > summary.AllocatedBytes {
		return errors.New("rollover traffic inputs must be non-negative")
	}
	return nil
}
