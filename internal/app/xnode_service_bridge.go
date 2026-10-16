package app

import (
	"context"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// The existing entitlement adapter consumes a policy-shaped Purchase value.
// This creates no purchase, user, payment, or other financial record.
func (a proxyProvider) bridgeServicePolicy(ctx context.Context, identity xnode.Identity, legacyUsed int64) (*model.Purchase, error) {
	access, err := a.native.ServiceAccess(ctx, identity)
	if err != nil {
		return nil, err
	}
	if access == nil {
		return nil, xnode.ErrConflict
	}
	if access.Disabled || !access.ExpiresAt.After(a.native.Now()) || identity.AccessDisabled || identity.CurrentPeriodID == "" {
		return nil, nil
	}
	period, err := a.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return nil, err
	}
	if period.Status != "open" || period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes {
		return nil, nil
	}
	limit := int64(0)
	if period.LimitBytes > 0 {
		remaining := period.LimitBytes - period.UsedBytes
		if legacyUsed > math.MaxInt64-remaining {
			return nil, xnode.ErrInvalid
		}
		limit = legacyUsed + remaining
	}
	groups, err := a.bridgeGroups(ctx, access.GroupIDs)
	if err != nil || len(groups) == 0 {
		return nil, err
	}
	return &model.Purchase{TrafficLimitBytes: limit, ResetStrategy: access.Strategy, ValidUntil: access.ExpiresAt, SquadUUIDs: groups}, nil
}
