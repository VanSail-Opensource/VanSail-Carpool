package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// HourlyPolicyProvider decrypts administrator settings outside database Store.
// Install during application construction, before starting the master workers.
type HourlyPolicyProvider interface {
	HourlyPolicy(context.Context) (model.HourlyPolicySnapshot, error)
}

func (s *Service) SetHourlyPolicyProvider(provider HourlyPolicyProvider) {
	s.hourlyPolicyProvider = provider
}

func (s *Service) enforcementHourlyPolicies(ctx context.Context, now time.Time) ([]HourlyAccountPolicy, error) {
	if s.hourlyPolicyProvider == nil {
		if repo, ok := s.repo.(HourlyPolicyRepository); ok {
			return repo.HourlyEnforcementPolicies(ctx, now)
		}
		return nil, nil
	}
	snapshot, err := s.hourlyPolicyProvider.HourlyPolicy(ctx)
	if err != nil {
		return nil, err
	}
	policy := snapshot.Policy
	if policy.DailyAllowanceBytes < 0 || policy.ReducedRateBytesPerSecond > 1_000_000_000_000 {
		return nil, ErrInvalid
	}
	var principals []HourlyPrincipal
	source, joined := s.repo.(interface {
		XNodeHourlyPolicyPrincipals(context.Context, time.Time) ([]HourlyPrincipal, error)
	})
	if joined {
		principals, err = source.XNodeHourlyPolicyPrincipals(ctx, now)
	} else {
		principals, err = s.hourlyPrincipals(ctx, "", "")
	}
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	byGroup := map[string]Group{}
	for _, group := range groups {
		byGroup[group.ID] = group
	}
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return nil, err
	}
	availableNodes := map[string]bool{}
	for _, node := range nodes {
		availableNodes[node.ID] = node.Enabled && node.RemovingAt == nil && node.RetiredAt == nil
	}
	byOwner := map[string]*HourlyAccountPolicy{}
	for _, principal := range principals {
		group := byGroup[principal.GroupID]
		if !hourlyPrincipalAvailable(principal, now) || !group.Enabled {
			continue
		}
		if !joined {
			allowed, e := s.hourlyPolicyPrincipalAllowed(ctx, principal)
			if e != nil {
				return nil, e
			}
			if !allowed {
				continue
			}
		}
		p := byOwner[principal.OwnerID]
		if p == nil {
			p = &HourlyAccountPolicy{OwnerID: principal.OwnerID, AccountGeneration: principal.AccountGeneration, DailyAllowanceBytes: policy.DailyAllowanceBytes, RateBytesPerSecond: policy.ReducedRateBytesPerSecond, NodePaidThrough: map[string]time.Time{}}
			byOwner[principal.OwnerID] = p
		}
		if principal.PaidThrough.After(p.PaidThrough) {
			p.PaidThrough = principal.PaidThrough
		}
		for _, binding := range group.Bindings {
			if !availableNodes[binding.NodeID] {
				continue
			}
			if principal.PaidThrough.After(p.NodePaidThrough[binding.NodeID]) {
				p.NodePaidThrough[binding.NodeID] = principal.PaidThrough
			}
		}
	}
	result := make([]HourlyAccountPolicy, 0, len(byOwner))
	for _, p := range byOwner {
		result = append(result, *p)
	}
	return result, nil
}

func (s *Service) hourlyPolicyPrincipalAllowed(ctx context.Context, p HourlyPrincipal) (bool, error) {
	owner, err := s.repo.UserByID(ctx, p.OwnerID)
	if err != nil {
		return false, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, p.OwnerID)
	if err != nil || restricted || owner.AccountGeneration != p.AccountGeneration || owner.AccountStatus != "active" || owner.OnboardingState != "complete" {
		return false, err
	}
	period, err := s.repo.XNodePeriod(ctx, p.PeriodID)
	return err == nil && period.Status == "open" && period.ProxyUserID == p.ProxyUserID, err
}
