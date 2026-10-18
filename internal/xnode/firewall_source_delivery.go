package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func isDeferredFirewall(err error) bool { return errors.Is(err, outbox.ErrDeferred) }
func (m *Master) deliverFirewallSources(ctx context.Context, peer *agentPeer, member FirewallMember, current wire.FirewallState, id string) error {
	if m.firewallInventory == nil {
		return nil
	}
	repo := m.service.repo.(firewallRepository)
	stored, err := repo.FirewallSources(ctx)
	if err != nil {
		return err
	}
	known := map[string]wire.FirewallSource{}
	for _, v := range stored {
		known[v.ID] = v
	}
	index := 0
	sent := map[string]bool{}
	send := func(update wire.FirewallUpdate) error {
		update.SourcesOnly = true
		update.Version = member.SourceVersion
		update.SnapshotID = id
		update.Index = index
		if len(update.Sources) > 0 {
			if err := repo.JoinFirewallParticipants(ctx, peer.nodeID, nil, update.Sources); err != nil {
				return err
			}
		}
		_, err := m.Command(ctx, peer.nodeID, "", "firewall.bans.sync", update)
		if err == nil {
			index++
		}
		return err
	}
	allowed, err := m.firewallInventory.FirewallSources(ctx, func(source wire.FirewallSource, walk func(func([]string) error) error) error {
		desired, exists := known[source.ID]
		if !exists || desired.Generation != source.Generation {
			return nil
		}
		if !containsCapability(m.Capabilities(peer.nodeID), "disk_firewall_sources_v1") {
			return &CapabilityError{Capability: "disk_firewall_sources_v1"}
		}
		if current.SourceGenerations[source.ID] == source.Generation {
			source.Reference = true
			sent[source.ID] = true
			return send(wire.FirewallUpdate{Sources: []wire.FirewallSource{source}})
		}
		sent[source.ID] = true
		return walk(func(prefixes []string) error {
			source.Prefixes = prefixes
			return send(wire.FirewallUpdate{Sources: []wire.FirewallSource{source}})
		})
	})
	if err != nil {
		return err
	}
	enabled := map[string]bool{}
	for _, id := range allowed {
		enabled[id] = true
	}
	var remove []string
	for id := range current.SourceGenerations {
		if !enabled[id] {
			remove = append(remove, id)
		}
	}
	// During master warm-up retain cached positive generations. Explicitly
	// disabled feeds are removed; a failed or partial refresh never clears them.
	for sourceID, source := range known {
		if enabled[sourceID] && !sent[sourceID] && current.SourceGenerations[sourceID] == source.Generation {
			source.Reference = true
			if err = send(wire.FirewallUpdate{Sources: []wire.FirewallSource{source}}); err != nil {
				return err
			}
		}
	}
	if err = send(wire.FirewallUpdate{Final: true, RemoveSources: remove}); err != nil {
		return err
	}
	return repo.MarkFirewallDelivered(ctx, peer.nodeID, member.SourceVersion, true, false, member.Epoch)
}
