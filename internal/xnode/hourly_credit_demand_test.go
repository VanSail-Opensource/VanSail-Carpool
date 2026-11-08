package xnode

import (
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyColdZeroHintCannotRestartCompletedScan(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	// The cached authoritative policy is already loaded. The production contract
	// is present; this hint admission path must not call any SQL method.
	m.service.repo = &hourlyIntentRepository{}
	c := m.hourlyCredits
	m.requestHourlyCredit(peer, nil)
	c.mu.Lock()
	state := c.peers[peer]
	state.scanned, state.work, state.running = true, false, true
	c.workers = 1
	c.activeAgents[peer.agentID] = peer
	c.mu.Unlock()
	c.finished(peer, false, false, *now)
	if c.peers[peer] != nil {
		t.Fatal("completed empty initial scan did not release its disposable state")
	}
	zero := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: masterHourlySession, LifetimeMillis: 10000}
	unknown := zero
	unknown.Owner.ID, unknown.PendingBytes, unknown.MinAtomicBytes = "unknown-owner", 1, 1
	m.requestHourlyCredit(peer, []wire.HourlyDemand{zero, unknown})
	if c.peers[peer] != nil || len(c.ready(*now)) != 0 {
		t.Fatal("zero or unauthoritative positive hint restarted a cold SQL scan")
	}
	positive := zero
	positive.PendingBytes, positive.MinAtomicBytes = 1, 1
	m.requestHourlyCredit(peer, []wire.HourlyDemand{positive})
	state = c.peers[peer]
	if state == nil || !state.work || state.hints[positive.Owner].demand.PendingBytes != 1 {
		t.Fatal("eligible positive demand could not restart credit admission")
	}
	*now = now.Add(time.Second)
	m.requestHourlyCredit(peer, []wire.HourlyDemand{zero})
	if state.hints[zero.Owner].demand.PendingBytes != 0 || !hourlyCreditHasDemand(state, *now) {
		t.Fatal("existing zero update erased recent confirmed/pending activity")
	}
	// Nil reconnect/financial reconciliation must remain able to create cold
	// work even when no live payload is pending and no hint is eligible.
	c.mu.Lock()
	c.hintCount -= len(state.hints)
	delete(c.peers, peer)
	c.mu.Unlock()
	m.requestHourlyCredit(peer, nil)
	if state = c.peers[peer]; state == nil || !state.work {
		t.Fatal("cold financial/reconnect wake was suppressed with zero hints")
	}
}
