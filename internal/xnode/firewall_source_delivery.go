package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"slices"
)

func isDeferredFirewall(err error) bool { return errors.Is(err, outbox.ErrDeferred) }

// deliverFirewallSources only clears retired inventory sources. It never reads
// panel inventories or emits prefixes, references, bans, or positive sources.
func (m *Master) deliverFirewallSources(ctx context.Context, peer *agentPeer, member FirewallMember, current wire.FirewallState, id string) error {
	remove := make([]string, 0, len(current.SourceGenerations))
	for source := range current.SourceGenerations {
		remove = append(remove, source)
	}
	slices.Sort(remove)
	update := wire.FirewallUpdate{SourcesOnly: true, Version: member.SourceVersion, SnapshotID: id, Final: true, RemoveSources: remove}
	if _, err := m.Command(ctx, peer.nodeID, "", "firewall.bans.sync", update); err != nil {
		return err
	}
	return m.service.repo.(firewallRepository).MarkFirewallDelivered(ctx, peer.nodeID, member.SourceVersion, true, false, member.Epoch)
}
