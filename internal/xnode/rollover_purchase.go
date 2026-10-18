package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

// QuiescePurchaseForRollover closes only the purchase's periods, including its
// resets. A successor already serving on this identity remains untouched.
func (s *Service) QuiescePurchaseForRollover(ctx context.Context, user model.User, purchase model.Purchase) error {
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
		if err := s.closePeriod(ctx, period); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PurchaseUsageSnapshotForRollover(ctx context.Context, user model.User, purchase model.Purchase, end time.Time) (rollover.UsageSnapshot, error) {
	identity, err := s.purchaseOwner(ctx, user, purchase)
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	if end.After(purchase.ValidUntil) {
		return rollover.UsageSnapshot{}, ErrInvalid
	}
	return s.rolloverSnapshot(ctx, identity, purchase.ValidFrom, end, purchase.ID)
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
