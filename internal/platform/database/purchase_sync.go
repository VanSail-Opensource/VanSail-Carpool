package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

// MarkPurchaseSyncResult advances an activating purchase after upstream work.
func (s *Store) retryAttempt_MarkPurchaseSyncResult(ctx context.Context, purchaseID string, success bool, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := s.markPurchaseSyncResultTx(ctx, tx, purchaseID, success, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) markPurchaseSyncResultTx(ctx context.Context, tx *sql.Tx, purchaseID string, success bool, now time.Time) (bool, error) {
	status := "failed"
	predicate := "status='activating'"
	if success {
		status = "active"
		predicate = "status IN ('activating','failed')"
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status=$1,updated_at=$2 WHERE id=$3 AND `+predicate, status, stamp(now), purchaseID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if success && affected == 1 {
		if _, err := tx.ExecContext(ctx, `INSERT   INTO ip_lookup_allowances(purchase_id,total)
			SELECT purchases.id,COALESCE(combos.ip_lookup_quota,0) FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.id=$1 ON CONFLICT DO NOTHING `, purchaseID); err != nil {
			return false, err
		}
		if err := s.insertActivationNotificationTx(ctx, tx, purchaseID, now); err != nil {
			return false, err
		}
	}
	return affected == 1, nil
}

// PurchaseTrafficResetPhase returns the durable phase of a new-term reset.
func (s *Store) PurchaseTrafficResetPhase(ctx context.Context, purchaseID string) (string, error) {
	var phase string
	err := s.db.QueryRowContext(ctx, `SELECT traffic_reset_phase FROM purchases WHERE id=$1`, purchaseID).Scan(&phase)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load purchase traffic reset phase: %w", err)
	}
	return phase, nil
}

// AdvancePurchaseTrafficReset atomically records a completed external phase.
// Repeating the same transition is accepted so a DB response loss is harmless.
func (s *Store) AdvancePurchaseTrafficReset(ctx context.Context, purchaseID, from, to string, now time.Time) error {
	valid := (from == "pending" && to == "quiesced") || (from == "quiesced" && to == "reset")
	if !valid {
		return ErrConflict
	}
	result, err := s.db.ExecContext(ctx, `UPDATE purchases SET traffic_reset_phase=$1,updated_at=$2
		WHERE id=$3 AND status IN ('activating','failed') AND traffic_reset_phase=$4`, to, stamp(now), purchaseID, from)
	if err != nil {
		return fmt.Errorf("advance purchase traffic reset: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect purchase traffic reset transition: %w", err)
	}
	if affected == 1 {
		return nil
	}
	phase, loadErr := s.PurchaseTrafficResetPhase(ctx, purchaseID)
	if loadErr != nil {
		return loadErr
	}
	if phase == to {
		return nil
	}
	return ErrConflict
}

// UserForPurchase returns the local user owning a purchase.
func (s *Store) UserForPurchase(ctx context.Context, purchaseID string) (model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` JOIN purchases ON purchases.user_id=users.id WHERE purchases.id=$1`, purchaseID))
}

// DesiredEntitlement returns the currently effective purchase, if any.
func (s *Store) DesiredEntitlement(ctx context.Context, userID string, now time.Time) (*model.Purchase, error) {
	selected, err := activePurchaseAt(ctx, s.db, userID, now)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	selected.SquadUUIDs, err = s.purchaseSquads(ctx, selected.ID)
	return &selected, err
}
