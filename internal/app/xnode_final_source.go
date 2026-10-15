package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
	"strconv"
	"time"
)

func (a proxyProvider) lockBridgeRead() func() {
	if a.bridgeGate == nil {
		return func() {}
	}
	a.bridgeGate.RLock()
	return a.bridgeGate.RUnlock
}

// Only the short final counter check holds this fence. Source history is staged
// earlier, while normal purchases and agent traffic continue independently.
func (s migrationSource) WithProvisioningFence(ctx context.Context, fn func(context.Context) error) error {
	gate := s.provider.bridgeGate
	if gate == nil {
		return fn(ctx)
	}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for !gate.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	defer gate.Unlock()
	return fn(ctx)
}

func finalAccount(remote remnawave.User) xnodemigration.Account {
	item := xnodemigration.Account{LegacyID: remote.ID, Username: remote.Username, UUID: remote.VLESSUUID, Trojan: remote.TrojanPassword, Chacha: remote.ShadowsocksPassword, UsedBytes: remote.UserTraffic.UsedTrafficBytes, LifetimeBytes: remote.UserTraffic.LifetimeUsedTrafficBytes, LastResetAt: remote.LastTrafficResetAt, ExpiresAt: &remote.ExpireAt, Disabled: remote.Status != remnawave.UserStatusActive}
	for _, group := range remote.ActiveInternalSquads {
		item.GroupIDs = append(item.GroupIDs, group.UUID)
	}
	item.TelegramID = remote.TelegramID
	return item
}

func (s migrationSource) FinalAccounts(ctx context.Context, config xnodemigration.SourceConfig) ([]xnodemigration.Node, []xnodemigration.Account, error) {
	adapter := s.adapter(config)
	nodes, err := remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) ([]remnawave.Node, error) { return client.ListNodes(ctx) })
	if err != nil {
		return nil, nil, err
	}
	projected := []xnodemigration.Node{}
	for _, node := range nodes {
		projected = append(projected, xnodemigration.Node{ID: node.UUID, Enabled: !node.IsDisabled})
	}
	accounts := []xnodemigration.Account{}
	identities, err := s.provider.store.XNodeIdentities(ctx)
	if err != nil {
		return nil, nil, err
	}
	linked := map[string]bool{}
	for _, identity := range identities {
		linked[identity.LegacyUserID] = true
	}
	err = s.visitAccounts(ctx, adapter, func(user remnawave.User) error {
		if linked[strconv.FormatInt(user.ID, 10)] {
			accounts = append(accounts, finalAccount(user))
		}
		return nil
	})
	return projected, accounts, err
}
