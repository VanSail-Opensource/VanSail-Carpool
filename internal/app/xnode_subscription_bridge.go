package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"slices"
	"strconv"
)

func (a proxyProvider) legacyProxies(ctx context.Context, identity xnode.Identity) ([]subscription.Proxy, error) {
	bridge, err := a.legacy(ctx)
	if err != nil {
		return nil, err
	}
	if !bridge {
		return []subscription.Proxy{}, nil
	}
	var purchase *model.Purchase
	if identity.UserID == "" {
		access, accessErr := a.native.ServiceAccess(ctx, identity)
		if accessErr != nil {
			return nil, accessErr
		}
		if access != nil {
			purchase = &model.Purchase{SquadUUIDs: access.GroupIDs}
		}
	} else {
		purchase, err = a.native.EffectivePurchase(ctx, identity.UserID)
	}
	if err != nil {
		return nil, err
	}
	if purchase == nil {
		return []subscription.Proxy{}, nil
	}
	enabled, err := a.store.EnabledSquadsForRemote(ctx, strconv.FormatInt(identity.ProxyUserID, 10), purchase.SquadUUIDs)
	if err != nil {
		return nil, err
	}
	state, err := a.store.XNodeMigration(ctx)
	if err != nil {
		return nil, err
	}
	ownership := map[string]string{}
	for _, node := range state.Nodes {
		ownership[node.LegacyNodeID] = node.Phase
	}
	id, err := strconv.ParseInt(identity.LegacyUserID, 10, 64)
	if err != nil {
		return nil, err
	}
	return remnaCall(ctx, a.remnaAdapter, func(ctx context.Context, client remnaClient) ([]subscription.Proxy, error) {
		user, err := client.GetUserByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if user == nil || user.ID != id {
			return nil, xnode.ErrConflict
		}
		if user.Status != remnawave.UserStatusActive {
			return []subscription.Proxy{}, nil
		}
		rawClient, ok := client.(memberRawClient)
		if !ok {
			return nil, xnode.ErrUnavailable
		}
		raw, err := rawClient.GetRawSubscription(ctx, user.ShortUUID)
		if err != nil {
			return nil, err
		}
		if raw == nil || raw.User.ID != id || raw.User.ShortUUID != user.ShortUUID {
			return nil, xnode.ErrConflict
		}
		hosts, err := client.ListHosts(ctx)
		if err != nil {
			return nil, err
		}
		squads, err := client.ListInternalSquads(ctx)
		if err != nil {
			return nil, err
		}
		nodes, err := client.ListNodes(ctx)
		if err != nil {
			return nil, err
		}
		inventory, err := projectMemberInventory(raw.ResolvedProxyConfigs, user.ActiveInternalSquads, enabled, hosts, squads, nodes, nil)
		if err != nil {
			return nil, err
		}
		eligible := map[string]connectivity.SubscriptionHost{}
		for _, host := range inventory.Hosts {
			eligible[host.UUID] = host
		}
		byHost := map[string]remnawave.Host{}
		for _, host := range hosts {
			byHost[host.UUID] = host
		}
		out := []subscription.Proxy{}
		for _, document := range raw.ResolvedProxyConfigs {
			host, err := decodeMemberHost(document)
			if err != nil {
				return nil, err
			}
			metadata, ok := eligible[host.Metadata.UUID]
			if !ok {
				continue
			}
			legacyHost := byHost[host.Metadata.UUID]
			legacyNode := ""
			if legacyHost.Inbound.ConfigProfileInboundUUID == nil {
				legacyHost.Inbound.ConfigProfileInboundUUID = &host.Metadata.ConfigProfileInboundUUID
			}
			hostNodes := legacyHost.HostNodeIDs(nodes)
			for _, node := range hostNodes {
				if ownership[node] != "native" {
					legacyNode = node
					break
				}
			}
			if legacyNode == "" && len(hostNodes) > 0 {
				continue
			}
			if check := raw.ConvertedUserInfo.HWIDCheckup; check != nil && !check.SubscriptionAllowed {
				continue
			}
			outbound, err := connectivity.ClientOutbound(connectivity.Target{HostUUID: host.Metadata.UUID, Resolved: document})
			if err != nil {
				return nil, err
			}
			proxy, err := proxyFromOutbound(host, legacyNode, outbound)
			if err != nil {
				return nil, err
			}
			proxy.GroupIDs = metadata.SquadUUIDs
			proxy.Countries = metadata.CountryCodes
			if len(enabled) > 0 && !slices.ContainsFunc(proxy.GroupIDs, func(group string) bool { return slices.Contains(enabled, group) }) {
				continue
			}
			out = append(out, proxy)
		}
		current, err := client.GetUserByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if current == nil || !sameSubscriptionCredentials(user, current) {
			return nil, xnode.ErrConflict
		}
		return out, nil
	})
}
func proxyFromOutbound(host memberResolvedHost, node string, out map[string]any) (subscription.Proxy, error) {
	proxy := subscription.Proxy{ID: host.Metadata.UUID, NodeID: node, Name: host.FinalRemark, Protocol: host.Protocol, Address: host.Address, Port: host.Port, Account: map[string]any{}}
	proxy.Stream, _ = out["streamSettings"].(map[string]any)
	settings, _ := out["settings"].(map[string]any)
	key := "servers"
	if proxy.Protocol == "vless" || proxy.Protocol == "vmess" {
		key = "vnext"
	}
	if proxy.Protocol == "hysteria" {
		proxy.Account["auth"] = objectValue(proxy.Stream, "hysteriaSettings")["auth"]
		return proxy, nil
	}
	servers, _ := settings[key].([]any)
	if len(servers) != 1 {
		return proxy, xnode.ErrInvalid
	}
	server, _ := servers[0].(map[string]any)
	if key == "vnext" {
		users, _ := server["users"].([]any)
		if len(users) != 1 {
			return proxy, xnode.ErrInvalid
		}
		proxy.Account, _ = users[0].(map[string]any)
	} else {
		for _, key := range []string{"password", "method", "uot", "UoTVersion"} {
			if value, ok := server[key]; ok {
				proxy.Account[key] = value
			}
		}
	}
	return proxy, nil
}
func objectValue(root map[string]any, key string) map[string]any {
	value, _ := root[key].(map[string]any)
	return value
}
