package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
)

func (a proxyProvider) LoadUnifiedSubscription(ctx context.Context, user model.User, links bool) (connectivity.MemberSubscription, error) {
	result, err := a.LoadSubscriptionForMember(ctx, user, nil, links)
	if errors.Is(err, xnode.ErrNotFound) {
		return connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}, nil
	}
	return result, err
}

func (a proxyProvider) LoadSubscriptionForMember(ctx context.Context, user model.User, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		return a.LoadMemberSubscription(ctx, strconv.FormatInt(identity.ProxyUserID, 10), enabled, links)
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return connectivity.MemberSubscription{}, err
	}
	return connectivity.MemberSubscription{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
}

func (a proxyProvider) LoadMemberSubscription(ctx context.Context, id string, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	identity, err := a.native.Identity(ctx, id)
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	proxies, err := a.native.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	monitored := make(map[string]bool, len(proxies))
	if enabled == nil {
		seen := map[string]bool{}
		for _, proxy := range proxies {
			for _, group := range proxy.GroupIDs {
				if !seen[group] {
					enabled = append(enabled, group)
					seen[group] = true
				}
			}
		}
	}
	for _, proxy := range proxies {
		monitored[proxy.ID] = true
	}
	if links {
		proxies, err = a.native.ExportProxiesForUser(ctx, identity.UserID, proxies)
		if err != nil {
			return connectivity.MemberSubscription{}, err
		}
	}
	result := connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}
	names, err := a.SquadNames(ctx)
	if err != nil {
		return result, err
	}
	result = projectMemberSubscription(proxies, monitored, enabled, names, links, a.native.Now())

	if links {
		link, err := a.native.SubscriptionURL(identity)
		if err != nil {
			return result, err
		}
		result.SubscriptionURL = &link
	}
	return result, nil
}
