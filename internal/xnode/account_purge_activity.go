package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type PurgeActivityScope struct {
	PrincipalID int64
	PeriodID    string
}

type purgeActivityAuthority struct {
	session  uint64
	revision uint64
	until    time.Time
}

type purgeActivityFence struct {
	periods     map[string]bool
	expires     time.Time
	authorities map[string]purgeActivityAuthority
}

// No network or SQL: callers hold the exact admitted account-generation lock.
// Shared IP lease banks survive; only the purged principals' associations leave.
func (m *Master) fenceAccountActivity(scopes []PurgeActivityScope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.service.now()
	m.expirePurgeActivityLocked(now, true)
	if m.purgedActivity == nil {
		m.purgedActivity = map[int]*purgeActivityFence{}
	}
	owned := map[int]bool{}
	for _, scope := range scopes {
		if scope.PrincipalID <= 0 || scope.PrincipalID >= 1<<53 || scope.PeriodID == "" {
			return ErrInvalid
		}
		owned[int(scope.PrincipalID)] = true
	}
	if len(owned) == 0 {
		return nil
	}
	newOwners, newScopes := 0, 0
	for id := range owned {
		if m.purgedActivity[id] == nil {
			newOwners++
		}
	}
	for _, scope := range scopes {
		if f := m.purgedActivity[int(scope.PrincipalID)]; f == nil || !f.periods[scope.PeriodID] {
			newScopes++
		}
	}
	if len(m.purgedActivity)+newOwners > maximumAutomaticLeases || m.purgeActivityScopes+newScopes > maximumAutomaticLeases {
		return ErrAccessAccountingPending
	}
	for _, scope := range scopes {
		id := int(scope.PrincipalID)
		fence := m.purgedActivity[id]
		if fence == nil {
			fence = &purgeActivityFence{periods: map[string]bool{}}
			m.purgedActivity[id] = fence
		}
		if !fence.periods[scope.PeriodID] {
			fence.periods[scope.PeriodID] = true
			m.purgeActivityScopes++
		}
		fence.expires = now.Add(2*time.Hour + 30*time.Second)
		fence.authorities = nil
	}
	for node, snapshot := range m.live {
		snapshot.report.Users = filterPurgeMetrics(snapshot.report.Users, owned)
		m.live[node] = snapshot
	}
	m.automaticLeases.mu.Lock()
	for _, lease := range m.automaticLeases.leases {
		for id := range lease.owners {
			if owned[int(id)] {
				delete(lease.owners, id)
				m.automaticLeases.ownerAssociations--
			}
		}
	}
	m.automaticLeases.mu.Unlock()
	return nil
}

func (m *Master) expirePurgeActivityLocked(now time.Time, force ...bool) {
	if len(force) == 0 && now.Before(m.purgeActivitySweepAt) {
		return
	}
	for id, fence := range m.purgedActivity {
		if !fence.expires.After(now) {
			m.purgeActivityScopes -= len(fence.periods)
			delete(m.purgedActivity, id)
		}
	}
	m.purgeActivitySweepAt = now.Add(30 * time.Second)
}

// Called by state publication under m.mu, without waiting on a peer lock. A
// different current paid period is authority for new activity, never its clock.
func (m *Master) recordPurgeActivityStateLocked(peer *agentPeer, config wire.Config, revision uint64) {
	if len(m.purgedActivity) == 0 {
		return
	}
	if peer.purgeActivitySession == 0 {
		if m.purgeActivitySession == ^uint64(0) {
			return
		}
		m.purgeActivitySession++
		peer.purgeActivitySession = m.purgeActivitySession
	}
	for _, fence := range m.purgedActivity {
		delete(fence.authorities, peer.nodeID)
	}
	for _, user := range config.Users {
		fence := m.purgedActivity[user.ID]
		if fence == nil || fence.periods[user.AccountingPeriod] || user.AccountingPeriod == "" || user.BlockNew || len(user.InboundTags) == 0 || len(user.Credentials) == 0 || user.ValidUntil == nil || !user.ValidUntil.After(m.service.now()) {
			continue
		}
		if fence.authorities == nil {
			fence.authorities = map[string]purgeActivityAuthority{}
		}
		if len(fence.authorities) < 128 || fence.authorities[peer.nodeID].session != 0 {
			fence.authorities[peer.nodeID] = purgeActivityAuthority{peer.purgeActivitySession, revision, *user.ValidUntil}
		}
	}
}

func (m *Master) filterPurgeActivityLocked(peer *agentPeer, report wire.TelemetryReport) wire.TelemetryReport {
	m.expirePurgeActivityLocked(m.service.now())
	if len(m.purgedActivity) == 0 {
		return report
	}
	owned := map[int]bool{}
	check := func(id int) {
		fence := m.purgedActivity[id]
		if fence == nil || !fence.expires.After(m.service.now()) {
			return
		}
		a := fence.authorities[peer.nodeID]
		if a.session != 0 && a.session == peer.purgeActivitySession && report.Ready && report.Synchronized && report.ConfigRevision >= a.revision && a.until.After(m.service.now()) {
			return
		}
		owned[id] = true
	}
	for id := range report.Users.Connections {
		check(id)
	}
	for id := range report.Users.TotalConnections {
		check(id)
	}
	for id := range report.Users.ActiveIPs {
		check(id)
	}
	for id := range report.Users.Status {
		check(id)
	}
	for id := range report.Users.Speed {
		check(id)
	}
	for id := range report.Users.Rates {
		check(id)
	}
	for _, ip := range report.Users.ActiveIPObservations {
		check(ip.UserID)
	}
	report.Users = filterPurgeMetrics(report.Users, owned)
	return report
}
