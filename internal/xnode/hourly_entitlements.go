package xnode

import (
	"context"
	"slices"
	"sort"
)

func (s *Service) hasPaidHourlyBenefit(ctx context.Context, owner string) (bool, error) {
	principals, err := s.hourlyPrincipals(ctx, owner, "")
	if err != nil {
		return false, err
	}
	for _, p := range principals {
		if hourlyPrincipalAvailable(p, s.now()) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) hourlyMemberTags(ctx context.Context, owner, node string) ([]string, error) {
	principals, err := s.hourlyPrincipals(ctx, owner, node)
	if err != nil || len(principals) == 0 {
		return nil, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, p := range principals {
		if hourlyPrincipalAvailable(p, s.now()) {
			allowed[p.GroupID] = true
		}
	}
	tags := []string{}
	for _, group := range groups {
		if !group.Enabled || !allowed[group.ID] {
			continue
		}
		for _, binding := range group.Bindings {
			if binding.NodeID == node {
				tags = append(tags, binding.InboundTag)
			}
		}
	}
	sort.Strings(tags)
	return slices.Compact(tags), nil
}
