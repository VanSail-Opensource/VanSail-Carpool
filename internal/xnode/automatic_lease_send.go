package xnode

import (
	"context"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) automaticLeaseEligible(peer *agentPeer) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, live := m.whitelistStates[peer.nodeID], m.live[peer.nodeID]
	return m.peers[peer.nodeID] == peer && state.firewallEnabled && state.revision > 0 && live.report.Ready && live.report.Synchronized && live.firewallPending == "" && live.report.ConfigRevision >= state.revision && freshSnapshot(live, m.service.Now(), state.interval) && containsCapability(live.hello.Capabilities, wire.AutomaticLeaseCapability)
}

// Control payloads use the existing per-node writer and memory-only completion
// channels. No IP arguments or result receipts enter the durable command store.
func (m *Master) deliverAutomaticLeases(ctx context.Context, peer *agentPeer) (bool, error) {
	for range 16 {
		if !m.automaticLeaseEligible(peer) {
			return false, ErrUnavailable
		}
		peer.mu.Lock()
		cursor, reset := peer.automaticCursor, peer.automaticReset
		peer.mu.Unlock()
		items, through, more := m.automaticLeases.delivery(cursor, m.service.Now())
		if len(items) > 0 {
			update := wire.AutomaticLeaseUpdate{Epoch: m.automaticLeases.epoch, Leases: items}
			raw, err := m.commandPeer(ctx, peer, "", wire.AutomaticLeaseCommand, update)
			if err != nil {
				return false, err
			}
			var ack wire.AutomaticLeaseAck
			if wire.Decode(raw, &ack) != nil || ack.Epoch != update.Epoch || ack.Generation != items[len(items)-1].Generation {
				return false, ErrInvalid
			}
		}
		peer.mu.Lock()
		if peer.automaticReset != reset {
			peer.mu.Unlock()
			return false, ErrConflict
		}
		peer.automaticCursor = max(peer.automaticCursor, through)
		peer.automaticRetryDelay, peer.automaticRetryAt = 0, time.Time{}
		peer.mu.Unlock()
		if !more {
			return false, nil
		}
	}
	return true, nil
}
