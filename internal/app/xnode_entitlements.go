package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (a proxyProvider) ApplyEntitlement(ctx context.Context, id string, limit int64, strategy string, groups []string, until time.Time) error {
	return a.native.ApplyPurchase(ctx, id, limit, strategy, until)
}

// ApplyMemberPurchase binds activation to its retained purchase and native owner;
// it never resets whichever account happens to be in a historical proxy link.
func (a proxyProvider) ApplyMemberPurchase(ctx context.Context, user model.User, purchase model.Purchase) error {
	return a.native.ApplyMemberPurchase(ctx, user, purchase)
}

func (a proxyProvider) RemoveEntitlement(ctx context.Context, id string) error {
	return a.native.CloseForReplacement(ctx, id)
}

// SyncMemberEntitlement shares owner-scoped desired-state reconciliation with
// administrative changes. A stale legacy link cannot select another identity.
func (a proxyProvider) SyncMemberEntitlement(ctx context.Context, user model.User, desired *model.Purchase) error {
	return a.SyncAdminUserEntitlement(ctx, user, desired)
}

func (a proxyProvider) ResetTraffic(ctx context.Context, id string) error {
	return a.native.ResetTraffic(ctx, id)
}
func (a proxyProvider) QuiesceForSettlement(ctx context.Context, id string) error {
	return a.native.Quiesce(ctx, id)
}
func (a proxyProvider) CloseAccountingPeriod(ctx context.Context, id string, start, end time.Time) error {
	return a.native.CloseAccountingPeriod(ctx, id, start, end)
}
func (a proxyProvider) UsageSnapshot(ctx context.Context, id string, start, end time.Time) (usageaccounting.UsageSnapshot, error) {
	return a.native.UsageSnapshot(ctx, id, start, end)
}
func (a proxyProvider) MemberOperationState(ctx context.Context, id string) (purchaseops.RemoteState, error) {
	return a.native.MemberOperationState(ctx, id)
}
func (a proxyProvider) QuiesceMemberOperation(ctx context.Context, id string) error {
	return a.native.Quiesce(ctx, id)
}
func (a proxyProvider) RestoreMemberOperation(ctx context.Context, id string, purchase model.Purchase) error {
	return a.native.RestorePurchase(ctx, id, purchase)
}
func (a proxyProvider) DefinitiveMutationFailure(err error) bool {
	return errors.Is(err, xnode.ErrInvalid) || errors.Is(err, xnode.ErrConflict)
}
