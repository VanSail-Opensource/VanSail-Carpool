package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type whitelistNodeState struct {
	firewallEnabled bool
	observing       bool
	revision        uint64
	interval        int
	exemptions      map[string]time.Time
}

func (m *Master) recordWhitelistState(peer *agentPeer, config wire.Config, revision uint64) {
	state := whitelistNodeState{firewallEnabled: config.Firewall.Enabled, observing: config.Firewall.Enabled && config.Firewall.Protect && config.Firewall.MasterControlled, revision: revision, interval: config.Telemetry.ReportSeconds, exemptions: map[string]time.Time{}}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.peers[peer.nodeID] != peer {
		return
	}
	m.recordPurgeActivityStateLocked(peer, config, revision)
	if m.whitelistStates == nil {
		m.whitelistStates = map[string]whitelistNodeState{}
	}
	if state.firewallEnabled && !m.whitelistStates[peer.nodeID].firewallEnabled {
		peer.mu.Lock()
		peer.automaticReset++
		peer.automaticCursor, peer.automaticRetryAt = 0, time.Time{}
		peer.mu.Unlock()
		m.notifyAutomaticLeases()
	}
	m.whitelistStates[peer.nodeID] = state
}
