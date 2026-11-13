package catalog

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type memberDashboardReader interface {
	DashboardForMember(context.Context, model.User) (RemoteDashboard, error)
}

// Dashboard prefers fresh upstream data and falls back to a clearly marked cached snapshot.
func (s *Service) Dashboard(ctx context.Context, user model.User) (model.Dashboard, error) {
	now := s.now().UTC()
	balance, err := s.repository.Balance(ctx, user.ID)
	if err != nil {
		return model.Dashboard{}, err
	}
	active, queued, err := s.repository.ActiveAndQueuedPurchases(ctx, user.ID, now)
	if err != nil {
		return model.Dashboard{}, err
	}
	dashboard := model.Dashboard{User: user, Balance: balance, ActivePurchase: active, QueuedPurchase: queued, FetchedAt: now}
	// Home needs entitlement labels, not the full purchasable Combo catalog.
	// Labels are optional presentation data, so failures do not hide finances.
	if active != nil {
		if squads, labelsErr := s.repository.ListSquadProductOverrides(ctx, false); labelsErr == nil {
			names := make(map[string]string, len(squads))
			for _, squad := range squads {
				names[squad.GroupID] = squad.Name
			}
			for _, id := range active.SquadUUIDs {
				if name := names[id]; name != "" {
					dashboard.ActiveSquadNames = append(dashboard.ActiveSquadNames, name)
				}
			}
		}
	}
	if repository, ok := s.repository.(automaticRenewalFailureRepository); ok {
		failure, failureErr := repository.AutoRenewalFailure(ctx, user.ID)
		if failureErr != nil {
			return model.Dashboard{}, failureErr
		}
		dashboard.AutoRenewalFailure = failure
	}
	memberRemote, native := s.provider.(memberDashboardReader)
	if !native && user.ProxyUserID == nil {
		return dashboard, nil
	}
	cached, exists, epoch := s.dashboardSnapshot(user)
	if exists && now.Sub(cached.fetchedAt) <= s.cacheTTL {
		dashboard.Statistics = &cached.value.Statistics
		dashboard.SubscriptionURL = &cached.value.SubscriptionURL
		dashboard.SubscriptionReady = cached.value.SubscriptionURL != ""
		dashboard.FetchedAt = cached.fetchedAt
		return dashboard, nil
	}
	var remote RemoteDashboard
	var remoteErr error
	if native {
		remote, remoteErr = memberRemote.DashboardForMember(ctx, user)
	} else {
		remote, remoteErr = s.provider.Dashboard(ctx, *user.ProxyUserID)
	}
	if remoteErr == nil {
		if !s.cacheDashboard(user, remote, now, epoch) {
			dashboard.StatisticsStale = true
			dashboard.StatisticsWarning = "Account or subscription state changed. Refresh to load current usage."
			return dashboard, nil
		}
		dashboard.Statistics = &remote.Statistics
		dashboard.SubscriptionURL = &remote.SubscriptionURL
		dashboard.SubscriptionReady = remote.SubscriptionURL != ""
		return dashboard, nil
	}
	s.cacheMu.RLock()
	exists = exists && s.cacheEpoch == epoch
	s.cacheMu.RUnlock()
	if !exists {
		dashboard.StatisticsStale = true
		dashboard.StatisticsWarning = "Live usage is temporarily unavailable. Local balance and entitlement data are still current."
		return dashboard, nil
	}
	dashboard.Statistics = &cached.value.Statistics
	dashboard.SubscriptionURL = &cached.value.SubscriptionURL
	dashboard.SubscriptionReady = cached.value.SubscriptionURL != ""
	dashboard.StatisticsStale = true
	dashboard.StatisticsWarning = "Live usage is unavailable. Showing the latest cached data."
	dashboard.FetchedAt = cached.fetchedAt
	return dashboard, nil
}

// InvalidateDashboard drops usage cached before an entitlement reset or change.
func (s *Service) InvalidateDashboard(userID string) {
	s.cacheMu.Lock()
	s.cacheBytes -= s.cache[userID].bytes
	delete(s.cache, userID)
	s.cacheEpoch++
	s.cacheMu.Unlock()
}
