package xnode

import (
	"context"
	"slices"
)

// Connection observations stay memory-only. The existing reconciler performs
// the small idempotent debit; the WebSocket reader never awaits a node command.
func (m *Master) observePAYG(ctx context.Context, peers []*agentPeer, reportSeconds int, sampleSeconds ...int) error {
	r, ok := m.service.paygRepository()
	if !ok {
		return nil
	}
	for _, peer := range peers {
		peer.mu.Lock()
		ready := peer.paygReady
		peer.mu.Unlock()
		if !ready {
			continue
		}
		m.mu.RLock()
		live := m.live[peer.nodeID]
		current := m.peers[peer.nodeID] == peer
		m.mu.RUnlock()
		// Repeated reports do not renew an old activity sample. A replaced
		// peer's recovery state cannot authorize observations from its successor.
		if !current || !activityAvailable(live, m.service.now(), reportSeconds, sampleSeconds...) {
			continue
		}
		connected := []int64{}
		for id, count := range live.report.Users.Connections {
			if count > 0 {
				connected = append(connected, int64(id))
			}
		}
		if len(connected) == 0 {
			continue
		}
		accounts, err := connectedPAYGAccounts(ctx, r, peer.nodeID, connected)
		if err != nil {
			return err
		}
		ids := []int64{}
		for _, a := range accounts {
			if a.Disabled || a.HasCredit(m.service.now()) || a.BalanceMinor < a.UnitPriceMinor || live.report.Users.Connections[int(a.ProxyUserID)] == 0 {
				continue
			}
			coverage, e := m.service.paygCoverage(ctx, a.UserID)
			if e != nil {
				return e
			}
			if coverage[a.GroupID] != "" {
				continue
			}
			ids = append(ids, a.ProxyUserID)
		}
		if len(ids) == 0 {
			continue
		}
		// Account/coverage reads can outlive the activity that justified them.
		// Recheck the latest authenticated evidence just before the debit.
		now := m.service.now().UTC()
		m.mu.RLock()
		latest := m.live[peer.nodeID]
		current = m.peers[peer.nodeID] == peer
		m.mu.RUnlock()
		if !current || !activityAvailable(latest, now, reportSeconds, sampleSeconds...) {
			continue
		}
		ids = slices.DeleteFunc(ids, func(id int64) bool { return latest.report.Users.Connections[int(id)] <= 0 })
		if len(ids) > 0 {
			if err = r.ObserveXNodePAYGConnections(ctx, peer.nodeID, ids, now); err != nil {
				return err
			}
		}
	}
	return nil
}
