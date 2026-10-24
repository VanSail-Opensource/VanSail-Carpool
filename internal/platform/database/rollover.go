package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	rolloverpkg "github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"time"
)

func (s *Store) RolloverByPurchase(ctx context.Context, purchaseID string) (model.PurchaseRollover, error) {
	return scanRollover(s.db.QueryRowContext(ctx, rolloverSelect+` WHERE purchase_id=?`, purchaseID))
}

// MarkRolloverProcessing records a successful upstream quiesce. Replays are
// harmless because no traffic reset occurs in this phase.

func (s *Store) MarkRolloverProcessing(ctx context.Context, purchaseID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE purchase_rollovers SET status='processing',attempts=attempts+1,updated_at=? WHERE purchase_id=? AND status='pending'`, stamp(now), purchaseID)
	if err != nil {
		return fmt.Errorf("mark rollover processing: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 1 {
		return nil
	}
	rollover, loadErr := s.RolloverByPurchase(ctx, purchaseID)
	if loadErr != nil {
		return loadErr
	}
	if rollover.Status == "processing" || rollover.Status == "credited" || rollover.Status == "zero" || rollover.Status == "exception" {
		return nil
	}
	return ErrConflict
}

// FinalizeRollover atomically records authoritative traffic inputs, applies one
// credit, expires the old term, and only then queues the next entitlement.

func (s *Store) FinalizeRollover(ctx context.Context, purchaseID string, limitBytes, usedBytes int64, exceptionCode string, now time.Time) (model.PurchaseRollover, error) {
	remaining := limitBytes - usedBytes
	if remaining < 0 {
		remaining = 0
	}
	summary := model.RolloverUsageSummary{AllocatedBytes: limitBytes, UsedBytes: usedBytes, EligibleUnusedBytes: remaining, AlgorithmVersion: "legacy-total-v1"}
	summary.EligibleUnusedBytes = rolloverpkg.EligibleUnused(limitBytes, usedBytes, 0)
	return s.finalizeRolloverUsage(ctx, purchaseID, summary, exceptionCode, now)
}

// FinalizeRolloverUsage commits a cadence-aware aggregate and never persists
// the raw provider series.

func (s *Store) FinalizeRolloverUsage(ctx context.Context, purchaseID string, summary model.RolloverUsageSummary, exceptionCode string, now time.Time) (model.PurchaseRollover, error) {
	return s.finalizeRolloverUsage(ctx, purchaseID, summary, exceptionCode, now)
}

func (s *Store) retryAttempt_finalizeRolloverUsage(ctx context.Context, purchaseID string, summary model.RolloverUsageSummary, exceptionCode string, now time.Time) (model.PurchaseRollover, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PurchaseRollover{}, err
	}
	defer func() { _ = tx.Rollback() }()
	rollover, err := scanRollover(tx.QueryRowContext(ctx, rolloverSelect+` WHERE purchase_id=?`, purchaseID))
	if err != nil {
		return model.PurchaseRollover{}, err
	}
	if rollover.Status == "credited" || rollover.Status == "zero" || rollover.Status == "exception" {
		return rollover, nil
	}
	if rollover.Status != "processing" {
		return model.PurchaseRollover{}, ErrConflict
	}
	if summary.AllocatedBytes < 0 || summary.UsedBytes < 0 || summary.EligibleUnusedBytes < 0 || summary.EligibleUnusedBytes > summary.AllocatedBytes {
		return model.PurchaseRollover{}, errors.New("rollover traffic inputs must be non-negative")
	}
	remaining := summary.AllocatedBytes - summary.UsedBytes
	if remaining < 0 {
		remaining = 0
	}
	eligible := rolloverEligibleForSummary(summary, rollover.MinimumRemainingBPS)
	credit := int64(0)
	status := "zero"
	if exceptionCode != "" {
		status = "exception"
	} else if summary.AllocatedBytes > 0 && eligible > 0 {
		credit = rolloverpkg.CreditMinor(rollover.NetPaidTXBMinor, eligible, summary.AllocatedBytes)
		if credit > 0 {
			status = "credited"
		}
	}
	var userID string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM purchases WHERE id=?`, purchaseID).Scan(&userID); err != nil {
		return model.PurchaseRollover{}, err
	}
	if credit > 0 {
		balance, balanceErr := changeBalanceTx(ctx, tx, userID, credit, now)
		if balanceErr != nil {
			return model.PurchaseRollover{}, balanceErr
		}
		if _, err := insertLedgerTx(ctx, tx, userID, credit, balance, "rollover_credit", purchaseID, "unused traffic rollover", now); err != nil {
			return model.PurchaseRollover{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status=?,traffic_limit_bytes=?,allocated_traffic_bytes=?,used_traffic_bytes=?,eligible_unused_bytes=?,remaining_traffic_bytes=?,credited_txb_minor=?,exception_code=?,algorithm_version=?,updated_at=?,completed_at=? WHERE purchase_id=? AND status='processing'`,
		status, rollover.TrafficLimitBytes, summary.AllocatedBytes, summary.UsedBytes, eligible, remaining, credit, exceptionCode, summary.AlgorithmVersion, stamp(now), stamp(now), purchaseID)
	if err != nil {
		return model.PurchaseRollover{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.PurchaseRollover{}, rowsErr
	} else if affected != 1 {
		return model.PurchaseRollover{}, ErrConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=? WHERE id=? AND status IN ('active','activating')`, stamp(now), purchaseID)
	if err != nil {
		return model.PurchaseRollover{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.PurchaseRollover{}, rowsErr
	} else if affected != 1 {
		return model.PurchaseRollover{}, ErrConflict
	}
	var nextID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM purchases WHERE user_id=? AND status='queued' AND valid_from<=? AND valid_until>? ORDER BY valid_from,id LIMIT 1`, userID, stamp(now), stamp(now)).Scan(&nextID)
	if err == nil {
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='activating',updated_at=? WHERE id=? AND status='queued'`, stamp(now), nextID)
		if err != nil {
			return model.PurchaseRollover{}, err
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return model.PurchaseRollover{}, rowsErr
		} else if affected != 1 {
			return model.PurchaseRollover{}, ErrConflict
		}
		if err := applyPendingExtensionsToActivationTx(ctx, tx, nextID, now); err != nil {
			return model.PurchaseRollover{}, err
		}
		if err := insertOutboxTx(ctx, tx, "xnode_apply_entitlement", `{"purchaseId":"`+nextID+`"}`, now, now); err != nil {
			return model.PurchaseRollover{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
			return model.PurchaseRollover{}, err
		}
		if err := s.insertExpirationNotificationTx(ctx, tx, purchaseID, userSyncGate(userID), now); err != nil {
			return model.PurchaseRollover{}, err
		}
	} else {
		return model.PurchaseRollover{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PurchaseRollover{}, err
	}
	return s.RolloverByPurchase(ctx, purchaseID)
}

const rolloverSelect = `SELECT purchase_id,status,traffic_limit_bytes,allocated_traffic_bytes,used_traffic_bytes,eligible_unused_bytes,remaining_traffic_bytes,minimum_remaining_bps,net_paid_txb_minor,credited_txb_minor,exception_code,attempts,created_at,updated_at,completed_at,algorithm_version FROM purchase_rollovers`
