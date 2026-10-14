package xnodemigration

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"slices"
)

func (s *Service) importMonitoring(ctx context.Context, account Account) error {
	identity, err := s.panel.ImportServiceIdentity(ctx, account.ServiceName, account.LegacyID, account.UUID, account.Trojan, account.Chacha)
	if err != nil {
		return err
	}
	if account.Disabled || account.ExpiresAt == nil || !account.ExpiresAt.After(s.panel.Now()) {
		return nil
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(account.GroupIDs, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			if err := s.repo.SaveXNodeServiceBinding(ctx, xnode.ServiceBinding{ProxyUserID: identity.ProxyUserID, NodeID: binding.NodeID, InboundTag: binding.InboundTag, ValidUntil: *account.ExpiresAt}); err != nil {
				return err
			}
		}
	}
	return nil
}
