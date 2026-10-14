package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/entitlements"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func (a remnaAdapter) ApplyEntitlement(ctx context.Context, remoteID string, trafficLimitBytes int64, resetStrategy string, squadUUIDs []string, expiresAt time.Time) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	status := remnawave.UserStatusActive
	strategy := remnawave.TrafficLimitStrategy(resetStrategy)
	expires := expiresAt.UTC()
	squads := append([]string(nil), squadUUIDs...)
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		if a.squadPreferences != nil {
			effective, err := a.squadPreferences.EnabledSquadsForRemote(callCtx, remoteID, squads)
			if err != nil {
				return err
			}
			squads = effective
		}
		_, callErr := client.UpdateUser(callCtx, remnawave.UpdateUserRequest{
			ID: userID, Status: &status, TrafficLimitBytes: &trafficLimitBytes,
			TrafficLimitStrategy: &strategy, ExpireAt: &expires,
			ActiveInternalSquads: &squads, ClearExternalSquad: true,
		})
		return callErr
	})
}

func (a remnaAdapter) ResetTraffic(ctx context.Context, remoteID string) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.ResetTraffic(callCtx, userID)
		return callErr
	})
}

func (a remnaAdapter) RemoveEntitlement(ctx context.Context, remoteID string) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	status := remnawave.UserStatusDisabled
	limit := int64(0)
	strategy := remnawave.TrafficNoReset
	expires := time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	squads := []string{}
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.UpdateUser(callCtx, remnawave.UpdateUserRequest{
			ID: userID, Status: &status, TrafficLimitBytes: &limit,
			TrafficLimitStrategy: &strategy, ExpireAt: &expires,
			ActiveInternalSquads: &squads, ClearExternalSquad: true,
		})
		return callErr
	})
}

func (a remnaAdapter) QuiesceForRollover(ctx context.Context, remoteID string) error {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	status := remnawave.UserStatusDisabled
	err = remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.UpdateUser(callCtx, remnawave.UpdateUserRequest{ID: userID, Status: &status})
		return callErr
	})
	if remnawave.IsNotFound(err) {
		return rollover.ErrRemoteUserMissing
	}
	return err
}

func (a remnaAdapter) UsageSnapshotForRollover(ctx context.Context, remoteID string, start, end time.Time) (rollover.UsageSnapshot, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(callCtx, userID)
	})
	if remnawave.IsNotFound(err) {
		return rollover.UsageSnapshot{}, rollover.ErrRemoteUserMissing
	}
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	stats, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.UserStats, error) {
		// Remnawave accepts date-only, inclusive ranges. Keep the final date so
		// a newly activated term with equal start and end timestamps remains valid.
		return client.GetUserStats(callCtx, userID, start.UTC(), end.UTC(), rolloverStatsTopNodesLimit)
	})
	if remnawave.IsNotFound(err) {
		return rollover.UsageSnapshot{}, rollover.ErrRemoteUserMissing
	}
	if err != nil {
		return rollover.UsageSnapshot{}, err
	}
	categories, validationErr := validateRolloverStatistics(user, stats, start, end)
	if validationErr != nil {
		return rollover.UsageSnapshot{}, validationErr
	}
	nodeSeries := make([]rollover.NodeUsageSeries, 0, len(stats.Series))
	for _, series := range stats.Series {
		multiplier, multiplierErr := a.nodeMultiplier(ctx, series.UUID)
		if multiplierErr != nil || multiplier <= 0 {
			return rollover.UsageSnapshot{}, rollover.ErrPerNodeUsageUnavailable
		}
		nodeSeries = append(nodeSeries, rollover.NodeUsageSeries{UUID: series.UUID, MultiplierFP: multiplier, Data: append([]int64(nil), series.Data...)})
	}
	daily, weightedTotal := rollover.WeightedNodeUsage(categories, nodeSeries)
	currentUsed := user.UserTraffic.UsedTrafficBytes
	return rollover.UsageSnapshot{
		LimitBytes: user.TrafficLimitBytes, Strategy: string(user.TrafficLimitStrategy),
		LastResetAt: user.LastTrafficResetAt, CurrentUsedBytes: &currentUsed, Daily: daily,
		WeightedUsedBytes: weightedTotal, NodeSeriesAvailable: true,
	}, nil
}

var _ entitlements.RemnawaveClient = remnaAdapter{}
var _ rollover.Remote = remnaAdapter{}
