package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ActiveAndQueuedPurchases returns the current home-screen entitlement summary.
func (s *Store) ActiveAndQueuedPurchases(ctx context.Context, userID string, now time.Time) (*model.Purchase, *model.Purchase, error) {
	purchases, err := s.ListPurchases(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	var active, queued *model.Purchase
	for index := range purchases {
		purchase := purchases[index]
		if (purchase.Status == "active" || purchase.Status == "activating") && !now.Before(purchase.ValidFrom) && now.Before(purchase.ValidUntil) && active == nil {
			copy := purchase
			active = &copy
		}
		if purchase.Status == "queued" && now.Before(purchase.ValidUntil) && (queued == nil || purchase.ValidFrom.Before(queued.ValidFrom) || (purchase.ValidFrom.Equal(queued.ValidFrom) && (purchase.CreatedAt.Before(queued.CreatedAt) || purchase.CreatedAt.Equal(queued.CreatedAt) && purchase.ID < queued.ID))) {
			copy := purchase
			queued = &copy
		}
	}
	return active, queued, nil
}
