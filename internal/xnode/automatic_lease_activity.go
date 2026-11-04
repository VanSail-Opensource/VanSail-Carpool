package xnode

import (
	"net/netip"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) configureAutomaticLeases(global GlobalConfig) {
	m.automaticLeases.configure(*global.AutomaticWhitelist)
	m.automaticFreshness.Store(int64(reportWindow(max(global.Telemetry.SampleSeconds, global.Telemetry.ReportSeconds))))
	m.notifyAutomaticLeases()
}

func (m *Master) notifyAutomaticLeases() {
	select {
	case m.automaticWake <- struct{}{}:
	default:
	}
}

// The caller owns m.mu and has authenticated this exact current peer. Firewall
// enablement, readiness and member admission are deliberately independent.
// Legacy snapshots cannot prove when an IP was sampled; only negotiated actual
// observation timestamps can create or renew an automatic lease.
func (m *Master) observeAutomaticActivityLocked(peer *agentPeer, report wire.TelemetryReport, previous *time.Time) bool {
	if !peer.activityReports || report.Users.SampledAt == nil || previous != nil && !report.Users.SampledAt.After(*previous) {
		return false
	}
	addresses := map[netip.Addr]time.Time{}
	for _, observation := range report.Users.ActiveIPObservations {
		if previous != nil && !observation.LastActive.After(*previous) {
			continue
		}
		ip, err := firewallPublicAddr(observation.IP)
		if err == nil && observation.LastActive.After(addresses[ip]) {
			addresses[ip] = observation.LastActive
		}
	}
	items := make([]AutomaticLeaseObservation, 0, len(addresses))
	for ip, at := range addresses {
		items = append(items, AutomaticLeaseObservation{ip.String(), peer.nodeID, at})
	}
	changed := m.automaticLeases.observe(items, m.service.now(), time.Duration(m.automaticFreshness.Load()))
	m.automaticLeases.associate(report.Users, m.service.now(), time.Duration(m.automaticFreshness.Load()))
	return changed
}
