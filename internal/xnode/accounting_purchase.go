package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

// ClosePurchasePeriods closes only the purchase's periods, including its
// resets. A successor already serving on this identity remains untouched.
func (s *Service) ClosePurchasePeriods(ctx context.Context, user model.User, purchase model.Purchase) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.purchaseOwner(ctx, user, purchase)
	if err != nil {
		return err
	}
	periods, err := s.termPeriods(ctx, identity, purchase.ValidFrom, purchase.ValidUntil, purchase.ID)
	if err != nil {
		return err
	}
	for _, period := range periods {
		if err := s.closePeriod(ctx, period, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PurchaseUsageSnapshot(ctx context.Context, user model.User, purchase model.Purchase, end time.Time) (usageaccounting.UsageSnapshot, error) {
	identity, err := s.purchaseOwner(ctx, user, purchase)
	if err != nil {
		return usageaccounting.UsageSnapshot{}, err
	}
	if end.After(purchase.ValidUntil) {
		return usageaccounting.UsageSnapshot{}, ErrInvalid
	}
	return s.accountingSnapshot(ctx, identity, purchase.ValidFrom, end, purchase.ID)
}

func (s *Service) purchaseOwner(ctx context.Context, user model.User, purchase model.Purchase) (Identity, error) {
	if purchase.ID == "" || user.ID == "" || purchase.UserID != user.ID {
		return Identity{}, ErrInvalid
	}
	identity, err := s.IdentityForUser(ctx, user.ID)
	if err == nil && identity.UserID != user.ID {
		return Identity{}, ErrConflict
	}
	return identity, err
}
