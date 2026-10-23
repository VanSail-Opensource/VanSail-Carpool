package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (a proxyProvider) ApplyEntitlement(ctx context.Context, id string, limit int64, strategy string, groups []string, until time.Time) error {
	return a.native.ApplyPurchase(ctx, id, limit, strategy, until)
}
func (a proxyProvider) RemoveEntitlement(ctx context.Context, id string) error {
	return a.native.CloseForReplacement(ctx, id)
}
func (a proxyProvider) ResetTraffic(ctx context.Context, id string) error {
	return a.native.ResetTraffic(ctx, id)
}
func (a proxyProvider) QuiesceForRollover(ctx context.Context, id string) error {
	return a.native.Quiesce(ctx, id)
}
func (a proxyProvider) QuiesceRolloverPeriod(ctx context.Context, id string, start, end time.Time) error {
	return a.native.QuiesceRolloverPeriod(ctx, id, start, end)
}
func (a proxyProvider) UsageSnapshotForRollover(ctx context.Context, id string, start, end time.Time) (rollover.UsageSnapshot, error) {
	return a.native.UsageSnapshotForRollover(ctx, id, start, end)
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
func (a proxyProvider) SetAdminUserDisabled(ctx context.Context, id string, disabled bool) error {
	identity, err := a.native.Identity(ctx, id)
	if err != nil {
		return err
	}
	if err = a.store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, disabled); err != nil {
		return err
	}
	if disabled {
		return a.native.CloseForReplacement(ctx, id)
	}
	purchase, err := a.store.DesiredEntitlement(ctx, identity.UserID, time.Now())
	if err != nil || purchase == nil {
		return err
	}
	return a.native.ResumePurchase(ctx, id, *purchase)
}
