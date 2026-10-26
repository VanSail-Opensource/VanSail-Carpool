package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

// cadenceReady retries the gap between revoking a due period and atomically
// publishing its successor. Earlier/manual closures and review stay untouched.
func cadenceReady(period Period, now time.Time) bool {
	if period.Strategy == "NO_RESET" || period.Strategy == "PAYG" || period.Status == "review" {
		return false
	}
	next := rollover.CadenceAdvance(period.StartedAt, period.Strategy)
	if next.After(now) {
		return false
	}
	return period.Status == "open" ||
		((period.Status == "closing" || period.Status == "closed") && period.EndedAt != nil && !period.EndedAt.Before(next))
}

func (s *Service) cadenceRecoveryAllowed(ctx context.Context, identity Identity, period Period) (bool, error) {
	if period.Status == "open" {
		return true, nil
	}
	if identity.AccessDisabled {
		return false, nil
	}
	if identity.UserID != "" {
		restricted, err := s.repo.XNodeUserRestricted(ctx, identity.UserID)
		if err != nil || restricted {
			return false, err
		}
		repo, ok := s.repo.(interface {
			ComboControlConflict(context.Context, string) (bool, error)
		})
		if !ok {
			return false, ErrUnavailable
		}
		conflict, err := repo.ComboControlConflict(ctx, identity.UserID)
		if err != nil || conflict {
			return false, err
		}
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return false, err
	}
	for _, other := range periods {
		if other.ID != period.ID && other.PurchaseID == period.PurchaseID && other.Status == "open" {
			return false, ErrConflict // Existing allowance recovery owns this case.
		}
	}
	return true, nil
}
