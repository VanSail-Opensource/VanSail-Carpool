package xnode

import (
	"slices"
	"strconv"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type whitelistNodeState struct {
	observing  bool
	revision   uint64
	interval   int
	users      map[int]WhitelistPolicy
	exemptions map[string]time.Time
}

func (m *Master) recordWhitelistState(peer *agentPeer, config wire.Config, revision uint64) {
	state := whitelistNodeState{observing: config.Firewall.Enabled && config.Firewall.Protect && config.Firewall.MasterControlled, revision: revision, interval: config.Telemetry.ReportSeconds, users: map[int]WhitelistPolicy{}, exemptions: map[string]time.Time{}}
	for _, user := range config.Users {
		state.users[user.ID] = WhitelistPolicy{Enabled: user.WhitelistEnabled, IPs: slices.Clone(user.WhitelistedIPs)}
		if user.WhitelistEnabled {
			until := time.Now().Add(365 * 24 * time.Hour)
			if user.ValidUntil != nil {
				until = *user.ValidUntil
			}
			for _, ip := range user.WhitelistedIPs {
				if until.After(state.exemptions[ip]) {
					state.exemptions[ip] = until
				}
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.whitelistStates == nil {
		m.whitelistStates = map[string]whitelistNodeState{}
	}
	m.whitelistStates[peer.nodeID] = state
}

func (m *Master) whitelistApplied(node, user string, desired WhitelistPolicy) bool {
	id, err := strconv.ParseInt(user, 10, 64)
	if err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer := m.peers[node]
	live := m.live[node]
	state := m.whitelistStates[node]
	interval := time.Duration(max(30, state.interval*3)) * time.Second
	if peer == nil || !live.report.Ready || !live.report.Synchronized || live.firewallPending != "" || live.report.ConfigRevision < state.revision || state.revision == 0 || m.service.now().Sub(live.receivedAt) > interval {
		return false
	}
	policy, exists := state.users[int(id)]
	if state.observing && (live.firewall == nil || !live.firewall.KernelReady) {
		return false
	}
	if !exists {
		return false
	}
	return policy.Enabled == desired.Enabled && (!desired.Enabled || slices.Equal(policy.IPs, desired.IPs))
}
