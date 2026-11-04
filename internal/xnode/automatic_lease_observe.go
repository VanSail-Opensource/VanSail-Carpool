package xnode

import (
	"container/heap"
	"math"
	"time"
)

func (m *automaticLeaseManager) observe(observations []AutomaticLeaseObservation, now time.Time, freshness time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	if !m.config.Enabled && len(m.leases) == 0 {
		return false
	}
	duration := time.Duration(m.config.DurationHours) * time.Hour
	changed := false
	for _, observation := range observations {
		if observation.Node == "" || observation.At.IsZero() || observation.At.After(now.Add(30*time.Second)) || observation.At.Before(now.Add(-freshness)) {
			continue
		}
		ip, err := firewallPublicAddr(observation.IP)
		if err != nil {
			continue
		}
		item := m.leases[ip]
		if item == nil {
			if !m.config.Enabled {
				continue
			}
			if !observation.At.Add(duration).After(now) {
				continue
			}
			if len(m.leases) >= maximumAutomaticLeases || m.generation == math.MaxUint64 {
				m.rejected++
				continue
			}
			item = &automaticLease{ip: ip, first: observation.At, last: observation.At, started: observation.At, until: observation.At.Add(duration)}
			m.leases[ip] = item
			heap.Push(&m.expiry, item)
			m.created++
			m.grantLocked(item)
			changed = true
		} else if !item.observeSource(observation.Node, observation.At) {
			continue
		} else if m.config.Enabled && !observation.At.Before(item.started.Add(item.until.Sub(item.started)/2)) && observation.At.Add(2*duration).After(now) && m.generation < math.MaxUint64 {
			item.started, item.until = observation.At, observation.At.Add(2*duration)
			at := observation.At
			item.renewed = &at
			item.renewals++
			m.renewed++
			heap.Fix(&m.expiry, item.index)
			m.grantLocked(item)
			changed = true
		}
		if item.observations == 0 {
			item.observeSource(observation.Node, observation.At)
		}
		if observation.At.Before(item.first) {
			item.first = observation.At
		}
		if observation.At.After(item.last) {
			item.last = observation.At
		}
		if item.observations < math.MaxUint64 {
			item.observations++
		}
	}
	return changed
}

func (m *automaticLeaseManager) grantLocked(item *automaticLease) {
	m.generation++
	item.generation = m.generation
	m.pendingBans[item.ip] = item.generation
	m.revision++
}

// Source cardinality is exact up to eight nodes; the API explicitly marks
// saturation. This also fences duplicate/reconnected reports without an
// unbounded per-IP/per-node registry or any durable observation writes.
func (item *automaticLease) observeSource(node string, at time.Time) bool {
	for i := range item.sourceCount {
		if item.sources[i].node == node {
			if !at.After(item.sources[i].last) {
				return false
			}
			item.sources[i].last = at
			return true
		}
	}
	if item.sourceCount < maximumLeaseSources {
		item.sources[item.sourceCount] = automaticLeaseSource{node, at}
		item.sourceCount++
		return true
	}
	item.sourcesTruncated = true
	return at.After(item.last)
}
