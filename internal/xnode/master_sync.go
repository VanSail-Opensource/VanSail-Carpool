package xnode

import (
	"context"
	"slices"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) reconcile(ctx context.Context) error {
	if err := m.service.reconcileRoutingPlans(ctx); err != nil {
		m.logger.Warn("routing reconciliation deferred", "error_code", "XNODE_ROUTING_RECONCILE_PENDING")
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return err
	}
	paid, err := m.service.principalOwners(ctx)
	if err != nil {
		return err
	}
	native, err := m.service.nativeAvailability(ctx)
	if err != nil {
		return err
	}
	now := m.service.now()
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
	if err := m.observePAYG(ctx, peers, global.Telemetry.ReportSeconds); err != nil {
		return err
	}
	for _, peer := range peers {
		peer.mu.Lock()
		force := peer.forceSync
		peer.forceSync = false
		peer.mu.Unlock()
		if force {
			if repo, ok := m.service.routingRepository(); ok {
				if err = repo.WakeXNodeRouting(ctx, now); err != nil {
					return err
				}
			}
		}
		if err = m.syncNode(ctx, peer, force); err != nil {
			if force {
				peer.mu.Lock()
				peer.forceSync = true
				peer.mu.Unlock()
			}
			if ctx.Err() != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Master) syncNode(ctx context.Context, peer *agentPeer, force bool) error {
	peer.syncMu.Lock()
	defer peer.syncMu.Unlock()
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

func (m *Master) syncControls(ctx context.Context, peer *agentPeer, current, previous map[int]bool) error {
	users := []wire.UserControl{}
	for id, blocked := range current {
		if old, ok := previous[id]; !ok || old != blocked {
			users = append(users, wire.UserControl{ID: id, BlockNew: blocked})
		}
	}
	if len(users) == 0 {
		return nil
	}
	peer.mu.Lock()
	base := peer.revision
	peer.mu.Unlock()
	revision, err := m.service.repo.AdvanceXNodeRevision(ctx, peer.nodeID, base)
	if err != nil {
		return err
	}
	message := wire.New("user.controls", "", revision, wire.UserControls{Users: users})
	message.BaseRevision = base
	peer.mu.Lock()
	peer.revision = revision
	peer.lastControls = current
	peer.mu.Unlock()
	if err = peer.send(ctx, message); err != nil {
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return err
	}
	return nil
}
