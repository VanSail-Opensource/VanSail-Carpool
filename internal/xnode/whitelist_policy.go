package xnode

import (
	"context"
	"slices"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) whitelistQuota(ctx context.Context, user string) (int, error) {
	purchase, err := s.EffectivePurchase(ctx, user)
	if err != nil {
		return 0, err
	}
	if purchase == nil {
		return 5, nil
	}
	if repo, ok := s.repo.(interface {
		XNodeWhitelistQuota(context.Context, string) (int, error)
	}); ok {
		return repo.XNodeWhitelistQuota(ctx, purchase.ID)
	}
	return 5, nil
}

func normalizeWhitelist(previous, input WhitelistPolicy, quota int) (WhitelistPolicy, error) {
	if len(input.IPs) > quota || len(input.IPs) > 1000 {
		return input, ErrInvalid
	}
	requested := map[string]bool{}
	for _, raw := range input.IPs {
		ip, err := FirewallPublicIP(strings.TrimSpace(raw))
		if err != nil || requested[ip] {
			return input, ErrInvalid
		}
		requested[ip] = true
	}
	result := WhitelistPolicy{Enabled: input.Enabled, IPs: []string{}}
	for _, ip := range previous.IPs {
		if requested[ip] {
			result.IPs = append(result.IPs, ip)
			delete(requested, ip)
		}
	}
	for _, raw := range input.IPs {
		ip, _ := FirewallPublicIP(strings.TrimSpace(raw))
		if requested[ip] {
			result.IPs = append(result.IPs, ip)
			delete(requested, ip)
		}
	}
	return result, nil
}

func (s *Service) applyMemberWhitelist(ctx context.Context, user string, policy *wire.User) error {
	_, settings, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return err
	}
	quota, err := s.whitelistQuota(ctx, user)
	if err != nil {
		return err
	}
	policy.WhitelistEnabled = settings.Whitelist.Enabled
	if !policy.WhitelistEnabled {
		policy.WhitelistedIPs = nil
		return nil
	}
	policy.WhitelistedIPs = slices.Clone(settings.Whitelist.IPs[:min(len(settings.Whitelist.IPs), quota)])
	return nil
}

func (s *Service) ownedWhitelist(ctx context.Context, user string, policy WhitelistPolicy) (WhitelistView, error) {
	quota, err := s.whitelistQuota(ctx, user)
	if err != nil {
		return WhitelistView{}, err
	}
	view := WhitelistView{WhitelistPolicy: policy, Quota: quota, Phase: "disabled", PendingNodes: []string{}}
	view.IPs = slices.Clone(policy.IPs[:min(len(policy.IPs), quota)])
	if !policy.Enabled {
		return view, nil
	}
	nodes, err := s.accessibleNodeIDs(ctx, user)
	if err != nil {
		return view, err
	}
	if len(nodes) == 0 {
		view.Phase = "inactive"
		return view, nil
	}
	for node := range nodes {
		if s.master == nil || !s.master.whitelistApplied(node, user, view.WhitelistPolicy) {
			view.PendingNodes = append(view.PendingNodes, node)
		}
	}
	slices.Sort(view.PendingNodes)
	view.Phase = "active"
	if len(view.PendingNodes) > 0 {
		view.Phase = "pending"
	}
	return view, nil
}
