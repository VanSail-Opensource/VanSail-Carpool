package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"slices"
	"strconv"
)

func (a proxyProvider) LoadSubscriptionForMember(ctx context.Context, user model.User, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		return a.LoadMemberSubscription(ctx, strconv.FormatInt(identity.ProxyUserID, 10), enabled, links)
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return connectivity.MemberSubscription{}, err
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	if bridge && user.RemnaUserID != nil {
		return a.remnaAdapter.LoadMemberSubscription(ctx, *user.RemnaUserID, enabled, links)
	}
	return connectivity.MemberSubscription{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
}

func (a proxyProvider) LoadMemberSubscription(ctx context.Context, id string, enabled []string, links bool) (connectivity.MemberSubscription, error) {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return connectivity.MemberSubscription{}, err
	}
	if !native {
		return a.remnaAdapter.LoadMemberSubscription(ctx, id, enabled, links)
	}
	proxies, err := a.native.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return connectivity.MemberSubscription{}, err
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
	squads := map[string]int{}
	for _, proxy := range proxies {
		groups := []string{}
		for _, id := range proxy.GroupIDs {
			if slices.Contains(enabled, id) {
				groups = append(groups, id)
			}
		}
		if len(groups) == 0 {
			continue
		}
		host := connectivity.SubscriptionHost{UUID: proxy.ID, Name: proxy.Name, CountryCodes: proxy.Countries, SquadUUIDs: groups, Timeline: connectivity.UnknownTimeline(a.native.Now())}
		if links {
			link, err := proxy.NativeLink()
			if err != nil {
				host.LinkErrorCode = "CONNECTIVITY_LINK_UNAVAILABLE"
			} else {
				host.Link = &link
			}
		}
		result.Hosts = append(result.Hosts, host)
		for _, id := range groups {
			index, ok := squads[id]
			if !ok {
				index = len(result.Squads)
				squads[id] = index
				result.Squads = append(result.Squads, connectivity.SubscriptionSquad{UUID: id, Name: names[id], HostUUIDs: []string{}, Timeline: connectivity.UnknownTimeline(a.native.Now())})
			}
			result.Squads[index].HostUUIDs = append(result.Squads[index].HostUUIDs, host.UUID)
		}
	}
	if links {
		link, err := a.native.SubscriptionURL(identity)
		if err != nil {
			return result, err
		}
		result.SubscriptionURL = &link
	}
	return result, nil
}
