package xnode

import (
	"context"
)

// Connection observations stay memory-only. The existing reconciler performs
// the small idempotent debit; the WebSocket reader never awaits a node command.
func (m *Master) observePAYG(ctx context.Context, peers []*agentPeer, reportSeconds int) error {
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
		m.mu.RUnlock()
		if !freshSnapshot(live, m.service.now(), reportSeconds) || !live.report.Synchronized {
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
		if len(ids) > 0 {
			if err = r.ObserveXNodePAYGConnections(ctx, peer.nodeID, ids, m.service.now().UTC()); err != nil {
				return err
			}
		}
	}
	return nil
}
