package xnode

import (
	"context"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func (s *Service) MemberOperationState(ctx context.Context, id string) (purchaseops.RemoteState, error) {
	return s.operationState(ctx, id, "")
}

func (s *Service) operationState(ctx context.Context, id, expectedPurchase string) (purchaseops.RemoteState, error) {
	state, _, err := s.operationStateSnapshot(ctx, id, expectedPurchase)
	return state, err
}

func (s *Service) operationStateSnapshot(ctx context.Context, id, expectedPurchase string) (purchaseops.RemoteState, identityClosure, error) {
	target, err := s.captureIdentityClosure(ctx, id, expectedPurchase, true)
	if err != nil {
		return purchaseops.RemoteState{}, target, err
	}
	period := target.period
	// "closing" is not settled: never allow a refund to use an unfinished count.
	// Already-closed and empty snapshots also need a final identity/generation check.
	if period.Status == "closing" || period.Status == "closed" || period.ID == "" {
		period, err = s.finishIdentityClosure(ctx, target)
	}
	if err != nil {
		return purchaseops.RemoteState{}, target, err
	}
	target.period = period
	return purchaseops.RemoteState{UsedTrafficBytes: period.UsedBytes, LastTrafficResetAt: period.LastResetAt, Quiesced: period.Status == "closed" || period.ID == ""}, target, nil
}

// MemberRefundState includes every reset owned by this purchase. Reset
// reconciliation uses MemberOperationState and is independent of older journals.
func (s *Service) MemberRefundState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	if purchase == "" {
		return purchaseops.RemoteState{}, ErrInvalid
	}
	state, target, err := s.operationStateSnapshot(ctx, id, purchase)
	if err != nil {
		return state, err
	}
	identity := target.identity
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return state, err
	}
	var used int64
	found := false
	for _, item := range periods {
		if item.PurchaseID != purchase {
			continue
		}
		if item.ID == identity.CurrentPeriodID {
			found = true
		}
		if state.Quiesced && item.Status != "closed" || item.Status == "closing" || item.Status == "review" || item.UsedBytes < 0 || used > math.MaxInt64-item.UsedBytes {
			return state, ErrUnavailable
		}
		used += item.UsedBytes
	}
	if !found {
		return state, ErrConflict
	}
	state.UsedTrafficBytes = used
	// An account imported after a source reset can have earlier traffic only in
	// its final source history. Any such evidence disqualifies a zero-use refund.
	if used == 0 && identity.LegacyUserID != "" {
		purchases, err := s.repo.ListPurchases(ctx, identity.UserID)
		if err != nil {
			return state, err
		}
		found := false
		for _, item := range purchases {
			if item.ID != purchase {
				continue
			}
			found = true
			history, err := s.repo.XNodeImportedHistory(ctx, identity.ProxyUserID, item.ValidFrom, item.ValidUntil)
			if err != nil {
				return state, err
			}
			for _, sample := range history {
				if sample.WeightedBytes < 0 {
					return state, ErrUnavailable
				}
				if sample.WeightedBytes > 0 {
					state.UsedTrafficBytes = sample.WeightedBytes
					break
				}
			}
			break
		}
		if !found {
			return state, ErrConflict
		}
	}
	if state.Quiesced {
		if _, err = s.checkIdentityClosure(ctx, target); err != nil {
			return purchaseops.RemoteState{}, err
		}
	}
	return state, nil
}
