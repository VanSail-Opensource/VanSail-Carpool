package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func (s *Service) PurchaseOperationState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	if purchase == "" {
		return purchaseops.RemoteState{}, ErrInvalid
	}
	return s.operationState(ctx, id, purchase)
}

func (s *Service) ResetPurchaseTraffic(ctx context.Context, id, purchase string) error {
	if purchase == "" {
		return ErrInvalid
	}
	return s.resetTraffic(ctx, id, purchase)
}

func (s *Service) CheckPurchaseOperation(ctx context.Context, id, purchase string) error {
	if purchase == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	if identity.CurrentPeriodID == "" {
		return ErrConflict
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	return s.checkPurchaseOperation(ctx, identity, period, purchase)
}

func (s *Service) QuiescePurchaseOperation(ctx context.Context, id, purchase string) error {
	if purchase == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	if identity.CurrentPeriodID == "" {
		return ErrConflict
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	if err := s.checkPurchaseOperation(ctx, identity, period, purchase); err != nil {
		return err
	}
	return s.closePeriod(ctx, period)
}

func (s *Service) checkPurchaseOperation(ctx context.Context, identity Identity, period Period, purchase string) error {
	if purchase == "" {
		return nil
	} // legacy unscoped compatibility
	if period.PurchaseID != purchase || period.ProxyUserID != identity.ProxyUserID {
		return ErrConflict
	}
	current, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if current == nil || current.ID != purchase {
		return ErrConflict
	}
	return nil
}
