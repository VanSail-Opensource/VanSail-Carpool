package xnodemigration

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
)

func (s *Service) reconcileFinalSnapshot(ctx context.Context, snapshot Snapshot) error {
	expected, err := s.legacyIdentities(ctx)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, account := range snapshot.Accounts {
		identity, ok := expected[account.LegacyID]
		if !ok {
			continue
		}
		if seen[account.LegacyID] {
			return xnode.ErrConflict
		}
		seen[account.LegacyID] = true
		if identity.UserID == "" {
			if account.ServiceName != identity.ServiceName {
				return xnode.ErrConflict
			}
			if _, err := s.panel.ImportServiceIdentity(ctx, identity.ServiceName, account.LegacyID, account.UUID, account.Trojan, account.Chacha); err != nil {
				return err
			}
			if err = s.panel.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: account.UsedBytes, LifetimeBytes: account.LifetimeBytes, LastResetAt: account.LastResetAt, SyncedAt: snapshot.TakenAt}); err != nil {
				return err
			}
			if err = s.importHistory(ctx, identity, account, snapshot); err != nil {
				return err
			}
			continue
		}
		if account.PanelUserID != identity.UserID {
			return xnode.ErrConflict
		}
		user, err := s.repo.UserByID(ctx, identity.UserID)
		if err != nil {
			return err
		}
		// Resume verifies compatible secrets without rotating the native keys.
		if _, err = s.panel.ImportIdentity(ctx, user, account.LegacyID, account.UUID, account.Trojan, account.Chacha); err != nil {
			return err
		}
		if err = s.panel.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: account.UsedBytes, LifetimeBytes: account.LifetimeBytes, LastResetAt: account.LastResetAt, SyncedAt: snapshot.TakenAt}); err != nil {
			return err
		}
		if err = s.importHistory(ctx, identity, account, snapshot); err != nil {
			return err
		}
	}
	for id := range expected {
		if !seen[id] {
			return xnode.ErrConflict
		}
	}
	return nil
}

func (s *Service) legacyIdentities(ctx context.Context) (map[int64]xnode.Identity, error) {
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return nil, err
	}
	result := map[int64]xnode.Identity{}
	for _, identity := range identities {
		if identity.LegacyUserID == "" {
			continue
		}
		id, err := strconv.ParseInt(identity.LegacyUserID, 10, 64)
		if err != nil || id <= 0 {
			return nil, xnode.ErrInvalid
		}
		if _, duplicate := result[id]; duplicate {
			return nil, xnode.ErrConflict
		}
		result[id] = identity
	}
	return result, nil
}

func (s *Service) verifyFinalAccounts(ctx context.Context, snapshot Snapshot, current []Account) error {
	expected, err := s.legacyIdentities(ctx)
	if err != nil {
		return err
	}
	archived := map[int64]Account{}
	for _, account := range snapshot.Accounts {
		if _, duplicate := archived[account.LegacyID]; duplicate {
			return xnode.ErrConflict
		}
		archived[account.LegacyID] = account
	}
	observed := map[int64]Account{}
	for _, account := range current {
		if _, duplicate := observed[account.LegacyID]; duplicate {
			return xnode.ErrConflict
		}
		observed[account.LegacyID] = account
	}
	for id, identity := range expected {
		before, exists := archived[id]
		after, present := observed[id]
		if !exists || !present || before.UUID != after.UUID || before.Trojan != after.Trojan || before.Chacha != after.Chacha {
			return xnode.ErrConflict
		}
		if identity.UserID == "" {
			if before.ServiceName != identity.ServiceName || before.UsedBytes != after.UsedBytes || before.LifetimeBytes != after.LifetimeBytes || !sameReset(before, after) || !sameServiceAccess(before, after) {
				return xnode.ErrConflict
			}
			continue
		}
		if before.PanelUserID != identity.UserID || before.UsedBytes != after.UsedBytes || before.LifetimeBytes != after.LifetimeBytes || !sameReset(before, after) {
			return xnode.ErrConflict
		}
	}
	return nil
}
func sameServiceAccess(left, right Account) bool {
	if left.ServiceAccess == nil || right.ServiceAccess == nil {
		return left.ServiceAccess == nil && right.ServiceAccess == nil
	}
	return left.ServiceAccess.Equal(*right.ServiceAccess)
}
func sameReset(left, right Account) bool {
	if left.LastResetAt == nil || right.LastResetAt == nil {
		return left.LastResetAt == nil && right.LastResetAt == nil
	}
	return left.LastResetAt.Equal(*right.LastResetAt)
}
