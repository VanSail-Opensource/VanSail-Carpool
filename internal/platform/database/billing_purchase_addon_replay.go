package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PurchaseAddonReplay returns an already charged selection before live catalog
// validation. Retrying a lost response must survive subsequent catalog changes.
func (s *Store) PurchaseAddonReplay(ctx context.Context, input PurchaseAddonInput) (model.Purchase, bool, error) {
	input.AddonSquadIDs = append([]string(nil), input.AddonSquadIDs...)
	fingerprint, err := normalizeAndFingerprintPurchaseAddons(&input)
	if err != nil {
		return model.Purchase{}, false, err
	}
	var saved string
	err = s.db.QueryRowContext(ctx, `SELECT adjustment.request_fingerprint FROM purchase_addon_adjustments adjustment JOIN purchases purchase ON purchase.id=adjustment.purchase_id WHERE purchase.user_id=? AND adjustment.purchase_id=? AND adjustment.idempotency_key=?`, input.UserID, input.PurchaseID, input.IdempotencyKey).Scan(&saved)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Purchase{}, false, nil
	}
	if err != nil {
		return model.Purchase{}, false, err
	}
	if saved != fingerprint {
		return model.Purchase{}, true, ErrConflict
	}
	purchase, err := s.PurchaseByID(ctx, input.PurchaseID)
	if err == nil && purchase.UserID != input.UserID {
		return model.Purchase{}, true, ErrNotFound
	}
	return purchase, true, err
}
