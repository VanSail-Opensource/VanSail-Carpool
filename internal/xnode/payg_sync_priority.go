package xnode

import (
	"slices"
	"time"
)

type peerSyncJob struct {
	offset   int // Original round-robin offset; retained for interrupted-pass fairness.
	priority bool
	deadline time.Time
}

// Healthy peers already serving PAYG receive their next renewal before slow
// initial compiles. Ordinary peers keep the existing round-robin order. An
// urgent attempt receives priority only once per renewal interval: a failing
// peer can still retry normally without monopolizing the front of every pass.
func (m *Master) peerSyncOrder(peers []*agentPeer, start int, now time.Time, reportSeconds int) []peerSyncJob {
	urgent := make([]peerSyncJob, 0, len(peers))
	ordinary := make([]peerSyncJob, 0, len(peers))
	for offset := range peers {
		peer := peers[(start+offset)%len(peers)]
		job := peerSyncJob{offset: offset}
		m.mu.RLock()
		live := m.live[peer.nodeID]
		current := m.peers[peer.nodeID] == peer
		m.mu.RUnlock()
		peer.mu.Lock()
		ready, until, previous := peer.paygReady, peer.paygPublishedUntil, peer.paygPriorityAt
		peer.mu.Unlock()
		agentNow := paygAgentTime(now, live)
		if current && ready && live.report.Ready && live.report.Synchronized && live.firewallPending == "" &&
			freshSnapshot(live, now, reportSeconds) && !until.IsZero() &&
			!until.After(agentNow.Add(paygLeaseLifetime-paygRenewalInterval)) &&
			(previous.IsZero() || !now.Before(previous.Add(paygRenewalInterval))) {
			job.priority = true
			// Compare deadlines in the master's clock across nodes with different
			// bounded authenticated clock offsets. This never changes admission.
			job.deadline = now.Add(until.Sub(agentNow))
			urgent = append(urgent, job)
		} else {
			ordinary = append(ordinary, job)
		}
	}
	slices.SortStableFunc(urgent, func(a, b peerSyncJob) int { return a.deadline.Compare(b.deadline) })
	return append(urgent, ordinary...)
}
