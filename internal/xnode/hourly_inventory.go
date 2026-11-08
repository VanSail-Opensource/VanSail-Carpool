package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func hourlyProxyID(endpoint, enrollment string) string {
	hash := sha256.Sum256([]byte(endpoint + "\x00" + enrollment))
	return "hourly-" + hex.EncodeToString(hash[:16])
}

func isPAYGProxy(proxy subscription.Proxy) bool {
	return proxy.BillingMode == "payg" || proxy.BillingMode == "" && strings.HasPrefix(proxy.ID, "payg-")
}

func hourlyInventoryStatus(p HourlyPrincipal) bool {
	return p.Status == "active" || p.Status == "funding_suspended"
}

// Keep an imported hourly host stable during automatic funding suspension;
// export never derives prepaid eligibility from the member's remaining wallet.
func (s *Service) hourlyInventory(ctx context.Context, owner string) ([]subscription.Proxy, error) {
	principals, err := s.hourlyPrincipals(ctx, owner, "")
	if err != nil || len(principals) == 0 {
		return []subscription.Proxy{}, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, group := range groups {
		enabled[group.ID] = group.Enabled
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	result := []subscription.Proxy{}
	for _, p := range principals {
		if !hourlyInventoryStatus(p) || p.ProxyUserID <= 0 || !enabled[p.GroupID] {
			continue
		}
		identity, e := s.repo.XNodeIdentity(ctx, p.ProxyUserID)
		if e != nil {
			return nil, e
		}
		if identity.AccessDisabled || identity.MemberOwnerID() != owner {
			continue
		}
		for _, endpoint := range endpoints {
			if !endpoint.Enabled {
				continue
			}
			node, e := s.repo.XNodeNode(ctx, endpoint.NodeID)
			if e != nil {
				return nil, e
			}
			if !node.Enabled || node.RetiredAt != nil || node.RemovingAt != nil {
				continue
			}
			proxy, e := s.endpointProxy(ctx, identity, endpoint)
			if e != nil {
				return nil, e
			}
			proxy.GroupIDs, e = filterEndpointGroups(proxy.GroupIDs, endpoint.Overrides)
			if e != nil {
				return nil, e
			}
			if !slices.Contains(proxy.GroupIDs, p.GroupID) {
				continue
			}
			proxy.ID, proxy.PublishedEndpointID = hourlyProxyID(endpoint.ID, p.EnrollmentID), endpoint.ID
			proxy.PrincipalID, proxy.BillingMode = p.ProxyUserID, "hourly"
			proxy.GroupIDs = []string{p.GroupID}
			result = append(result, proxy)
		}
	}
	return result, nil
}
