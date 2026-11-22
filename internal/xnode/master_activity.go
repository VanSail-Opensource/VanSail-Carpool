package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receiveActivity(peer *agentPeer, message wire.Message) error {
	report, err := decodeTelemetryFrame(peer, message)
	if err != nil {
		return err
	}
	if report.AgentID != peer.agentID || report.BootID != peer.bootID {
		return rejectTelemetry("identity_mismatch", ErrInvalid)
	}
	if !validMetrics(report.Users) {
		return rejectTelemetry("user_metrics", ErrInvalid)
	}
	if !peer.activityReports && (report.Users.SampledAt != nil || len(report.Users.ActiveIPObservations) != 0 || report.Users.ObservationDrops != 0) {
		return rejectTelemetry("activity_not_negotiated", ErrInvalid)
	}
	now := m.service.now()
	if report.Users.SampledAt != nil {
		if report.Users.SampledAt.After(report.Time) {
			return rejectTelemetry("sample_after_report", ErrInvalid)
		}
		if report.Users.SampledAt.After(now.Add(30 * time.Second)) {
			return rejectTelemetry("sample_in_future", ErrInvalid)
		}
	}
	m.mu.Lock()
	if m.peers[peer.nodeID] != peer {
		m.mu.Unlock()
		return rejectTelemetry("peer_replaced", ErrConflict)
	}
	report = m.filterPurgeActivityLocked(peer, report)
	snapshot := m.live[peer.nodeID]
	if previous := snapshot.report.Users.SampledAt; previous != nil && (report.Users.SampledAt == nil || report.Users.SampledAt.Before(*previous)) {
		m.mu.Unlock()
		return rejectTelemetry("sample_regressed", ErrConflict)
	}
	if sampled := report.Users.SampledAt; sampled != nil && (snapshot.report.Users.SampledAt == nil || sampled.After(*snapshot.report.Users.SampledAt)) {
		snapshot.activitySampleReceivedAt = now
	}
	automaticChanged := m.observeAutomaticActivityLocked(peer, report, snapshot.report.Users.SampledAt)
	snapshot.report = report
	snapshot.receivedAt = now
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
