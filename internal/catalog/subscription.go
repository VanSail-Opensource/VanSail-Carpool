package catalog

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type subscriptionSource interface {
	LoadMemberSubscription(context.Context, string, []string, bool) (connectivity.MemberSubscription, error)
}
type nativeSubscriptionSource interface {
	LoadSubscriptionForMember(context.Context, model.User, []string, bool) (connectivity.MemberSubscription, error)
}
type subscriptionPreferences interface {
	UserDisabledSquads(context.Context, string) ([]string, error)
}

// SetConnectivity shares retained monitoring outcomes with member projections.
func (s *Service) SetConnectivity(monitor *connectivity.Service) { s.connectivity = monitor }

// Subscription authorizes the local entitlement before loading this user's live links.
func (s *Service) Subscription(ctx context.Context, user model.User, links bool) (connectivity.MemberSubscription, error) {
	now := s.now().UTC()
	result := connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}
	result.Summary = connectivity.UptimeSummary{UptimeTimeline: connectivity.UnknownTimeline(now)}
	active, _, err := s.repository.ActiveAndQueuedPurchases(ctx, user.ID, now)
	if err != nil {
		return result, err
	}
	if !activeSubscription(active, now) {
		return result, nil
	}
	result.ActiveCombo, result.Summary.ActiveCombo = true, true
	nativeSource, native := s.provider.(nativeSubscriptionSource)
	if !native && user.ProxyUserID == nil {
		result.Summary.ErrorCode = "CONNECTIVITY_ACCOUNT_UNAVAILABLE"
		return result, nil
	}
	preferences, ok := s.repository.(subscriptionPreferences)
	if !ok {
		return result, &connectivity.CodeError{Code: "CONNECTIVITY_UNAVAILABLE"}
	}
	disabled, err := preferences.UserDisabledSquads(ctx, user.ID)
	if err != nil {
		return result, err
	}
	enabled := purchaseops.EnabledSquads(active.SquadUUIDs, disabled)
	source, ok := s.provider.(subscriptionSource)
	if !ok && !native {
		return result, &connectivity.CodeError{Code: "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE"}
	}
	if native {
		result, err = nativeSource.LoadSubscriptionForMember(ctx, user, enabled, links)
	} else {
		result, err = source.LoadMemberSubscription(ctx, *user.ProxyUserID, enabled, links)
	}
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	// A slow queued read must not release credentials after an entitlement change.
	current, _, err := s.repository.ActiveAndQueuedPurchases(ctx, user.ID, s.now().UTC())
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	currentDisabled, err := preferences.UserDisabledSquads(ctx, user.ID)
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	if !activeSubscription(current, s.now().UTC()) || current.ID != active.ID || !sameSquads(enabled, purchaseops.EnabledSquads(current.SquadUUIDs, currentDisabled)) {
		return connectivity.MemberSubscription{}, &connectivity.CodeError{Code: "CONNECTIVITY_INTERRUPTED"}
	}
	result.ActiveCombo = true
	s.connectivity.ProjectMember(ctx, &result, s.now().UTC())
	return result, nil
}

func activeSubscription(purchase *model.Purchase, now time.Time) bool {
	return purchase != nil && purchase.Status == "active" && !now.Before(purchase.ValidFrom) && now.Before(purchase.ValidUntil)
}

func sameSquads(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
