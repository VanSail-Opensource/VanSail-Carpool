package app

import (
	"context"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
)

// visitAccounts bounds each upstream page and rejects cursor loops. The visitor
// selects required metadata; unrelated credentials never enter stored previews.
func (s migrationSource) visitAccounts(ctx context.Context, adapter remnaAdapter, visit func(remnawave.User) error) error {
	cursor := ""
	seen := map[string]bool{}
	for {
		page, err := remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) (notificationUserPage, error) {
			users, next, more, err := client.ListUsersStream(ctx, cursor, 200)
			return notificationUserPage{users: users, next: next, more: more}, err
		})
		if err != nil {
			return err
		}
		for _, user := range page.users {
			if err = visit(user); err != nil {
				return err
			}
		}
		if !page.more {
			return nil
		}
		if page.next == nil || *page.next == cursor || seen[*page.next] {
			return xnode.ErrInvalid
		}
		seen[cursor] = true
		cursor = *page.next
	}
}

func (s migrationSource) NodeAccounts(ctx context.Context, config xnodemigration.SourceConfig, id string) ([]xnodemigration.Account, error) {
	adapter := s.adapter(config)
	groups, err := remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) (map[string]bool, error) {
		nodes, err := client.ListNodes(ctx)
		if err != nil {
			return nil, err
		}
		inbounds := map[string]bool{}
		found := false
		for _, node := range nodes {
			if node.UUID == id {
				found = true
				for _, inbound := range node.ConfigProfile.ActiveInbounds {
					inbounds[inbound.UUID] = true
				}
			}
		}
		if !found {
			return nil, xnode.ErrConflict
		}
		squads, err := client.ListInternalSquads(ctx)
		if err != nil {
			return nil, err
		}
		groups := map[string]bool{}
		for _, squad := range squads {
			if slices.ContainsFunc(squad.Inbounds, func(inbound remnawave.NodeInbound) bool { return inbounds[inbound.UUID] }) {
				groups[squad.UUID] = true
			}
		}
		return groups, nil
	})
	if err != nil {
		return nil, err
	}
	result := []xnodemigration.Account{}
	err = s.visitAccounts(ctx, adapter, func(user remnawave.User) error {
		if user.Status == remnawave.UserStatusActive && user.ExpireAt.After(time.Now()) && slices.ContainsFunc(user.ActiveInternalSquads, func(group remnawave.SquadSummary) bool { return groups[group.UUID] }) {
			result = append(result, finalAccount(user))
		}
		return nil
	})
	return result, err
}

func (s migrationSource) captureUnmatched(ctx context.Context, adapter remnaAdapter, snapshot *xnodemigration.Snapshot, seen map[int64]bool) error {
	return s.visitAccounts(ctx, adapter, func(user remnawave.User) error {
		if !seen[user.ID] && user.Status == remnawave.UserStatusActive && user.ExpireAt.After(time.Now()) {
			seen[user.ID] = true
			snapshot.Accounts = append(snapshot.Accounts, xnodemigration.Account{LegacyID: user.ID, Username: user.Username})
		}
		return nil
	})
}
