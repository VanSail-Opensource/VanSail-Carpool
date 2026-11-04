package xnode

import (
	"container/heap"
	"math"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Associations share the lease lifetime and disappear with its heap entry.
// Principal IDs are resolved to owners only on reads, outside socket readers.
func (m *automaticLeaseManager) associate(report wire.UserMetrics, now time.Time, freshness time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, observation := range report.ActiveIPObservations {
		if observation.UserID <= 0 || observation.LastActive.Before(now.Add(-freshness)) || observation.LastActive.After(now.Add(30*time.Second)) {
			continue
		}
		ip, err := firewallPublicAddr(observation.IP)
		item := m.leases[ip]
		if err != nil || item == nil {
			continue
		}
		if item.owners == nil {
			item.owners = map[int64]time.Time{}
		}
		id := int64(observation.UserID)
		if _, exists := item.owners[id]; !exists && m.ownerAssociations >= maximumAutomaticLeases {
			m.associationsTruncated = true
			continue
		}
		if _, exists := item.owners[id]; !exists {
			m.ownerAssociations++
		}
		if observation.LastActive.After(item.owners[id]) {
			item.owners[id] = observation.LastActive
		}
	}
}

func (m *automaticLeaseManager) manualGrant(principal int64, raw string, now time.Time) (MemberConnection, error) {
	ip, err := firewallPublicAddr(raw)
	if err != nil || raw != ip.String() || principal <= 0 {
		return MemberConnection{}, ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	if !m.config.Enabled {
		return MemberConnection{}, ErrConflict
	}
	item := m.leases[ip]
	if item == nil {
		if m.ownerAssociations >= maximumAutomaticLeases {
			return MemberConnection{}, ErrConflict
		}
		if len(m.leases) >= maximumAutomaticLeases || m.generation == math.MaxUint64 {
			return MemberConnection{}, ErrConflict
		}
		item = &automaticLease{ip: ip, first: now, started: now, until: now.Add(time.Duration(m.config.DurationHours) * time.Hour)}
		m.leases[ip] = item
		heap.Push(&m.expiry, item)
		m.created++
		m.grantLocked(item)
	}
	if item.owners == nil {
		item.owners = map[int64]time.Time{}
	}
	if _, exists := item.owners[principal]; !exists {
		if m.ownerAssociations >= maximumAutomaticLeases {
			return MemberConnection{}, ErrConflict
		}
		m.ownerAssociations++
		item.owners[principal] = time.Time{}
	}
	return memberLeaseView(item, item.owners[principal]), nil
}

func memberLeaseView(item *automaticLease, observed time.Time) MemberConnection {
	first, until := item.first, item.until
	view := MemberConnection{IP: item.ip.String(), InitialLeaseAt: &first, ExpiresAt: &until}
	if !observed.IsZero() {
		view.LastObservedAt = &observed
	}
	return view
}

func (m *automaticLeaseManager) memberLeases(principals []int64, now time.Time) ([]MemberConnection, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	items := []MemberConnection{}
	for _, item := range m.leases {
		found, last := false, time.Time{}
		for _, principal := range principals {
			if at, ok := item.owners[principal]; ok {
				found = true
				if at.After(last) {
					last = at
				}
			}
		}
		if found {
			items = append(items, memberLeaseView(item, last))
		}
	}
	return items, m.associationsTruncated
}
