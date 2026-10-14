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
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.ApplyEntitlement(ctx, id, limit, strategy, groups, until)
	}
	if err = a.native.ApplyPurchase(ctx, id, limit, strategy, until); err != nil {
		return err
	}
	if err = a.bridgeEntitlement(ctx, identity, limit, strategy, groups, until); err != nil {
		return err
	}
	return nil
}
func (a proxyProvider) RemoveEntitlement(ctx context.Context, id string) error {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.RemoveEntitlement(ctx, id)
	}
	if err = a.native.CloseForReplacement(ctx, id); err != nil {
		return err
	}
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge || identity.LegacyUserID == "" {
		return err
	}
	return a.remnaAdapter.RemoveEntitlement(ctx, identity.LegacyUserID)
}
func (a proxyProvider) ResetTraffic(ctx context.Context, id string) error {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.ResetTraffic(ctx, id)
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return err
	}
	// A cross-provider reset must reconcile the legacy baseline before reopening.
	if bridge && identity.LegacyUserID != "" {
		return a.resetBridge(ctx, identity)
	}
	return a.native.ResetTraffic(ctx, id)
}
func (a proxyProvider) QuiesceForRollover(ctx context.Context, id string) error {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.QuiesceForRollover(ctx, id)
	}
	if err = a.requireBridgeSettlement(ctx, identity); err != nil {
		return err
	}
	return a.native.Quiesce(ctx, id)
}
func (a proxyProvider) UsageSnapshotForRollover(ctx context.Context, id string, start, end time.Time) (rollover.UsageSnapshot, error) {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	if !native {
		return a.remnaAdapter.UsageSnapshotForRollover(ctx, id, start, end)
	}
	if err = a.requireBridgeSettlement(ctx, identity); err != nil {
		return rollover.UsageSnapshot{}, err
	}
	if err = a.reconcileBridgeUsage(ctx, identity); err != nil {
		return rollover.UsageSnapshot{}, err
	}
	return a.native.UsageSnapshotForRollover(ctx, id, start, end)
}
func (a proxyProvider) MemberOperationState(ctx context.Context, id string) (purchaseops.RemoteState, error) {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return purchaseops.RemoteState{}, err
	}
	if !native {
		return a.remnaAdapter.MemberOperationState(ctx, id)
	}
	if err = a.requireBridgeSettlement(ctx, identity); err != nil {
		return purchaseops.RemoteState{}, err
	}
	if err = a.reconcileBridgeUsage(ctx, identity); err != nil {
		return purchaseops.RemoteState{}, err
	}
	return a.native.MemberOperationState(ctx, id)
}
func (a proxyProvider) QuiesceMemberOperation(ctx context.Context, id string) error {
	return a.QuiesceForRollover(ctx, id)
}
func (a proxyProvider) RestoreMemberOperation(ctx context.Context, id string, purchase model.Purchase) error {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.RestoreMemberOperation(ctx, id, purchase)
	}
	if err = a.native.RestorePurchase(ctx, id, purchase); err != nil {
		return err
	}
	return a.bridgeEntitlement(ctx, identity, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.SquadUUIDs, purchase.ValidUntil)
}
func (a proxyProvider) DefinitiveMutationFailure(err error) bool {
	return errors.Is(err, xnode.ErrInvalid) || errors.Is(err, xnode.ErrConflict)
}
func (a proxyProvider) SetAdminUserDisabled(ctx context.Context, id string, disabled bool) error {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !native {
		return a.remnaAdapter.SetAdminUserDisabled(ctx, id, disabled)
	}
	if err = a.store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, disabled); err != nil {
		return err
	}
	if disabled {
		if err = a.native.CloseForReplacement(ctx, id); err != nil {
			return err
		}
	} else {
		purchase, e := a.store.DesiredEntitlement(ctx, identity.UserID, time.Now())
		if e != nil {
			return e
		}
		if purchase != nil {
			if err = a.native.ResumePurchase(ctx, id, *purchase); err != nil {
				return err
			}
		}
	}
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge || identity.LegacyUserID == "" {
		return err
	}
	return a.remnaAdapter.SetAdminUserDisabled(ctx, identity.LegacyUserID, disabled)
}
func (a proxyProvider) VerifyAdminRemoteUser(ctx context.Context, id string) error {
	_, _, err := a.FindUserByID(ctx, id)
	return err
}
