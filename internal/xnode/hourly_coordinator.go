package xnode

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
)

type hourlyCoordinator struct {
	allocatorMu sync.Mutex // serialize caller clock capture with allocator admission
	allocator   *hourly.Allocator
	policyMu    sync.RWMutex
	policies    map[string]HourlyAccountPolicy
	policyAt    time.Time
}

func newHourlyCoordinator(now time.Time) *hourlyCoordinator {
	return &hourlyCoordinator{allocator: hourly.New(now, hourly.Limits{}), policies: map[string]HourlyAccountPolicy{}}
}

func (m *Master) runHourly() {
	defer m.wg.Done()
	ticker := time.NewTicker(hourly.DecisionInterval)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		m.refreshHourlyPolicies(ctx)
		cancel()
		co := m.hourly
		co.allocatorMu.Lock()
		now := m.service.now()
		grants := co.allocator.Tick(now, co.allocatorPolicies(now))
		co.allocatorMu.Unlock()
		m.enqueueHourlyGrants(grants)
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// HourlyCapable checks negotiation for any participating source or transit node.
// It does not authorize commerce or infer compatibility from an agent version.
func (m *Master) HourlyCapable(nodeID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer := m.peers[nodeID]
	return m.hourly != nil && peer != nil && peer.hourlySession != "" && peer.ctx != nil && peer.ctx.Err() == nil
}

func (m *Master) currentHourlyPeer(peer *agentPeer) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hourly != nil && m.peers[peer.nodeID] == peer && peer.hourlySession != "" && peer.ctx != nil && peer.ctx.Err() == nil
}

func hourlyPeer(peer *agentPeer) hourly.Peer {
	return hourly.Peer{NodeID: peer.nodeID, AgentID: peer.agentID, BootID: peer.bootID, SessionID: peer.hourlySession}
}
