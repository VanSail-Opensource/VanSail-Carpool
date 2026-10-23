package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"time"
)

func (m *Master) runFirewall() {
	defer m.wg.Done()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-tick.C:
		case <-m.firewallWake:
		}
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		if err := m.reconcileFirewall(ctx); err != nil && ctx.Err() == nil {
			m.logger.Warn("firewall reconciliation pending", "error_code", "XNODE_FIREWALL_PENDING", "cause", frameErrorCause(err))
		}
		cancel()
	}
}
func (m *Master) reconcileFirewall(ctx context.Context) error {
	if err := m.pruneManagedNodes(ctx); err != nil {
		return err
	}
	if err := m.refreshHostOrigins(ctx); err != nil {
		return err
	}
	repo := m.service.repo.(firewallRepository)
	ips := m.protectedIPs(false)
	digest := protectedDigest(ips)
	if digest != m.protectedReleaseDigest {
		if repo, ok := m.service.repo.(interface {
			ExcludeManagedFirewallIPs(context.Context, []string, time.Time) error
		}); ok {
			if err := repo.ExcludeManagedFirewallIPs(ctx, ips, m.service.now()); err != nil {
				return err
			}
		}
		m.protectedReleaseDigest = digest
	}
	if err := repo.SweepFirewall(ctx, m.service.Now()); err != nil {
		return err
	}
	m.mu.RLock()
	peers := make([]*agentPeer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	m.mu.RUnlock()
	for _, peer := range peers {
		if err := m.queueProtected(ctx, peer); err != nil {
			return err
		}
		m.mu.RLock()
		live := m.live[peer.nodeID]
		m.mu.RUnlock()
		if !hasFirewallCapability(live.hello.Capabilities) || !live.report.Synchronized {
			continue
		}
		node, settings, err := m.service.Node(ctx, peer.nodeID)
		if err != nil {
			return err
		}
		owned, err := m.service.NativeOwner(ctx, peer.nodeID)
		if err != nil {
			return err
		}
		active := owned && node.Enabled && settings.Firewall.Enabled
		if !active && (live.firewall == nil || live.firewall.Enabled) {
			continue
		} // retire only after host enforcement is disabled
		observed := uint64(0)
		if live.firewall != nil {
			observed = max(live.firewall.MasterVersion, live.firewall.SourceVersion)
		}
		peer.mu.Lock()
		reconnect := !peer.firewallJoined
		peer.mu.Unlock()
		if err = repo.ReconcileFirewallMember(ctx, peer.nodeID, active, reconnect, observed); err != nil {
			return err
		}
		peer.mu.Lock()
		peer.firewallJoined = true
		peer.mu.Unlock()
	}
	if m.firewallInventory != nil {
		allowed, err := m.firewallInventory.FirewallSources(ctx, func(source wire.FirewallSource, _ func(func([]string) error) error) error {
			_, e := repo.PublishFirewallSource(ctx, source, m.service.Now())
			return e
		})
		if err != nil {
			return err
		}
		if err = repo.ReconcileFirewallSources(ctx, allowed, m.service.Now()); err != nil {
			return err
		}
		version, err := repo.FirewallRevision(ctx)
		if err != nil {
			return err
		}
		if version == m.firewallInventoryVersion {
			return nil
		}
		prefixes := []string{}
		if err = repo.FirewallPrefixes(ctx, func(page []string) error { prefixes = append(prefixes, page...); return nil }); err != nil {
			return err
		}
		m.firewallInventory.SetDetectedPrefixes(prefixes)
		m.firewallInventoryVersion = version
	}
	return nil
}
