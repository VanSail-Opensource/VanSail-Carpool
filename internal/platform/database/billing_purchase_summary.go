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
		if (purchase.Status == "active" || purchase.Status == "activating") && !now.Before(purchase.ValidFrom) && now.Before(purchase.ValidUntil) && (active == nil || moreRecentPurchase(purchase, *active)) {
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

// Whole-second outer bounds retain indexed candidate filtering without deciding
// eligibility lexically: RFC3339Nano omits zero fractions and trims trailing zeros.
func purchaseWindowCandidateBounds(now time.Time) (lower, upper string) {
	second := now.UTC().Truncate(time.Second)
	return stamp(second.Add(-time.Second)), stamp(second.Add(time.Second))
}

func moreRecentPurchase(candidate, current model.Purchase) bool {
	return candidate.ValidFrom.After(current.ValidFrom) ||
		candidate.ValidFrom.Equal(current.ValidFrom) && (candidate.CreatedAt.After(current.CreatedAt) ||
			candidate.CreatedAt.Equal(current.CreatedAt) && candidate.ID > current.ID)
}
