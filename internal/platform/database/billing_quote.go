package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// QuotePurchase performs the same catalog, coupon, and effective-date checks as
// checkout without changing balances, coupon grants, or entitlement state.
func (s *Store) QuotePurchase(ctx context.Context, input PurchaseInput, now time.Time) (model.PurchaseQuote, error) {
	input.IdempotencyKey = "quote"
	if _, err := normalizeAndFingerprintPurchase(&input); err != nil {
		return model.PurchaseQuote{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.PurchaseQuote{}, fmt.Errorf("begin purchase quote: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, _, err := quotePurchaseTx(ctx, tx, input, now.UTC())
	return quote, err
}
