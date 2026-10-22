package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"slices"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) rememberNodeIPs(peer *agentPeer, raw json.RawMessage) {
	var machine struct {
		Interfaces []struct {
			Addresses []string `json:"addresses"`
		} `json:"interfaces"`
	}
	if json.Unmarshal(raw, &machine) != nil {
		return
	}
	ips := []string{}
	if peer.publicIP != "" {
		ips = append(ips, peer.publicIP)
	}
	for _, device := range machine.Interfaces {
		for _, raw := range device.Addresses {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil {
				continue
			}
			if ip, err := FirewallPublicIP(prefix.Addr().Unmap().String()); err == nil {
				ips = append(ips, ip)
			}
		}
	}
	slices.Sort(ips)
	ips = slices.Compact(ips)
	m.mu.Lock()
	if m.managedNodeIPs == nil {
		m.managedNodeIPs = map[string][]string{}
	}
	changed := !slices.Equal(ips, m.managedNodeIPs[peer.nodeID])
	m.managedNodeIPs[peer.nodeID] = ips
	m.mu.Unlock()
	if changed {
		m.NotifyFirewallSources()
	}
}

func (m *Master) protectedIPs(includeMembers bool) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set := map[string]bool{}
	for _, ips := range m.managedNodeIPs {
		for _, ip := range ips {
			set[ip] = true
		}
	}
	for _, ip := range m.managedHostIPs {
		set[ip] = true
	}
	if includeMembers {
		for _, state := range m.whitelistStates {
			for ip, until := range state.exemptions {
				if until.After(m.service.now()) {
					set[ip] = true
				}
			}
		}
	}
	ips := make([]string, 0, len(set))
	for ip := range set {
		ips = append(ips, ip)
	}
	slices.Sort(ips)
	return ips
}

func protectedDigest(ips []string) string {
	raw, _ := json.Marshal(ips)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func (m *Master) deliverProtected(ctx context.Context, peer *agentPeer) error {
	if !containsCapability(m.Capabilities(peer.nodeID), "firewall_master_observation_v1") {
		return &CapabilityError{Capability: "firewall_master_observation_v1"}
	}
	peer.protectedMu.Lock()
	defer peer.protectedMu.Unlock()
	ips := m.protectedIPs(false)
	digest := protectedDigest(ips)
	peer.mu.Lock()
	current := peer.protectedDigest == digest && m.service.now().Sub(peer.protectedAt) < time.Hour
	peer.mu.Unlock()
	if current {
		return nil
	}
	_, err := m.Command(ctx, peer.nodeID, "", "firewall.protected.sync", map[string]any{"ips": ips})
	if err == nil {
		peer.mu.Lock()
		peer.protectedDigest = digest
		peer.protectedAt = m.service.now()
		peer.mu.Unlock()
	}
	return err
}

func (m *Master) queueProtected(ctx context.Context, peer *agentPeer) error {
	if !containsCapability(m.Capabilities(peer.nodeID), "firewall_master_observation_v1") {
		return nil
	}
	digest := protectedDigest(m.protectedIPs(false))
	peer.mu.Lock()
	current := peer.protectedDigest == digest && m.service.now().Sub(peer.protectedAt) < time.Hour
	peer.mu.Unlock()
	if current {
		return nil
	}
	repo, ok := m.service.repo.(interface {
		EnqueueOutbox(context.Context, string, string, time.Time) error
	})
	if !ok {
		return ErrUnavailable
	}
	raw, _ := json.Marshal(map[string]string{"nodeId": peer.nodeID, "bank": "protected"})
	return repo.EnqueueOutbox(ctx, FirewallJob, string(raw), m.service.now())
}

func (m *Master) pruneManagedNodes(ctx context.Context) error {
	nodes, err := m.service.Nodes(ctx)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, node := range nodes {
		known[node.ID] = true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.managedNodeIPs {
		if !known[id] {
			delete(m.managedNodeIPs, id)
			delete(m.whitelistStates, id)
		}
	}
	return nil
}

func (m *Master) effectiveFirewall(ctx context.Context, node string) (wire.FirewallPolicy, error) {
	_, settings, err := m.service.Node(ctx, node)
	if err != nil {
		return wire.FirewallPolicy{}, err
	}
	global, err := m.service.Global(ctx)
	return global.FirewallDetection.Apply(settings.Firewall), err
}
