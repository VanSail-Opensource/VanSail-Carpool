package app

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
	"strconv"
	"time"
)

type migrationSource struct{ provider proxyProvider }
type migrationClient interface {
	remnaClient
	MigrationProfiles(context.Context) ([]remnawave.MigrationProfile, error)
	MigrationHosts(context.Context) ([]json.RawMessage, error)
	SetMigrationNodeDisabled(context.Context, string, bool) error
}

func (s migrationSource) adapter(config xnodemigration.SourceConfig) remnaAdapter {
	adapter := s.provider.remnaAdapter
	adapter.clientFactory = func(context.Context) (remnaClient, error) { return remnawave.NewClient(config.URL, config.Token) }
	return adapter
}
func (s migrationSource) Capture(ctx context.Context, config xnodemigration.SourceConfig, users []model.User, monitoring []xnodemigration.MonitoringMapping) (xnodemigration.Snapshot, error) {
	snapshot, err := s.EndpointTopology(ctx, config)
	snapshot.Monitoring = monitoring
	adapter := s.adapter(config)
	if err != nil {
		return snapshot, err
	}
	for _, panelUser := range users {
		if panelUser.RemnaUserID == nil {
			continue
		}
		id, err := strconv.ParseInt(*panelUser.RemnaUserID, 10, 64)
		if err != nil {
			return snapshot, err
		}
		account, err := remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) (xnodemigration.Account, error) {
			remote, err := client.GetUserByID(ctx, id)
			if err != nil {
				return xnodemigration.Account{}, missingMigrationAccountError(ctx, s.provider.store, panelUser.ID, err)
			}
			if remote == nil || remote.ID != id {
				return xnodemigration.Account{}, xnode.ErrConflict
			}
			item := xnodemigration.Account{LegacyID: id, Username: remote.Username, Disabled: remote.Status == remnawave.UserStatusDisabled}
			if panelUser.Username == nil || *panelUser.Username != remote.Username || remote.TelegramID == nil || *remote.TelegramID != panelUser.TelegramID {
				return item, nil
			}
			item.PanelUserID = panelUser.ID
			item.UUID, item.Trojan, item.Chacha = remote.VLESSUUID, remote.TrojanPassword, remote.ShadowsocksPassword
			item.UsedBytes, item.LifetimeBytes, item.LastResetAt = remote.UserTraffic.UsedTrafficBytes, remote.UserTraffic.LifetimeUsedTrafficBytes, remote.LastTrafficResetAt
			stats, err := client.GetUserStats(ctx, id, panelUser.CreatedAt, time.Now().UTC(), rolloverStatsTopNodesLimit)
			if err != nil {
				return item, err
			}
			if _, err := validateRolloverStatistics(remote, stats, panelUser.CreatedAt, time.Now().UTC()); err != nil {
				return item, err
			}
			item.Stats = *stats
			return item, nil
		})
		if err != nil {
			return snapshot, err
		}
		if account.LegacyID == 0 {
			continue
		}
		snapshot.Accounts = append(snapshot.Accounts, account)
	}
	seen := map[int64]bool{}
	for _, account := range snapshot.Accounts {
		seen[account.LegacyID] = true
	}
	for _, mapping := range monitoring {
		if seen[mapping.LegacyUserID] || mapping.ServiceName == "" {
			return snapshot, xnode.ErrConflict
		}
		seen[mapping.LegacyUserID] = true
		account, err := remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) (xnodemigration.Account, error) {
			remote, err := client.GetUserByID(ctx, mapping.LegacyUserID)
			if err != nil {
				return xnodemigration.Account{}, err
			}
			if remote == nil || remote.ID != mapping.LegacyUserID {
				return xnodemigration.Account{}, xnode.ErrConflict
			}
			return captureServiceAccount(ctx, client, *remote, mapping.ServiceName)
		})
		if err != nil {
			return snapshot, err
		}
		snapshot.Accounts = append(snapshot.Accounts, account)
	}
	err = s.captureUnmatched(ctx, adapter, &snapshot, seen)
	return snapshot, err
}
func (s migrationSource) SetNodeDisabled(ctx context.Context, config xnodemigration.SourceConfig, id string, disabled bool) error {
	return remnaExecute(ctx, s.adapter(config), func(ctx context.Context, client remnaClient) error {
		source, ok := client.(migrationClient)
		if !ok {
			return xnode.ErrUnavailable
		}
		return source.SetMigrationNodeDisabled(ctx, id, disabled)
	})
}
func (s migrationSource) SyncUsage(ctx context.Context, _ xnodemigration.SourceConfig, _ []model.User) error {
	return s.provider.syncBridgeAccounts(ctx)
}
