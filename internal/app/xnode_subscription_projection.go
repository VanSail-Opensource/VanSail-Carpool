package app

import (
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Project only hosts whose squad remains enabled for this owner.
func projectMemberSubscription(proxies []subscription.Proxy, enabled []string, links bool) subscription.MemberSubscription {
	result := subscription.MemberSubscription{Hosts: []subscription.SubscriptionHost{}}
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
		host := subscription.SubscriptionHost{UUID: proxy.ID, Name: proxy.Name, CountryCodes: proxy.Countries, SquadUUIDs: groups}
		host.PublishedName, host.NameOverride, host.PriceLabel = proxy.PublishedName, proxy.NameOverride, proxy.PriceLabel
		host.CustomRoute, host.SplitXHTTP = proxy.CustomRoute, proxy.SplitXHTTP
		if links {
			link, err := proxy.NativeLink()
			if err != nil {
				host.LinkErrorCode = "SUBSCRIPTION_LINK_UNAVAILABLE"
			} else {
				host.Link = &link
			}
		}
		result.Hosts = append(result.Hosts, host)
	}
	return result
}
