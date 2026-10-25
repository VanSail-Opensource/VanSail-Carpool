package catalog

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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
		if squads, labelsErr := s.repository.ListSquadProducts(ctx, false); labelsErr == nil {
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
	s.cacheMu.RLock()
	cached, exists := s.cache[user.ID]
	s.cacheMu.RUnlock()
	if exists && now.Sub(cached.fetchedAt) <= s.cacheTTL {
		dashboard.Statistics = &cached.value.Statistics
		dashboard.SubscriptionURL = &cached.value.SubscriptionURL
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
		dashboard.Statistics = &remote.Statistics
		dashboard.SubscriptionURL = &remote.SubscriptionURL
		s.cacheMu.Lock()
		s.cache[user.ID] = cachedDashboard{value: remote, fetchedAt: now}
		s.cacheMu.Unlock()
		return dashboard, nil
	}
	if !exists {
		dashboard.StatisticsStale = true
		dashboard.StatisticsWarning = "Live usage is temporarily unavailable. Local balance and entitlement data are still current."
		return dashboard, nil
	}
	dashboard.Statistics = &cached.value.Statistics
	dashboard.SubscriptionURL = &cached.value.SubscriptionURL
	dashboard.StatisticsStale = true
	dashboard.StatisticsWarning = "Live usage is unavailable. Showing the latest cached data."
	dashboard.FetchedAt = cached.fetchedAt
	return dashboard, nil
}

// RevokeSubscription rotates the bearer URL and invalidates the process-local cache.
func (s *Service) RevokeSubscription(ctx context.Context, user model.User) (string, error) {
	if user.ProxyUserID == nil {
		return "", database.ErrNotFound
	}
	url, err := s.provider.RevokeSubscription(ctx, *user.ProxyUserID)
	if err != nil {
		return "", err
	}
	s.cacheMu.Lock()
	delete(s.cache, user.ID)
	s.cacheMu.Unlock()
	return url, nil
}

// InvalidateDashboard drops usage cached before an entitlement reset or change.
func (s *Service) InvalidateDashboard(userID string) {
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
}
