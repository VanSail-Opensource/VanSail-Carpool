package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) Quiesce(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	return s.closeIdentityPeriod(ctx, identity)
}

// CloseForReplacement retains unavailable old journals without delaying a new
// independently paid replacement's hosts or accounting period.
func (s *Service) CloseForReplacement(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	if identity.CurrentPeriodID == "" {
		return nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	return s.closePeriod(ctx, period, true)
}
func (s *Service) ResetTraffic(ctx context.Context, id string) error {
	return s.resetTraffic(ctx, id, "")
}

func (s *Service) resetTraffic(ctx context.Context, id, expectedPurchase string, operation ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	if len(operation) > 0 {
		applied, err := s.resetOperationCompleted(ctx, identity, expectedPurchase, operation[0])
		if err != nil {
			return err
		}
		if applied {
			return nil
		}
	}
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if expectedPurchase != "" && (purchase == nil || purchase.ID != expectedPurchase || identity.CurrentPeriodID == "") {
		return ErrConflict
	}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return err
		}
		if expectedPurchase != "" && (period.PurchaseID != expectedPurchase || period.ProxyUserID != identity.ProxyUserID) {
			return ErrConflict
		}
		if err = s.closePeriod(ctx, period, true); err != nil {
			return err
		}
	}
	purchase, err = s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if expectedPurchase != "" && (purchase == nil || purchase.ID != expectedPurchase) {
		return ErrConflict
	}
	if purchase == nil {
		return nil
	}
	return s.startPeriod(ctx, identity, *purchase, s.now().UTC(), operation...)
}

// RestorePurchase resumes the same accounting period after a refused refund.
func (s *Service) RestorePurchase(ctx context.Context, id string, purchase model.Purchase) error {
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
	if period.PurchaseID != purchase.ID || period.Status != "closed" {
		return ErrConflict
	}
	return s.repo.ReopenXNodePeriod(ctx, period.ID)
}

// ResumePurchase removes an administrative restriction without replacing the
// paid period or requiring offline participants to finish financial settlement.
func (s *Service) ResumePurchase(ctx context.Context, id string, purchase model.Purchase) error {
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
	if period.PurchaseID != purchase.ID || period.Status == "review" {
		return ErrConflict
	}
	if period.Status == "open" {
		return nil
	}
	return s.repo.ReopenXNodePeriod(ctx, period.ID)
}
