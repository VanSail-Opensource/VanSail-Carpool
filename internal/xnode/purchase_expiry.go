package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type purchaseExpiryRepository interface {
	BeginXNodePurchaseExpiryClosure(context.Context, model.User, model.Purchase, Period, time.Time) (Period, bool, error)
}

// CloseExpiredPurchasePeriods admits expiry only while the captured purchase is
// still due. Its store transaction serializes that decision with term edits.
// Refund, reset and replacement closure use their separate admission rules.
func (s *Service) CloseExpiredPurchasePeriods(ctx context.Context, user model.User, purchase model.Purchase, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.purchaseOwner(ctx, user, purchase)
	if err != nil {
		return err
	}
	repo, ok := s.repo.(purchaseExpiryRepository)
	if !ok {
		return ErrUnavailable
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return err
	}
	for _, period := range periods {
		if period.PurchaseID != purchase.ID {
			continue
		}
		closed, admitted, err := repo.BeginXNodePurchaseExpiryClosure(ctx, user, purchase, period, at)
		if err != nil {
			return err
		}
		if !admitted {
			return nil
		}
		if closed.Status != "closed" {
			if err := s.revokePeriodProbes(ctx, closed.ID); err != nil {
				return err
			}
		}
		if err := s.settlePeriodClosure(ctx, closed, true); err != nil {
			return err
		}
	}
	return nil
}
