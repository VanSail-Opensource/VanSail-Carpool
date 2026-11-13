package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
)

func (a proxyProvider) LoadUnifiedSubscription(ctx context.Context, user model.User, links bool) (subscription.MemberSubscription, error) {
	return a.readAuthenticatedSubscription(ctx, user, func() (subscription.MemberSubscription, error) {
		identity, err := a.native.IdentityForUser(ctx, user.ID)
		if errors.Is(err, xnode.ErrNotFound) {
			return subscription.MemberSubscription{Hosts: []subscription.SubscriptionHost{}}, nil
		}
		if err != nil {
			return subscription.MemberSubscription{}, err
		}
		return a.LoadMemberSubscription(ctx, strconv.FormatInt(identity.ProxyUserID, 10), nil, links)
	})
}

func (a proxyProvider) LoadSubscriptionForMember(ctx context.Context, user model.User, enabled []string, links bool) (subscription.MemberSubscription, error) {
	return a.readAuthenticatedSubscription(ctx, user, func() (subscription.MemberSubscription, error) {
		identity, err := a.native.IdentityForUser(ctx, user.ID)
		if err == nil {
			return a.LoadMemberSubscription(ctx, strconv.FormatInt(identity.ProxyUserID, 10), enabled, links)
		}
		if !errors.Is(err, xnode.ErrNotFound) {
			return subscription.MemberSubscription{}, err
		}
		return subscription.MemberSubscription{}, xnode.ErrUnavailable
	})
}

func (a proxyProvider) LoadMemberSubscription(ctx context.Context, id string, enabled []string, links bool) (subscription.MemberSubscription, error) {
	identity, err := a.native.Identity(ctx, id)
	if err != nil {
		return subscription.MemberSubscription{}, err
	}
	return xnode.ReadSubscription(ctx, a.native, identity, func(current xnode.Identity) (subscription.MemberSubscription, error) {
		return a.readMemberSubscription(ctx, current, enabled, links)
	})
}

func (a proxyProvider) readMemberSubscription(ctx context.Context, identity xnode.Identity, enabled []string, links bool) (subscription.MemberSubscription, error) {
	readiness, err := a.native.SubscriptionReadiness(ctx, identity)
	if err != nil {
		return subscription.MemberSubscription{}, err
	}
	links = links && readiness.Ready
	proxies, err := a.native.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return subscription.MemberSubscription{}, err
	}
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
	if links {
		proxies, err = a.native.ExportProxiesForUser(ctx, identity.UserID, proxies)
		if err != nil {
			return subscription.MemberSubscription{}, err
		}
	}
	result := subscription.MemberSubscription{Hosts: []subscription.SubscriptionHost{}}
	result = projectMemberSubscription(proxies, enabled, links)
	result.Ready, result.FirstSyncedAt = readiness.Ready, readiness.FirstSyncedAt

	if links {
		link, err := a.native.SubscriptionURL(identity)
		if err != nil {
			return subscription.MemberSubscription{}, err
		}
		result.SubscriptionURL = &link
	}
	return result, nil
}
