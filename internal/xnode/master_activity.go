package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receiveActivity(peer *agentPeer, message wire.Message) error {
	report, err := decodeTelemetryFrame(peer, message)
	if err != nil {
		return ErrInvalid
	}
	if report.AgentID != peer.agentID || report.BootID != peer.bootID || !validMetrics(report.Users) {
		return ErrInvalid
	}
	if !peer.activityReports && (report.Users.SampledAt != nil || len(report.Users.ActiveIPObservations) != 0 || report.Users.ObservationDrops != 0) {
		return ErrInvalid
	}
	if report.Users.SampledAt != nil && (report.Users.SampledAt.After(report.Time) || report.Users.SampledAt.After(m.service.now().Add(30*time.Second))) {
		return ErrInvalid
	}
	m.mu.Lock()
	if m.peers[peer.nodeID] != peer {
		m.mu.Unlock()
		return ErrConflict
	}
	report = m.filterPurgeActivityLocked(peer, report)
	snapshot := m.live[peer.nodeID]
	if previous := snapshot.report.Users.SampledAt; previous != nil && (report.Users.SampledAt == nil || report.Users.SampledAt.Before(*previous)) {
		m.mu.Unlock()
		return ErrConflict
	}
	if sampled := report.Users.SampledAt; sampled != nil && (snapshot.report.Users.SampledAt == nil || sampled.After(*snapshot.report.Users.SampledAt)) {
		snapshot.activitySampleReceivedAt = m.service.now()
	}
	automaticChanged := m.observeAutomaticActivityLocked(peer, report, snapshot.report.Users.SampledAt)
	snapshot.report = report
	snapshot.receivedAt = m.service.now()
	if report.Ready && report.Synchronized {
		snapshot.hello.UpgradePending = false
	}
	m.live[peer.nodeID] = snapshot
	m.mu.Unlock()
	if automaticChanged {
		m.notifyAutomaticLeases()
	}
	m.rememberNodeIPs(peer, report.Machine)
	return nil
}
