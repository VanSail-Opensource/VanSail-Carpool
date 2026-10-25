package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"slices"
)

// Inventory is intentionally independent of wallet eligibility and node
// transport liveness. Imported clients retain entries through a funding pause.
func (s *Service) memberInventory(ctx context.Context, user string) ([]subscription.Proxy, bool, error) {
	member, err := s.repo.UserByID(ctx, user)
	if err != nil || member.OnboardingState != "complete" {
		return []subscription.Proxy{}, false, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, user)
	if err != nil || restricted {
		return []subscription.Proxy{}, false, err
	}
	parent, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return nil, true, err
	}
	if parent.AccessDisabled {
		return []subscription.Proxy{}, false, nil
	}
	combo, pending, err := s.comboInventory(ctx, user)
	if err != nil {
		return nil, pending, err
	}
	payg, err := s.paygInventory(ctx, user)
	return append(combo, payg...), pending, err
}

func paygProxyID(endpoint, group string) string {
	hash := sha256.Sum256([]byte(endpoint + "\x00" + group))
	return "payg-" + hex.EncodeToString(hash[:16])
}

func (s *Service) paygInventory(ctx context.Context, user string) ([]subscription.Proxy, error) {
	out := []subscription.Proxy{}
	r, ok := s.paygRepository()
	if !ok {
		return out, nil
	}
	accounts, err := r.XNodePAYGAccounts(ctx, user, "")
	if err != nil {
		return nil, err
	}
	products, err := r.ListSquadProductOverrides(ctx, false)
	if err != nil {
		return nil, err
	}
	payg := map[string]bool{}
	for _, p := range products {
		payg[p.GroupID] = p.PAYGAvailable()
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		payg[g.ID] = payg[g.ID] && g.Enabled
	}
	coverage, err := s.paygCoverage(ctx, user)
	if err != nil {
		return nil, err
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		if a.Disabled || !payg[a.GroupID] || coverage[a.GroupID] != "" {
			continue
		}
		i, e := s.repo.XNodeIdentity(ctx, a.ProxyUserID)
		if e != nil {
			return nil, e
		}
		if i.AccessDisabled {
			continue
		}
		for _, endpoint := range endpoints {
			if !endpoint.Enabled || endpoint.NodeID != a.NodeID {
				continue
			}
			node, e := s.repo.XNodeNode(ctx, endpoint.NodeID)
			if e != nil {
				return nil, e
			}
			if !node.Enabled {
				continue
			}
			proxy, e := s.endpointProxy(ctx, i, endpoint)
			if e != nil {
				return nil, e
			}
			proxy.GroupIDs, e = filterEndpointGroups(proxy.GroupIDs, endpoint.Overrides)
			if e != nil {
				return nil, e
			}
			if !slices.Contains(proxy.GroupIDs, a.GroupID) {
				continue
			}
			proxy.PublishedEndpointID = endpoint.ID
			proxy.PrincipalID = a.ProxyUserID
			proxy.ID = paygProxyID(endpoint.ID, a.GroupID)
			proxy.GroupIDs = []string{a.GroupID}
			out = append(out, proxy)
		}
	}
	return out, nil
}

// MemberPrincipals is an owner-scoped mapping for commands and observations;
// callers never accept arbitrary internal principal IDs from a member.
func (s *Service) MemberPrincipals(ctx context.Context, user, node string) ([]int64, error) {
	i, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return nil, err
	}
	out := []int64{i.ProxyUserID}
	if r, ok := s.paygRepository(); ok {
		accounts, e := r.XNodePAYGAccounts(ctx, user, node)
		if e != nil {
			return nil, e
		}
		for _, a := range accounts {
			out = append(out, a.ProxyUserID)
		}
	}
	return out, nil
}
