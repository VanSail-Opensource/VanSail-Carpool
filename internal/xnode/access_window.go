package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

func (s *Service) requestedPurchase(ctx context.Context, user string, limit int64, strategy string, expires time.Time) (*model.Purchase, error) {
	current, err := s.repo.DesiredEntitlement(ctx, user, s.now())
	if err != nil {
		return nil, err
	}
	if current != nil && current.TrafficLimitBytes == limit && current.ResetStrategy == strategy && current.ValidUntil.Equal(expires) {
		return current, nil
	}
	purchases, err := s.repo.ListPurchases(ctx, user)
	if err != nil {
		return nil, err
	}
	for _, purchase := range purchases {
		if purchase.Status == "queued" && purchase.ValidUntil.Equal(expires) {
			continuity, err := s.repo.ContinuityEntitlement(ctx, purchase.ID, s.now())
			if err != nil {
				return nil, err
			}
			if continuity != nil && continuity.TrafficLimitBytes == limit && continuity.ResetStrategy == strategy {
				return continuity, nil
			}
		}
		if purchase.Status == "failed" && !purchase.ValidFrom.After(s.now()) && purchase.ValidUntil.After(s.now()) && purchase.ValidUntil.Equal(expires) && purchase.TrafficLimitBytes == limit && purchase.ResetStrategy == strategy {
			return &purchase, nil
		}
	}
	if current == nil {
		return nil, nil
	}
	return nil, ErrConflict
}
func (s *Service) continuityUntil(ctx context.Context, purchase model.Purchase) (time.Time, error) {
	until := purchase.ValidUntil
	items, err := s.repo.ListPurchases(ctx, purchase.UserID)
	if err != nil {
		return until, err
	}
	for _, next := range items {
		if next.Status == "queued" && next.ValidFrom.Equal(until) && next.ValidUntil.After(until) {
			until = next.ValidUntil
		}
	}
	return until, nil
}

// EffectivePurchase preserves the already prepared paid successor window while
// the durable transition worker closes the predecessor. It grants no access to
// unpaid registrations, unrelated queued purchases, or expired successors.
func (s *Service) EffectivePurchase(ctx context.Context, user string) (*model.Purchase, error) {
	current, err := s.repo.DesiredEntitlement(ctx, user, s.now())
	if err != nil || current != nil {
		return current, err
	}
	purchases, err := s.repo.ListPurchases(ctx, user)
	if err != nil {
		return nil, err
	}
	for _, successor := range purchases {
		if successor.Status != "queued" || successor.ValidFrom.After(s.now()) || !successor.ValidUntil.After(s.now()) {
			continue
		}
		continuity, err := s.repo.ContinuityEntitlement(ctx, successor.ID, s.now())
		if err != nil {
			return nil, err
		}
		if continuity != nil && continuity.UserID == user {
			return continuity, nil
		}
	}
	return nil, nil
}
