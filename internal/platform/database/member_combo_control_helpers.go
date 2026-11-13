package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type controlQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func comboControlConflictFrom(ctx context.Context, reader controlQueryer, userID string) (bool, error) {
	var conflict bool
	err := reader.QueryRowContext(ctx, `SELECT `+comboControlConflictSQL("$1"), userID).Scan(&conflict)

	return conflict, err
}

// ComboControlConflict prevents switching while another provider mutation owns the member.
func (s *Store) ComboControlConflict(ctx context.Context, userID string) (bool, error) {
	return comboControlConflictFrom(ctx, s.db, userID)
}

func activeControlPurchaseTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, now time.Time) (model.Purchase, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2`, purchaseID, userID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.Status != "active" || now.Before(purchase.ValidFrom) || !now.Before(purchase.ValidUntil) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	var other bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=$1 AND id<>$2 AND status IN ('active','activating') AND `+purchaseTimestampOrderSQL("valid_until")+`>$3)`, userID, purchaseID, purchaseTimestampOrder(now)).Scan(&other)
	if err != nil {
		return model.Purchase{}, err
	}
	if other {
		return model.Purchase{}, ErrConflict
	}
	return purchase, nil
}

func auditComboControlTx(ctx context.Context, tx *sql.Tx, userID, action, purchaseID string, details map[string]any, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	body, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &userID, action, "purchase", purchaseID, string(body), now)
}

func queuedControlPurchaseTx(ctx context.Context, tx *sql.Tx, userID, queuedID string) (model.Purchase, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.user_id=$1 AND purchases.status='queued'
 ORDER BY `+purchaseTimestampOrderSQL("purchases.valid_from")+`,`+purchaseTimestampOrderSQL("purchases.created_at")+`,purchases.id LIMIT 1`, userID))
	if errors.Is(err, ErrNotFound) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.ID != queuedID || !purchase.ValidUntil.After(purchase.ValidFrom) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	return purchase, nil
}
