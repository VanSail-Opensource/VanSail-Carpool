package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
	"slices"
	"time"
)

func (a proxyProvider) bridgeEntitlement(ctx context.Context, identity xnode.Identity, limit int64, strategy string, groups []string, until time.Time) error {
	if identity.LegacyUserID == "" {
		return nil
	}
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge {
		return err
	}
	if err = a.reconcileBridgeUsage(ctx, identity); err != nil {
		return err
	}
	legacy, err := a.store.XNodeLegacyUsage(ctx, identity.ProxyUserID)
	if err != nil {
		return err
	}
	return a.applyBridgePolicy(ctx, identity, legacy.UsedBytes, nil)
}

// The source counter already includes legacy bytes. Its absolute limit is that
// counter plus the remaining combined allowance, so neither provider receives a
// separate full quota. Refresh after native reports and before rollback reopens.
func (a proxyProvider) bridgePurchase(ctx context.Context, identity xnode.Identity, legacyUsed int64) (*model.Purchase, error) {
	current, err := a.store.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		return nil, err
	}
	if current.UserID == "" {
		return a.bridgeServicePolicy(ctx, current, legacyUsed)
	}
	purchase, err := a.native.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return nil, err
	}
	restricted, err := a.store.XNodeUserRestricted(ctx, identity.UserID)
	if err != nil {
		return nil, err
	}
	disable := purchase == nil || restricted || current.CurrentPeriodID == ""
	var limit int64
	groups := []string{}
	if !disable {
		period, err := a.store.XNodePeriod(ctx, current.CurrentPeriodID)
		if err != nil {
			return nil, err
		}
		disable = period.Status != "open" || period.PurchaseID != purchase.ID || (period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes)
		if !disable {
			if period.LimitBytes > 0 {
				remaining := period.LimitBytes - period.UsedBytes
				if legacyUsed > math.MaxInt64-remaining {
					return nil, xnode.ErrInvalid
				}
				limit = legacyUsed + remaining
			}
			groups, err = a.bridgeGroups(ctx, purchase.SquadUUIDs)
			if err != nil {
				return nil, err
			}
			groups, err = a.store.EnabledSquadsForUser(ctx, identity.UserID, groups)
			if err != nil {
				return nil, err
			}
			disable = len(groups) == 0
		}
	}
	if disable {
		return nil, nil
	}
	next := *purchase
	next.TrafficLimitBytes, next.SquadUUIDs = limit, groups
	return &next, nil
}

func (a proxyProvider) applyBridgePolicy(ctx context.Context, identity xnode.Identity, legacyUsed int64, remote *remnawave.User) error {
	purchase, err := a.bridgePurchase(ctx, identity, legacyUsed)
	if err != nil {
		return err
	}
	if purchase == nil {
		if remote != nil && remote.Status == remnawave.UserStatusDisabled && len(remote.ActiveInternalSquads) == 0 {
			return nil
		}
		return a.remnaAdapter.RemoveEntitlement(ctx, identity.LegacyUserID)
	}
	if remote != nil && remote.Status == remnawave.UserStatusActive && remote.TrafficLimitBytes == purchase.TrafficLimitBytes && string(remote.TrafficLimitStrategy) == purchase.ResetStrategy && remote.ExpireAt.Equal(purchase.ValidUntil) {
		existing := []string{}
		for _, group := range remote.ActiveInternalSquads {
			existing = append(existing, group.UUID)
		}
		slices.Sort(existing)
		wanted := slices.Clone(purchase.SquadUUIDs)
		slices.Sort(wanted)
		if slices.Equal(existing, wanted) {
			return nil
		}
	}
	return a.remnaAdapter.ApplyEntitlement(ctx, identity.LegacyUserID, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.SquadUUIDs, purchase.ValidUntil)
}

func (a proxyProvider) bridgeGroups(ctx context.Context, requested []string) ([]string, error) {
	state, err := a.store.XNodeMigration(ctx)
	if err != nil {
		return nil, err
	}
	if len(state.Nodes) == 0 {
		return requested, nil
	}
	legacyNodes := map[string]bool{}
	for _, node := range state.Nodes {
		legacyNodes[node.NodeID] = node.Phase != "native"
	}
	groups, err := a.store.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(requested, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			if legacyNodes[binding.NodeID] {
				result = append(result, group.ID)
				break
			}
		}
	}
	return result, nil
}
