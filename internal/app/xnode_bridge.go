package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
	"time"
)

// reconcileBridgeUsage imports only still legacy-owned node counters. Native
// batches are accounted separately and never inferred from Remnawave totals.
func (a proxyProvider) reconcileBridgeUsage(ctx context.Context, identity xnode.Identity) error {
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge || identity.LegacyUserID == "" {
		return err
	}
	id, err := strconv.ParseInt(identity.LegacyUserID, 10, 64)
	if err != nil {
		return err
	}
	user, err := remnaCall(ctx, a.remnaAdapter, func(ctx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(ctx, id)
	})
	if err != nil {
		return err
	}
	if user == nil || user.ID != id {
		return xnode.ErrConflict
	}
	return a.native.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: user.UserTraffic.UsedTrafficBytes, LifetimeBytes: user.UserTraffic.LifetimeUsedTrafficBytes, LastResetAt: user.LastTrafficResetAt, SyncedAt: time.Now().UTC()})
}
func (a proxyProvider) resetBridge(ctx context.Context, identity xnode.Identity) error {
	id := strconv.FormatInt(identity.ProxyUserID, 10)
	// Revocation on both providers precedes the final counter reconciliation.
	if err := a.remnaAdapter.RemoveEntitlement(ctx, identity.LegacyUserID); err != nil {
		return err
	}
	if err := a.reconcileBridgeUsage(ctx, identity); err != nil {
		return err
	}
	if err := a.remnaAdapter.ResetTraffic(ctx, identity.LegacyUserID); err != nil {
		return err
	}
	// Open the new native period before importing the source's new reset anchor.
	// Otherwise subsequent source deltas remain attributed to the old period.
	if err := a.native.ResetTraffic(ctx, id); err != nil {
		return err
	}
	if err := a.reconcileBridgeUsage(ctx, identity); err != nil {
		return err
	}
	purchase, err := a.store.DesiredEntitlement(ctx, identity.UserID, time.Now())
	if err != nil {
		return err
	}
	if purchase == nil {
		return nil
	}
	if err := a.native.ApplyPurchase(ctx, id, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
		return err
	}
	return a.bridgeEntitlement(ctx, identity, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.SquadUUIDs, purchase.ValidUntil)
}
func (a proxyProvider) requireBridgeSettlement(ctx context.Context, identity xnode.Identity) error {
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge || identity.LegacyUserID == "" {
		return err
	}
	// The legacy API has no accounting barrier. A verified final export is needed
	// before money can be settled from a mixed-provider term.
	state, err := a.store.XNodeMigration(ctx)
	if err != nil {
		return err
	}
	if state.Phase != "complete" {
		return xnode.ErrUnavailable
	}
	for _, node := range state.Nodes {
		if node.Phase != "native" {
			return xnode.ErrUnavailable
		}
	}
	return nil
}
