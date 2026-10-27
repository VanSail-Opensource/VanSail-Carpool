package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PurchaseReplay resolves a committed request before live catalog or quote checks.
func (s *Store) PurchaseReplay(ctx context.Context, input PurchaseInput) (model.Purchase, bool, error) {
	fingerprint, err := normalizeAndFingerprintPurchase(&input)
	if err != nil {
		return model.Purchase{}, false, err
	}
	var id, saved string
	err = s.db.QueryRowContext(ctx, `SELECT id,request_fingerprint FROM purchases WHERE user_id=? AND idempotency_key=?`, input.UserID, input.IdempotencyKey).Scan(&id, &saved)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Purchase{}, false, nil
	}
	if err != nil {
		return model.Purchase{}, false, err
	}
	if saved != fingerprint {
		return model.Purchase{}, true, ErrConflict
	}
	p, err := s.PurchaseByID(ctx, id)
	return p, true, err
}
