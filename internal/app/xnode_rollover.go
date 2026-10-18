package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Native rollover uses panel ownership and explicit purchase references. Legacy
// IDs are consulted only while the configured migration bridge is active.
func (a proxyProvider) rolloverIdentity(ctx context.Context, user model.User) (xnode.Identity, bool, error) {
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		return identity, true, nil
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return identity, false, err
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return identity, false, err
	}
	if !bridge || user.RemnaUserID == nil {
		return identity, false, xnode.ErrUnavailable
	}
	return identity, false, nil
}

func (a proxyProvider) QuiescePurchaseForRollover(ctx context.Context, user model.User, purchase model.Purchase) error {
	identity, native, err := a.rolloverIdentity(ctx, user)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.QuiesceForRollover(ctx, *user.RemnaUserID)
	}
	if err := a.requireBridgeSettlement(ctx, identity); err != nil {
		return err
	}
	if err := a.reconcileBridgeUsage(ctx, identity); err != nil {
		return err
	}
	return a.native.QuiescePurchaseForRollover(ctx, user, purchase)
}

func (a proxyProvider) PurchaseUsageSnapshotForRollover(ctx context.Context, user model.User, purchase model.Purchase, end time.Time) (rollover.UsageSnapshot, error) {
	identity, native, err := a.rolloverIdentity(ctx, user)
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	if !native {
		return a.remnaAdapter.UsageSnapshotForRollover(ctx, *user.RemnaUserID, purchase.ValidFrom, end)
	}
	if err := a.requireBridgeSettlement(ctx, identity); err != nil {
		return rollover.UsageSnapshot{}, err
	}
	if err := a.reconcileBridgeUsage(ctx, identity); err != nil {
		return rollover.UsageSnapshot{}, err
	}
	return a.native.PurchaseUsageSnapshotForRollover(ctx, user, purchase, end)
}
