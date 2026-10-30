package xnode

import (
	"context"
	"slices"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) reconcile(ctx context.Context) error {
	if err := m.refreshDDNSProtection(ctx); err != nil {
		return err
	}
	if err := m.service.reconcileRoutingPlans(ctx); err != nil {
		m.logger.Warn("routing reconciliation deferred", "error_code", "XNODE_ROUTING_RECONCILE_PENDING")
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return err
	}
	native, err := m.service.nativeAvailability(ctx)
	if err != nil {
		return err
	}
	now := m.service.now()
	paid := map[int]string{}
	if global.ConnectionLimit > 0 {
		observed := map[int]bool{}
		m.mu.RLock()
		for id := range m.peers {
			snapshot := m.live[id]
			if native[id] && activityAvailable(snapshot, now, global.Telemetry.ReportSeconds) {
				for user, count := range snapshot.report.Users.Connections {
					if count > 0 {
						observed[user] = true
					}
				}
			}
		}
		m.mu.RUnlock()
		ids := make([]int, 0, len(observed))
		for id := range observed {
			ids = append(ids, id)
		}
		paid, err = m.service.observedPrincipalOwners(ctx, ids)
		if err != nil {
			return err
		}
	}
	m.mu.Lock()
	totals := map[string]int64{}
	peers := []*agentPeer{}
	for id, peer := range m.peers {
		peers = append(peers, peer)
		snapshot := m.live[id]
		if native[id] && activityAvailable(snapshot, now, global.Telemetry.ReportSeconds) {
			for user, count := range snapshot.report.Users.Connections {
				if owner := paid[user]; owner != "" {
					totals[owner] += int64(count)
				}
			}
		}
	}
	blocks := map[int]bool{}
	if global.ConnectionLimit > 0 {
		for id, owner := range paid {
			if totals[owner] >= int64(global.ConnectionLimit) {
				blocks[id] = true
			}
		}
	}
	m.globalBlocks = blocks
	m.mu.Unlock()
	if err = m.syncPeers(ctx, peers, now); err != nil {
		return err
	}
	// Renew authorization before connection observations can consume the deadline
	// or fail. Delivered traffic settles independently in the journal transaction.
	return m.observePAYG(ctx, peers, global.Telemetry.ReportSeconds)
}

func (m *Master) syncNode(ctx context.Context, peer *agentPeer, force bool) error {
	peer.syncMu.Lock()
	defer peer.syncMu.Unlock()
	return m.syncNodeLocked(ctx, peer, force)
}

func (m *Master) syncNodeLocked(ctx context.Context, peer *agentPeer, force bool) error {
	m.mu.RLock()
	hello := m.live[peer.nodeID].hello
	m.mu.RUnlock()
	if hello.UpgradePending {
		return nil
	}
	config, err := m.cachedConfiguration(ctx, peer, force)
	if err != nil {
		return err
	}
	config, ready, err := m.prepareCountryForPeer(ctx, peer, config, hello.Capabilities)
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}
	config, err = m.overlayPAYG(ctx, peer, config, hello.Capabilities)
	if err != nil {
		return err
	}
	prepared, err := preparePeerWire(peer, config, hello.Capabilities)
	if err != nil {
		return err
	}
	if ready, err = m.ddnsRouteProtectionReady(ctx, peer, prepared, hello.Capabilities); err != nil || !ready {
		return err
	}
	controls := m.connectionControls(prepared.config)
	digest := prepared.digest
	peer.mu.Lock()
	same := peer.digest == digest
	previous := peer.revision
	oldControls := peer.lastControls
	retry := peer.failedDigest == digest && m.service.now().Before(peer.retryAt)
	peer.mu.Unlock()
	if retry {
		return nil
	}
	if same && !force {
		return m.syncControls(ctx, peer, controls, oldControls)
	}
	revision, err := m.service.repo.AdvanceXNodeRevision(ctx, peer.nodeID, previous)
	if err != nil {
		return err
	}
	normalized := prepared.config
	normalized.Users = slices.Clone(prepared.config.Users)
	for i := range normalized.Users {
		normalized.Users[i].BlockNew = controls[normalized.Users[i].ID]
	}
	message := wire.New("state.sync", "", revision, normalized)
	for _, user := range config.Users {
		if user.AccountingPeriod != "" && !isPAYGPeriod(user.AccountingPeriod) {
			if err = m.service.repo.TrackXNodePeriodNode(ctx, user.AccountingPeriod, peer.nodeID); err != nil {
				return err
			}
		}
	}
	peer.mu.Lock()
	peer.revision = revision
	peer.digest, peer.lastControls = digest, controls
	if peer.failedDigest != digest {
		peer.failedDigest, peer.retryDelay, peer.retryAt = "", 0, time.Time{}
	}
	peer.mu.Unlock()
	m.recordWhitelistState(peer, normalized, revision)
	if err = peer.send(ctx, message); err != nil {
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return err
	}
	if !config.Firewall.Enabled {
		m.mu.Lock()
		snapshot := m.live[peer.nodeID]
		if m.peers[peer.nodeID] == peer {
			snapshot.firewallPending = ""
			m.live[peer.nodeID] = snapshot
		}
		m.mu.Unlock()
	}
	return nil
}
