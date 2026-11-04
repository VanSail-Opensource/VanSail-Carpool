package xnode

import (
	"container/heap"
	"time"
)

type automaticLeaseHeap []*automaticLease

func (h automaticLeaseHeap) Len() int           { return len(h) }
func (h automaticLeaseHeap) Less(i, j int) bool { return h[i].until.Before(h[j].until) }
func (h automaticLeaseHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *automaticLeaseHeap) Push(value any) {
	item := value.(*automaticLease)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *automaticLeaseHeap) Pop() any {
	items := *h
	last := len(items) - 1
	item := items[last]
	items[last], item.index = nil, -1
	*h = items[:last]
	return item
}

func (m *automaticLeaseManager) expireLocked(now time.Time) {
	for len(m.expiry) > 0 && !m.expiry[0].until.After(now) {
		item := heap.Pop(&m.expiry).(*automaticLease)
		m.ownerAssociations -= len(item.owners)
		delete(m.leases, item.ip)
		delete(m.pendingBans, item.ip)
		m.expired++
		m.revision++
		m.snapshot, m.snapshotAt = nil, 0
	}
}

func (m *automaticLeaseManager) expire(now time.Time) {
	m.mu.Lock()
	m.expireLocked(now)
	m.mu.Unlock()
}
