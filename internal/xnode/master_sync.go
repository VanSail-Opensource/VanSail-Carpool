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
	now := m.service.now()
	m.mu.Lock()
	totals := map[int]int64{}
	peers := []*agentPeer{}
	for id, peer := range m.peers {
		peers = append(peers, peer)
		snapshot := m.live[id]
		if freshSnapshot(snapshot, now, global.Telemetry.ReportSeconds) {
			for user, count := range snapshot.report.Users.Connections {
				totals[user] += int64(count)
			}
		}
	}
	blocks := map[int]bool{}
	if global.ConnectionLimit > 0 {
		for user, count := range totals {
			if count > int64(global.ConnectionLimit) {
				blocks[user] = true
			}
		}
	}
	m.globalBlocks = blocks
	m.mu.Unlock()
	for _, peer := range peers {
		if err = m.syncNode(ctx, peer, false); err != nil && ctx.Err() != nil {
			return err
		}
	}
	return nil
}

func (m *Master) syncNode(ctx context.Context, peer *agentPeer, force bool) error {
	peer.syncMu.Lock()
	defer peer.syncMu.Unlock()
	config, err := m.service.Compile(ctx, peer.nodeID)
	if err != nil {
		return err
	}
	m.mu.RLock()
	hello := m.live[peer.nodeID].hello
	m.mu.RUnlock()
	if hello.UpgradePending {
		return nil
	}
	if len(config.Users) > 0 || len(config.MemberOutbounds) > 0 {
		supported := map[string]bool{}
		for _, capability := range hello.Capabilities {
			supported[capability] = true
		}
		for _, required := range []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"} {
			if !supported[required] {
				return ErrUnavailable
			}
		}
	}
	controls := map[int]bool{}
	normalized := wire.Clone(config)
	for i, user := range config.Users {
		controls[user.ID] = user.BlockNew
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
	oldControls := peer.lastControls
	peer.mu.Unlock()
	if same && !force {
		return m.syncControls(ctx, peer, controls, oldControls)
	}
	revision, err := m.service.repo.AdvanceXNodeRevision(ctx, peer.nodeID, previous)
	if err != nil {
		return err
	}
	message := wire.New("state.sync", "", revision, config)
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
