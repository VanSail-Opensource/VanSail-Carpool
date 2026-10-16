package xnodemigration

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"slices"
)

func (s *Service) importMonitoring(ctx context.Context, account Account, snapshot Snapshot) error {
	if account.ServiceAccess == nil {
		return xnode.ErrInvalid
	}
	identity, err := s.panel.ImportServiceIdentity(ctx, account.ServiceName, account.LegacyID, account.UUID, account.Trojan, account.Chacha, *account.ServiceAccess)
	if err != nil {
		return err
	}
	if err = s.panel.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: account.UsedBytes, LifetimeBytes: account.LifetimeBytes, LastResetAt: account.LastResetAt, SyncedAt: snapshot.TakenAt}); err != nil {
		return err
	}
	if err = s.importHistory(ctx, identity, account, snapshot); err != nil {
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
