package xnode

import (
	"context"
	"slices"
)

func (s *Service) servicePeriod(ctx context.Context, identity Identity, access *ServiceAccess) (*Period, error) {
	if err := access.validate(); err != nil {
		return nil, err
	}
	if identity.AccessDisabled || access.Disabled || !access.ExpiresAt.After(s.now()) || identity.CurrentPeriodID == "" {
		return nil, nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return nil, err
	}
	if period.Status != "open" || period.PurchaseID != "" || period.ProxyUserID != identity.ProxyUserID || period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes {
		return nil, nil
	}
	return &period, nil
}

func (s *Service) serviceBindings(ctx context.Context, identity Identity, access *ServiceAccess) ([]ServiceBinding, error) {
	if access == nil {
		return s.repo.XNodeServiceBindings(ctx, identity.ProxyUserID)
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, err
	}
	bindings := []ServiceBinding{}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(access.GroupIDs, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			bindings = append(bindings, ServiceBinding{ProxyUserID: identity.ProxyUserID, NodeID: binding.NodeID, InboundTag: binding.InboundTag, ValidUntil: access.ExpiresAt})
		}
	}
	return bindings, nil
}
