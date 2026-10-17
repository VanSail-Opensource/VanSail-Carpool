package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) reconcile(ctx context.Context) error {
	global, err := m.service.Global(ctx)
	if err != nil {
		return err
	}
	identities, err := m.service.repo.XNodeIdentities(ctx)
	if err != nil {
		return err
	}
	paid := map[int]bool{}
	for _, identity := range identities {
		if identity.UserID != "" {
			paid[int(identity.ProxyUserID)] = true
		}
	}
	native, err := m.service.nativeAvailability(ctx)
	if err != nil {
		return err
	}
	now := m.service.now()
	m.mu.Lock()
	totals := map[int]int64{}
	peers := []*agentPeer{}
	for id, peer := range m.peers {
		peers = append(peers, peer)
		snapshot := m.live[id]
		if native[id] && activityAvailable(snapshot, now, global.Telemetry.ReportSeconds) {
			for user, count := range snapshot.report.Users.Connections {
				if paid[user] {
					totals[user] += int64(count)
				}
			}
		}
	}
	blocks := map[int]bool{}
	if global.ConnectionLimit > 0 {
		for user, count := range totals {
			if count >= int64(global.ConnectionLimit) {
				blocks[user] = true
			}
		}
	}
	m.globalBlocks = blocks
	m.mu.Unlock()
	for _, peer := range peers {
		peer.mu.Lock()
		force := peer.forceSync
		peer.forceSync = false
		peer.mu.Unlock()
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
	previousCompiled := peer.compiled
	config, err := m.cachedConfiguration(ctx, peer, force)
	if err != nil {
		return err
	}
	m.mu.RLock()
	hello := m.live[peer.nodeID].hello
	m.mu.RUnlock()
	if hello.UpgradePending {
		return nil
	}
	config = firewallForCapabilities(config, hello.Capabilities)
	if err = configurationCapabilities(config, hello.Capabilities); err != nil {
		return err
	}
	controls := m.connectionControls(config)
	peer.mu.Lock()
	unchanged := peer.digest != "" && previousCompiled != nil && previousCompiled == peer.compiled
	oldControls := peer.lastControls
	peer.mu.Unlock()
	if unchanged && !force {
		return m.syncControls(ctx, peer, controls, oldControls)
	}
	normalized := wire.Clone(config)
	for i := range config.Users {
		normalized.Users[i].BlockNew = false
	}
	raw, err := json.Marshal(normalized)
	if err != nil || len(raw) > wire.MaxFrameBytes-1024 {
		return ErrInvalid
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	peer.mu.Lock()
	same := peer.digest == digest
	previous := peer.revision
	oldControls = peer.lastControls
	peer.mu.Unlock()
	if same && !force {
		return m.syncControls(ctx, peer, controls, oldControls)
	}
	revision, err := m.service.repo.AdvanceXNodeRevision(ctx, peer.nodeID, previous)
	if err != nil {
		return err
	}
	for i := range normalized.Users {
		normalized.Users[i].BlockNew = controls[normalized.Users[i].ID]
	}
	message := wire.New("state.sync", "", revision, normalized)
	for _, user := range config.Users {
		if user.AccountingPeriod != "" {
			if err = m.service.repo.TrackXNodePeriodNode(ctx, user.AccountingPeriod, peer.nodeID); err != nil {
				return err
			}
		}
	}
	peer.mu.Lock()
	peer.revision = revision
	peer.mu.Unlock()
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
	peer.mu.Lock()
	peer.revision, peer.digest, peer.lastControls = revision, digest, controls
	peer.mu.Unlock()
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
	peer.mu.Unlock()
	if err = peer.send(ctx, message); err != nil {
		peer.mu.Lock()
		peer.digest = ""
		peer.mu.Unlock()
		return err
	}
	peer.mu.Lock()
	peer.revision, peer.lastControls = revision, current
	peer.mu.Unlock()
	return nil
}
