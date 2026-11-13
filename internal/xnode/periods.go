package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

// ApplyPurchase is idempotent for an existing term. A replacement fences the
// predecessor immediately and retains unfinished journals for later settlement.
func (s *Service) ApplyPurchase(ctx context.Context, id string, limit int64, strategy string, expires time.Time) error {
	s.mu.Lock()
	target, err := s.applyPurchaseLocked(ctx, id, limit, strategy, expires, nil, -1)
	s.mu.Unlock()
	if err != nil || target == nil {
		return err
	}
	_, err = s.finishIdentityClosure(ctx, *target)
	return err
}

func (s *Service) applyPurchaseLocked(ctx context.Context, id string, limit int64, strategy string, expires time.Time, expected *model.Purchase, generation int64) (*identityClosure, error) {
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return nil, err
	}
	purchase, err := s.requestedPurchase(ctx, identity.UserID, limit, strategy, expires)
	if err != nil {
		return nil, err
	}
	if expected != nil && (purchase == nil || !sameMemberPurchaseTarget(*purchase, *expected)) {
		return nil, ErrConflict
	}
	if purchase == nil {
		target, err := s.captureIdentityClosureLocked(ctx, identity, "", false)
		return &target, err
	}
	if limit != purchase.TrafficLimitBytes || strategy != purchase.ResetStrategy || !expires.Equal(purchase.ValidUntil) {
		return nil, ErrConflict
	}
	var period Period
	if identity.CurrentPeriodID != "" {
		period, err = s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return nil, err
		}
		if period.PurchaseID == purchase.ID {
			if repository, ok := s.repo.(interface {
				RestoreXNodePurchasePeriod(context.Context, Identity, Period, model.Purchase, int64, time.Time) error
			}); ok {
				generation, err = s.purchasePeriodGeneration(ctx, identity, generation)
				if err != nil {
					return nil, err
				}
				return nil, repository.RestoreXNodePurchasePeriod(ctx, identity, period, *purchase, generation, s.now().UTC())
			}
			// Generic fixtures can retain an already-open period; reopening is
			// allowed only through the authoritative atomic storage adapter.
			if period.Status != "open" {
				return nil, ErrAccessAccountingPending
			}
			if err := s.normalizePurchaseSquads(ctx, identity, *purchase); err != nil {
				return nil, err
			}
			period.LimitBytes = limit
			period.Strategy = strategy
			return nil, s.repo.SaveXNodePeriod(ctx, period)
		}
	}
	if repository, ok := s.repo.(purchasePeriodAdmissionRepository); ok {
		return nil, s.admitPurchasePeriod(ctx, repository, identity, period, *purchase, generation)
	}
	if period.ID != "" {
		if err = s.closePeriod(ctx, period, true); err != nil {
			return nil, err
		}
	}
	return nil, s.startPeriod(ctx, identity, *purchase, s.now().UTC())
}
func (s *Service) startPeriod(ctx context.Context, identity Identity, purchase model.Purchase, at time.Time, reference ...string) error {
	if err := s.normalizePurchaseSquads(ctx, identity, purchase); err != nil {
		return err
	}
	var id string
	if len(reference) > 0 {
		id = reference[0]
	} else {
		var err error
		id, err = ids.New()
		if err != nil {
			return err
		}
	}
	return s.repo.SaveXNodePeriod(ctx, Period{ID: id, ProxyUserID: identity.ProxyUserID, PurchaseID: purchase.ID, StartedAt: at, LimitBytes: purchase.TrafficLimitBytes, Strategy: purchase.ResetStrategy, LastResetAt: &at, Status: "open"})
}

func (s *Service) normalizePurchaseSquads(ctx context.Context, identity Identity, purchase model.Purchase) error {
	if identity.UserID == "" {
		return nil
	}
	if preferences, ok := s.repo.(interface {
		EnabledSquadsForUser(context.Context, string, []string) ([]string, error)
	}); ok {
		_, err := preferences.EnabledSquadsForUser(ctx, identity.UserID, purchase.SquadUUIDs)
		return err
	}
	return nil
}
