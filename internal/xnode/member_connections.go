package xnode

import (
	"context"
	"slices"
	"strings"
)

func (s *Service) memberGrantEligible(ctx context.Context, user string) (bool, error) {
	member, err := s.repo.UserByID(ctx, user)
	if err != nil || member.OnboardingState != "complete" || member.AccountStatus != "active" {
		return false, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, user)
	if err != nil || restricted {
		return false, err
	}
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if isMissing(err) {
		return false, nil
	}
	if err != nil || identity.AccessDisabled {
		return false, err
	}
	purchase, err := s.EffectivePurchase(ctx, user)
	if err != nil || purchase != nil {
		return purchase != nil && err == nil, err
	}
	if paid, err := s.hasPaidHourlyBenefit(ctx, user); err != nil || paid {
		return paid, err
	}
	r, ok := s.paygRepository()
	if !ok {
		return false, ErrUnavailable
	}
	balance, err := r.Balance(ctx, user)
	return err == nil && balance.MinorInt64() > 0, err
}

// OwnedConnections projects existing native telemetry and volatile leases only.
func (s *Service) OwnedConnections(ctx context.Context, user, search string, page, size int) (MemberConnectionPage, error) {
	result := MemberConnectionPage{Items: []MemberConnection{}, Page: page, PageSize: size, Status: "unavailable", FetchedAt: s.Now()}
	if !validAutomaticLeasePage(search, page, size) {
		return result, ErrInvalid
	}
	global, err := s.Global(ctx)
	if err != nil {
		return result, err
	}
	result.Settings = *global.AutomaticWhitelist
	result.CanGrant, err = s.memberGrantEligible(ctx, user)
	result.CanGrant = result.CanGrant && result.Settings.Enabled
	if err != nil || s.master == nil {
		return result, err
	}
	principals, err := s.MemberPrincipals(ctx, user, "")
	if isMissing(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	nodes, err := s.master.UserNodeMetrics(ctx, user)
	if err != nil {
		return result, err
	}
	byIP := map[string]MemberConnection{}
	available, stale := false, false
	for _, node := range nodes {
		available = available || node.ActiveIPStatus == "available"
		stale = stale || node.ActiveIPStatus == "stale"
		result.Incomplete = result.Incomplete || node.ActiveIPStatus != "available" || node.ActiveIPsTruncated
		for _, observation := range node.ActiveIPs {
			item := byIP[observation.IP]
			if item.LastObservedAt == nil || observation.LastActive.After(*item.LastObservedAt) {
				at := observation.LastActive
				item.IP, item.LastObservedAt = observation.IP, &at
				byIP[item.IP] = item
			}
		}
	}
	leases, clipped := s.master.automaticLeases.memberLeases(principals, result.FetchedAt)
	result.Incomplete = result.Incomplete || clipped
	for _, item := range leases {
		if observation := byIP[item.IP]; observation.LastObservedAt != nil && (item.LastObservedAt == nil || observation.LastObservedAt.After(*item.LastObservedAt)) {
			item.LastObservedAt = observation.LastObservedAt
		}
		byIP[item.IP] = item
	}
	if available {
		result.Status = "available"
	} else if stale {
		result.Status = "stale"
	}
	if ip, e := FirewallPublicIP(search); e == nil {
		search = ip
	}
	search = strings.ToLower(strings.TrimSpace(search))
	items := []MemberConnection{}
	for _, item := range byIP {
		if strings.Contains(item.IP, search) {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b MemberConnection) int { return strings.Compare(a.IP, b.IP) })
	result.Total = len(items)
	result.Page = min(page, max(1, (len(items)+size-1)/size))
	start := (result.Page - 1) * size
	result.Items = items[start:min(start+size, len(items))]
	if available && len(items) == 0 {
		result.Status = "empty"
	}
	return result, nil
}

// GrantOwnedConnection reuses automatic lease delivery without billing or SQL IPs.
func (s *Service) GrantOwnedConnection(ctx context.Context, user, ip string) (MemberConnection, error) {
	if s.master == nil {
		return MemberConnection{}, ErrUnavailable
	}
	eligible, err := s.memberGrantEligible(ctx, user)
	if err != nil {
		return MemberConnection{}, err
	}
	if !eligible {
		return MemberConnection{}, ErrConflict
	}
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return MemberConnection{}, err
	}
	grant, err := s.master.automaticLeases.manualGrant(identity.ProxyUserID, ip, s.Now())
	if err == nil {
		s.master.notifyAutomaticLeases()
	}
	return grant, err
}
