package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const replayStateMessageID = "state-replay"

// Replaying an already applied baseline does not reactivate native state. The
// evidence belongs to this connection; reconnects and unknown outcomes advance.
func (m *Master) canReplayAppliedState(peer *agentPeer, revision uint64, reportSeconds int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	live := m.live[peer.nodeID]
	return revision > 0 && m.peers[peer.nodeID] == peer && live.firewallPending == "" &&
		freshSnapshot(live, m.service.now(), reportSeconds) && live.report.Ready &&
		live.report.Synchronized && live.report.ConfigRevision == revision
}

// Caller holds syncMu and proved identical configuration and volatile controls.
// Leave first-sync evidence and durable period tracking untouched: an ACK-only
// replay neither activates users nor creates evidence of a new activation.
func (m *Master) replayAppliedState(ctx context.Context, peer *agentPeer, config wire.Config, revision uint64) (bool, error) {
	if err := peer.send(ctx, wire.New("state.sync", replayStateMessageID, revision, config)); err != nil {
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return false, err
	}
	return true, nil
}
