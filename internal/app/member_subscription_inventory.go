package app

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func projectMemberInventory(raws []json.RawMessage, active []remnawave.SquadSummary, enabled []string, hosts []remnawave.Host, squads []remnawave.InternalSquad, nodes []remnawave.Node, keys *remnawave.ConnectionKeys) (connectivity.MemberSubscription, error) {
	result := connectivity.MemberSubscription{Hosts: []connectivity.SubscriptionHost{}, Squads: []connectivity.SubscriptionSquad{}}
	activeSet := map[string]bool{}
	for _, squad := range active {
		activeSet[squad.UUID] = true
	}
	squadInbounds := map[string]map[string]bool{}
	for _, squad := range squads {
		if !activeSet[squad.UUID] || !slices.Contains(enabled, squad.UUID) {
			continue
		}
		squadInbounds[squad.UUID] = map[string]bool{}
		for _, inbound := range squad.Inbounds {
			squadInbounds[squad.UUID][inbound.UUID] = true
		}
		result.Squads = append(result.Squads, connectivity.SubscriptionSquad{UUID: squad.UUID, Name: squad.Name, HostUUIDs: []string{}})
	}
	// Pending provider synchronization is unknown, never a smaller optimistic squad set.
	for _, uuid := range enabled {
		if squadInbounds[uuid] == nil {
			result.Squads = append(result.Squads, connectivity.SubscriptionSquad{UUID: uuid, Name: uuid, HostUUIDs: []string{}})
		}
	}
	sort.Slice(result.Squads, func(i, j int) bool { return result.Squads[i].UUID < result.Squads[j].UUID })
	byHost := map[string]remnawave.Host{}
	for _, host := range hosts {
		byHost[host.UUID] = host
	}
	positions := map[string]int64{}
	seen := map[string]bool{}
	for _, raw := range raws {
		resolved, err := decodeMemberHost(raw)
		if err != nil {
			return result, err
		}
		if resolved.Metadata.IsDisabled {
			continue
		}
		if seen[resolved.Metadata.UUID] {
			return result, connectivityFailure("INVALID_SUBSCRIPTION")
		}
		seen[resolved.Metadata.UUID] = true
		metadata, exists := byHost[resolved.Metadata.UUID]
		if !exists {
			return result, connectivityFailure("SUBSCRIPTION_UNAVAILABLE")
		}
		if metadata.IsDisabled {
			continue
		}
		squadUUIDs := []string{}
		for _, squad := range result.Squads {
			if squadInbounds[squad.UUID][resolved.Metadata.ConfigProfileInboundUUID] && metadata.AllowsSquad(squad.UUID) {
				squadUUIDs = append(squadUUIDs, squad.UUID)
			}
		}
		if len(squadUUIDs) == 0 {
			continue
		}
		link, code := memberHostLink(resolved, keys)
		result.Hosts = append(result.Hosts, connectivity.SubscriptionHost{UUID: resolved.Metadata.UUID, Name: resolved.FinalRemark,
			CountryCodes: memberHostCountries(metadata, resolved.Metadata.ConfigProfileInboundUUID, nodes), SquadUUIDs: squadUUIDs, Link: link, LinkErrorCode: code})
		positions[resolved.Metadata.UUID] = resolved.Metadata.ViewPosition
	}
	sort.Slice(result.Hosts, func(i, j int) bool {
		if positions[result.Hosts[i].UUID] == positions[result.Hosts[j].UUID] {
			return result.Hosts[i].UUID < result.Hosts[j].UUID
		}
		return positions[result.Hosts[i].UUID] < positions[result.Hosts[j].UUID]
	})
	for index := range result.Squads {
		for _, host := range result.Hosts {
			if slices.Contains(host.SquadUUIDs, result.Squads[index].UUID) {
				result.Squads[index].HostUUIDs = append(result.Squads[index].HostUUIDs, host.UUID)
			}
		}
	}
	return result, nil
}

func memberHostCountries(host remnawave.Host, inbound string, nodes []remnawave.Node) []string {
	countries := []string{}
	if host.Inbound.ConfigProfileInboundUUID == nil {
		host.Inbound.ConfigProfileInboundUUID = &inbound
	}
	relatedNodes := host.HostNodeIDs(nodes)
	for _, node := range nodes {
		related := slices.Contains(relatedNodes, node.UUID)
		country := strings.ToUpper(strings.TrimSpace(node.CountryCode))
		if related && country != "" && !slices.Contains(countries, country) {
			countries = append(countries, country)
		}
	}
	sort.Strings(countries)
	return countries
}
