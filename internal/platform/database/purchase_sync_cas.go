package database

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// MarkPurchaseSyncResultIfUnchanged completes only the current native target.
// Every outbox retry reloads that target; a stale result cannot relabel a later
// edit, cancellation, lifecycle or already-completed activation.
func (s *Store) retryAttempt_MarkPurchaseSyncResultIfUnchanged(ctx context.Context, expected model.Purchase, generation int64, success bool, now time.Time) error {
	if expected.ID == "" || expected.UserID == "" || expected.UpdatedAt.IsZero() || generation < 0 {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockNativeMemberIdentityTx(ctx, tx, expected.UserID, generation, true); err != nil {
		return err
	}
	if err := lockMemberPurchaseTx(ctx, tx, expected.UserID); err != nil {
		return err
	}
	current, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+`
		WHERE purchases.id=$1 AND purchases.user_id=$2 FOR UPDATE OF purchases`, expected.ID, expected.UserID))
	if errors.Is(err, ErrNotFound) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	current.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, current.ID)
	if err != nil {
		return err
	}
	if !current.UpdatedAt.Equal(expected.UpdatedAt) || current.Status != expected.Status ||
		!samePurchaseActivationMaterial(current, expected) || now.Before(current.ValidFrom) || !now.Before(current.ValidUntil) {
		return ErrConflict
	}
	switch current.Status {
	case "active":
		return nil // Existing activation evidence and allowance remain unchanged.
	case "failed":
		if !success {
			return nil
		}
	case "activating":
	default:
		return ErrConflict
	}
	changed, err := s.markPurchaseSyncResultTx(ctx, tx, current.ID, success, now)
	if err != nil {
		return err
	}
	if !changed {
		return ErrConflict
	}
	return tx.Commit()
}

func samePurchaseActivationMaterial(a, b model.Purchase) bool {
	return a.ID == b.ID && a.UserID == b.UserID && a.ComboID == b.ComboID &&
		a.ValidFrom.Equal(b.ValidFrom) && a.ValidUntil.Equal(b.ValidUntil) &&
		a.TrafficLimitBytes == b.TrafficLimitBytes && a.ResetStrategy == b.ResetStrategy &&
		slices.Equal(uniqueSorted(a.SquadUUIDs), uniqueSorted(b.SquadUUIDs))
}
