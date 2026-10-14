package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CancelQueuedPurchase refunds a queued purchase only to its owning user.
// Status transition, balance credit, and immutable ledger entry share one
// transaction so a retry cannot produce a partial refund.
func (s *Store) CancelQueuedPurchase(ctx context.Context, userID, purchaseID, reason string, now time.Time) (model.Purchase, error) {
	userID = strings.TrimSpace(userID)
	purchaseID = strings.TrimSpace(purchaseID)
	if userID == "" || purchaseID == "" {
		return model.Purchase{}, ErrConflict
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()

	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, purchaseID, userID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.Status != "queued" {
		return model.Purchase{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=? AND user_id=? AND status='queued'`, stamp(now), purchaseID, userID)
	if err != nil {
		return model.Purchase{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.Purchase{}, rowsErr
	} else if affected != 1 {
		return model.Purchase{}, ErrConflict
	}
	balance, err := adjustBalanceTx(ctx, tx, userID, purchase.PriceTXBMinor, now)
	if err != nil {
		return model.Purchase{}, fmt.Errorf("refund queued purchase: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, userID, purchase.PriceTXBMinor, balance, "purchase_cancellation", purchase.ID, reason, now); err != nil {
		return model.Purchase{}, err
	}
	if err := s.insertQueuedCancellationNoticeTx(ctx, tx, purchase, balance, now); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return s.PurchaseByID(ctx, purchase.ID)
}
