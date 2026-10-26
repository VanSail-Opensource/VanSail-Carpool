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
func (a proxyProvider) RemoveEntitlement(ctx context.Context, id string) error {
	return a.native.CloseForReplacement(ctx, id)
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
