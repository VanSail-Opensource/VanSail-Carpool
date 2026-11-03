package xnode

import (
	"cmp"
	"net/netip"
	"slices"
	"time"
)

// One generation watermark per node replaces per-node per-IP ACK maps. Shared
// sorted snapshots are rebuilt only for grants, renewals or expiry, never for
// unchanged reports or statistics updates. Expired holes can be acknowledged.
func (m *automaticLeaseManager) delivery(after uint64, now time.Time) (grants []AutomaticLeaseGrant, through uint64, more bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	if !m.config.Enabled {
		return nil, after, false
	}
	if m.snapshotAt != m.revision {
		m.snapshot = make([]AutomaticLeaseGrant, 0, len(m.leases))
		for _, item := range m.leases {
			m.snapshot = append(m.snapshot, AutomaticLeaseGrant{item.ip.String(), item.generation, item.until})
		}
		slices.SortFunc(m.snapshot, func(a, b AutomaticLeaseGrant) int { return cmp.Compare(a.Generation, b.Generation) })
		m.snapshotAt = m.revision
	}
	start, found := slices.BinarySearchFunc(m.snapshot, after, func(item AutomaticLeaseGrant, generation uint64) int { return cmp.Compare(item.Generation, generation) })
	if found {
		start++
	}
	end := min(start+automaticLeaseBatch, len(m.snapshot))
	grants = slices.Clone(m.snapshot[start:end])
	through = m.generation
	if end < len(m.snapshot) {
		through, more = grants[len(grants)-1].Generation, true
	}
	return grants, through, more
}

func (m *automaticLeaseManager) pendingBanReleases(now time.Time) []AutomaticLeaseGrant {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	items := make([]AutomaticLeaseGrant, 0, min(automaticLeaseBatch, len(m.pendingBans)))
	for ip, generation := range m.pendingBans {
		item := m.leases[ip]
		items = append(items, AutomaticLeaseGrant{ip.String(), generation, item.until})
		if len(items) == automaticLeaseBatch {
			break
		}
	}
	return items
}

func (m *automaticLeaseManager) acknowledgeBanReleases(items []AutomaticLeaseGrant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		ip, err := netip.ParseAddr(item.IP)
		if err == nil && m.pendingBans[ip] == item.Generation {
			delete(m.pendingBans, ip)
		}
	}
}
