package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
)

// EndpointTopology reads current source configuration/host relations through the
// existing queue, without fetching user credentials or traffic history.
func (s migrationSource) EndpointTopology(ctx context.Context, config xnodemigration.SourceConfig) (xnodemigration.Snapshot, error) {
	snapshot := xnodemigration.Snapshot{TakenAt: time.Now().UTC(), Accounts: []xnodemigration.Account{}}
	adapter := s.adapter(config)
	return remnaCall(ctx, adapter, func(ctx context.Context, client remnaClient) (xnodemigration.Snapshot, error) {
		source, ok := client.(migrationClient)
		if !ok {
			return snapshot, xnode.ErrUnavailable
		}
		var err error
		snapshot.Profiles, err = source.MigrationProfiles(ctx)
		if err != nil {
			return snapshot, err
		}
		snapshot.Hosts, err = source.MigrationHosts(ctx)
		if err != nil {
			return snapshot, err
		}
		nodes, err := source.ListNodes(ctx)
		if err != nil {
			return snapshot, err
		}
		for _, node := range nodes {
			item := xnodemigration.Node{ID: node.UUID, Name: node.Name, CountryCode: node.CountryCode, Enabled: !node.IsDisabled, MultiplierFP: fixedNodeMultiplier(node.ConsumptionMultiplier), Inbounds: node.ConfigProfile.ActiveInbounds}
			if node.ConfigProfile.UUID != nil {
				item.ProfileID = *node.ConfigProfile.UUID
			}
			if node.Provider != nil {
				item.ProviderName = node.Provider.Name
			}
			snapshot.Nodes = append(snapshot.Nodes, item)
		}
		groups, err := source.ListInternalSquads(ctx)
		if err != nil {
			return snapshot, err
		}
		for _, group := range groups {
			snapshot.Groups = append(snapshot.Groups, xnodemigration.Group{ID: group.UUID, Name: group.Name, Inbounds: group.Inbounds})
		}
		return snapshot, nil
	})
}
