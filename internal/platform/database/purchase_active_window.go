package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// activePurchaseAt shares exact paid-window selection across entitlement reads
// and transactional rewards. SQL bounds only narrow indexed candidates.
func activePurchaseAt(ctx context.Context, reader renewalAdmissionReader, userID string, now time.Time) (model.Purchase, error) {
	lower, upper := purchaseWindowCandidateBounds(now)
	rows, err := reader.QueryContext(ctx, purchaseSelect+` WHERE purchases.user_id=$1
  AND purchases.status IN ('activating','active') AND purchases.valid_from<$2 AND purchases.valid_until>$3`, userID, upper, lower)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = rows.Close() }()
	var selected model.Purchase
	for rows.Next() {
		purchase, err := scanPurchase(rows)
		if err != nil {
			return model.Purchase{}, err
		}
		if now.Before(purchase.ValidFrom) || !now.Before(purchase.ValidUntil) {
			continue
		}
		if selected.ID == "" || moreRecentPurchase(purchase, selected) {
			selected = purchase
		}
	}
	if err := rows.Err(); err != nil {
		return model.Purchase{}, err
	}
	if selected.ID == "" {
		return model.Purchase{}, ErrNotFound
	}
	return selected, nil
}
