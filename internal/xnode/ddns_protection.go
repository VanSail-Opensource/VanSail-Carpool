package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Desired DDNS addresses participate in the existing protected firewall bank.
// The cache is keyed by both authoritative compile revision and durable phase;
// no DNS answer, owner projection or financial decision is cached here.
func (m *Master) refreshDDNSProtection(ctx context.Context) error {
	repo, ok := m.service.repo.(ddnsChangeRepository)
	if !ok {
		return nil
	}
	versions, ok := m.service.repo.(compileVersionRepository)
	if !ok {
		return ErrUnavailable
	}
	m.ddnsProtectionMu.Lock()
	defer m.ddnsProtectionMu.Unlock()
	states, err := repo.XNodeDDNSChanges(ctx)
	if err != nil {
		return err
	}
	version, _, err := versions.XNodeCompileVersion(ctx, m.service.now())
	if err != nil {
		return err
	}
	raw, err := json.Marshal(states)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	if digest == m.ddnsProtectionDigest && version == m.ddnsProtectionVersion {
		return nil
	}
	protected := []string{}
	trusted := map[string][]string{}
	jobs := map[string]map[string]wire.DDNSJob{}
	if len(states) > 0 {
		global, err := m.service.Global(ctx)
		if err != nil {
			return err
		}
		references, err := m.service.ddnsAddressReferences(ctx)
		if err != nil {
			return err
		}
		for _, state := range states {
			if jobs[state.NodeID] == nil {
				_, settings, err := m.service.Node(ctx, state.NodeID)
				if err != nil {
					return err
				}
				jobs[state.NodeID] = map[string]wire.DDNSJob{}
				for _, job := range ddnsJobsForNode(state.NodeID, settings, global.Providers) {
					jobs[state.NodeID][job.ID] = job
				}
			}
			pending := state.Phase == "staging" || state.Phase == "routes"
			for _, address := range []string{state.Change.IPv4, state.Change.IPv6} {
				if pending || jobs[state.NodeID][state.Change.JobID].ID != "" || references[address] {
					trusted[state.NodeID] = append(trusted[state.NodeID], address)
				}
			}
			for _, address := range state.PreviousIPs {
				if pending || references[address] {
					trusted[state.NodeID] = append(trusted[state.NodeID], address)
				}
			}
		}
		for node, addresses := range trusted {
			ips := []string{}
			for _, address := range addresses {
				if ip, e := FirewallPublicIP(address); e == nil {
					ips = append(ips, ip)
				}
			}
			slices.Sort(ips)
			ips = slices.Compact(ips)
			trusted[node] = ips
			protected = append(protected, ips...)
		}
	}
	slices.Sort(protected)
	protected = slices.Compact(protected)
	m.mu.Lock()
	m.ddnsProtectedIPs = protected
	m.ddnsNodeIPs = trusted
	m.mu.Unlock()
	if err = m.service.publishDDNSOrigins(ctx, states, jobs); err != nil {
		return err
	}
	m.ddnsProtectionDigest, m.ddnsProtectionVersion = digest, version
	return nil
}

// Queue all connected nodes through the existing durable firewall pipeline.
// Offline nodes receive the latest desired bank on reconnect. Never occupy an
// executor slot waiting for another firewall job's executor slot.
func (m *Master) stageDDNSProtection(ctx context.Context) (bool, error) {
	if err := m.refreshDDNSProtection(ctx); err != nil {
		return false, err
	}
	m.mu.RLock()
	peers := make([]*agentPeer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	m.mu.RUnlock()
	digest := protectedDigest(m.protectedIPs(false))
	ready := true
	for _, peer := range peers {
		if !containsCapability(m.Capabilities(peer.nodeID), "firewall_master_observation_v1") {
			continue
		}
		if err := m.queueProtected(ctx, peer); err != nil {
			return false, err
		}
		peer.mu.Lock()
		current := peer.protectedDigest == digest
		peer.mu.Unlock()
		if !current {
			ready = false
		}
	}
	return ready, nil
}

// A reconnect must stage the protected bank before publishing custom outbounds.
// Older agents retain their ordinary authorization/configuration compatibility.
func (m *Master) ddnsRouteProtectionReady(ctx context.Context, peer *agentPeer, prepared *preparedWire, capabilities []string) (bool, error) {
	m.mu.RLock()
	needed := []string{}
	for _, ip := range m.ddnsProtectedIPs {
		if prepared.routeAddresses[ip] {
			needed = append(needed, ip)
		}
	}
	for node, ips := range m.ddnsNodeIPs {
		if prepared.routeSources[node] {
			needed = append(needed, ips...)
		}
	}
	m.mu.RUnlock()
	if len(needed) == 0 || !containsCapability(capabilities, "firewall_master_observation_v1") {
		return true, nil
	}
	peer.mu.Lock()
	ready := true
	for _, ip := range needed {
		if !slices.Contains(peer.protectedIPs, ip) {
			ready = false
			break
		}
	}
	peer.mu.Unlock()
	if ready {
		return true, nil
	}
	return false, m.queueProtected(ctx, peer)
}
