package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"time"
)

// MemberRefundState retains the strict migration accounting fence for financial
// decisions while allowing ordinary reset reconciliation to use its own period.
func (a proxyProvider) MemberRefundState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	if !native {
		state, err := a.remnaAdapter.MemberOperationState(ctx, id)
		if err != nil {
			return state, err
		}
		term, err := a.store.PurchaseByID(ctx, purchase)
		if err != nil {
			return state, err
		}
		until := time.Now().UTC()
		if until.After(term.ValidUntil) {
			until = term.ValidUntil
		}
		snapshot, err := a.remnaAdapter.UsageSnapshotForRollover(ctx, id, term.ValidFrom, until)
		if err != nil {
			return state, err
		}
		if !snapshot.NodeSeriesAvailable {
			return state, rollover.ErrPerNodeUsageUnavailable
		}
		if snapshot.WeightedUsedBytes > state.UsedTrafficBytes {
			state.UsedTrafficBytes = snapshot.WeightedUsedBytes
		}
		return state, nil
	}
	if err = a.requireBridgeSettlement(ctx, identity); err != nil {
		return purchaseops.RemoteState{}, err
	}
	if err = a.reconcileBridgeUsage(ctx, identity); err != nil {
		return purchaseops.RemoteState{}, err
	}
	return a.native.MemberRefundState(ctx, id, purchase)
}
