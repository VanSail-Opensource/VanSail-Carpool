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
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	if identity.CurrentPeriodID == "" {
		if expectedPurchase != "" {
			return purchaseops.RemoteState{}, ErrConflict
		}
		return purchaseops.RemoteState{Quiesced: true}, nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	if err := s.checkPurchaseOperation(ctx, identity, period, expectedPurchase); err != nil {
		return purchaseops.RemoteState{}, err
	}
	// "closing" is not settled: never allow a refund to use an unfinished count.
	if period.Status == "closing" {
		if err = s.closePeriod(ctx, period); err != nil {
			return purchaseops.RemoteState{}, err
		}
		period, err = s.repo.XNodePeriod(ctx, period.ID)
		if err == nil && period.Status != "closed" {
			return purchaseops.RemoteState{}, ErrUnavailable
		}
	}
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	return purchaseops.RemoteState{UsedTrafficBytes: period.UsedBytes, LastTrafficResetAt: period.LastResetAt, Quiesced: period.Status == "closed"}, nil
}

// MemberRefundState includes every reset owned by this purchase. Reset
// reconciliation uses MemberOperationState and is independent of older journals.
func (s *Service) MemberRefundState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	state, err := s.PurchaseOperationState(ctx, id, purchase)
	if err != nil {
		return state, err
	}
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return state, err
	}
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
		if item.Status == "closing" || item.Status == "review" || item.UsedBytes < 0 || used > math.MaxInt64-item.UsedBytes {
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
	return state, nil
}
