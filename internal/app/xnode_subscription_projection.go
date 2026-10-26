package app

import (
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Export-only derivatives retain their links but never enter monitored squads.
// The published inventory is authoritative; names and synthetic ID prefixes are
// not used to decide which hosts have independent monitoring observations.
func projectMemberSubscription(proxies []subscription.Proxy, monitored map[string]bool, enabled []string, names map[string]string, links bool, now time.Time) connectivity.MemberSubscription {
	result := connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}
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
		host := connectivity.SubscriptionHost{UUID: proxy.ID, Name: proxy.Name, CountryCodes: proxy.Countries, SquadUUIDs: groups, Timeline: connectivity.UnknownTimeline(now)}
		host.PublishedName, host.NameOverride, host.PriceLabel = proxy.PublishedName, proxy.NameOverride, proxy.PriceLabel
		host.CustomRoute, host.SplitXHTTP = proxy.CustomRoute, proxy.SplitXHTTP
		if links {
			link, err := proxy.NativeLink()
			if err != nil {
				host.LinkErrorCode = "CONNECTIVITY_LINK_UNAVAILABLE"
			} else {
				host.Link = &link
			}
		}
		result.Hosts = append(result.Hosts, host)
		if !monitored[proxy.ID] {
			continue
		}
		for _, id := range groups {
			index, ok := squads[id]
			if !ok {
				index = len(result.Squads)
				squads[id] = index
				result.Squads = append(result.Squads, connectivity.SubscriptionSquad{UUID: id, Name: names[id], HostUUIDs: []string{}, Timeline: connectivity.UnknownTimeline(now)})
			}
			result.Squads[index].HostUUIDs = append(result.Squads[index].HostUUIDs, host.UUID)
		}
	}
	return result
}
