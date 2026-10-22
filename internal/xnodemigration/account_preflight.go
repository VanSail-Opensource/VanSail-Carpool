package xnodemigration

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Verify every currently served source identity before disabling its node.
// Preview imports only linked members and explicitly mapped service accounts;
// unrelated active users must never disappear silently during a handover.
func (s *Service) verifyNodeAccounts(ctx context.Context, source SourceConfig, node string, users []wire.User) error {
	accounts, err := s.source.NodeAccounts(ctx, source, node)
	if err != nil || len(accounts) == 0 {
		return err
	}
	expected, err := s.legacyIdentities(ctx)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	ready := map[int64]bool{}
	for _, user := range users {
		ready[int64(user.ID)] = true
	}
	for _, account := range accounts {
		identity, matched := expected[account.LegacyID]
		if !matched || seen[account.LegacyID] {
			return fmt.Errorf("%w: source node has unmapped or duplicate identities", xnode.ErrConflict)
		}
		if !ready[identity.ProxyUserID] {
			return fmt.Errorf("%w: source account is not ready in the native configuration", xnode.ErrConflict)
		}
		seen[account.LegacyID] = true
		if identity.UserID == "" {
			_, err = s.panel.ImportServiceIdentity(ctx, identity.ServiceName, account.LegacyID, account.UUID, account.Trojan, account.Chacha)
		} else {
			user, userErr := s.repo.UserByID(ctx, identity.UserID)
			if userErr != nil {
				return userErr
			}
			if account.TelegramID == nil || *account.TelegramID != user.TelegramID {
				return xnode.ErrConflict
			}
			_, err = s.panel.ImportIdentity(ctx, user, account.LegacyID, account.UUID, account.Trojan, account.Chacha)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
