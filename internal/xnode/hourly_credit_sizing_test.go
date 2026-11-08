package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyCreditSizingAdaptsBeyond64MiBAndSharesDemandingNodes(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	state.hints[owner] = hourlyCreditHint{seenAt: *now, demand: wire.HourlyDemand{PendingBytes: 1}}
	state.observed["credit"] = hourlyCreditObserved{at: *now}
	state.roundTrip = 100 * time.Millisecond
	c.mu.Unlock()
	*now = now.Add(100 * time.Millisecond)
	m.observeHourlyCreditProgress(peer, []model.HourlyCreditReservation{{HourlyCreditRequest: model.HourlyCreditRequest{ReservationID: "credit", OwnerID: "owner", AccountGeneration: 1}, AdmittedUploadBytes: hourlyCreditQuantum}})
	const pool int64 = 1 << 30
	target := m.hourlyCreditTarget(peer, owner, pool)
	if target <= hourlyCreditQuantum || target > pool {
		t.Fatal("fixed startup quantum capped sustained full-speed admission", target)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	other := &agentPeer{nodeID: "other", agentID: "other-agent", ctx: ctx}
	c.mu.Lock()
	c.stateLocked(other, *now).hints[owner] = hourlyCreditHint{seenAt: *now, demand: wire.HourlyDemand{PendingBytes: 1}}
	c.mu.Unlock()
	if shared := m.hourlyCreditTarget(peer, owner, pool); shared > pool/2 || shared <= hourlyCreditQuantum {
		t.Fatal("active node consumed another demanding node's share", shared)
	}
	*now = now.Add(hourlyCreditDemandFreshness + time.Nanosecond)
	if cold := m.hourlyCreditTarget(peer, owner, pool); cold != hourlyCreditQuantum {
		t.Fatal("expired live throughput estimate retained extra authority", cold)
	}
}

func TestHourlyCreditWorkersSerializePersistentAgentsAndBackOffOnlyFailures(t *testing.T) {
	c := newHourlyCreditCoordinator()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now()
	first := &agentPeer{nodeID: "a", agentID: "one-agent", ctx: ctx}
	second := &agentPeer{nodeID: "b", agentID: "one-agent", ctx: ctx}
	c.mu.Lock()
	c.stateLocked(first, now).work = true
	c.stateLocked(second, now).work = true
	c.mu.Unlock()
	ready := c.ready(now)
	if len(ready) != 1 || len(c.ready(now)) != 0 {
		t.Fatal("two workers could issue reordered credit to one persistent agent")
	}
	c.finished(ready[0], false, false, now)
	if len(c.ready(now)) != 1 {
		t.Fatal("successful pass imposed a fixed one-second throughput limit")
	}
	var active *agentPeer
	c.mu.Lock()
	active = c.activeAgents["one-agent"]
	c.mu.Unlock()
	c.finished(active, false, true, now)
	c.mu.Lock()
	retry := c.peers[active].retryAt
	c.mu.Unlock()
	if !retry.After(now) {
		t.Fatal("failed RPC lost bounded retry backoff")
	}
}

func TestHourlyProgressRefillsLowWaterWithoutPendingPayload(t *testing.T) {
	m, peer, _, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.stateLocked(peer, *now)
	state.hints[owner] = hourlyCreditHint{seenAt: *now}
	state.observed["credit"] = hourlyCreditObserved{at: *now}
	c.mu.Unlock()
	if owners := m.hourlyCreditOwners(peer, 4); len(owners) != 0 {
		t.Fatal("zero-demand keepalive alone prefetched credit", owners)
	}
	*now = now.Add(100 * time.Millisecond)
	r := model.HourlyCreditReservation{HourlyCreditRequest: model.HourlyCreditRequest{ReservationID: "credit", OwnerID: owner.ID, AccountGeneration: 1, DayUTC: now.UTC().Format(time.DateOnly), BootID: peer.bootID, SessionID: peer.hourlySession, ByteCeiling: 1000}, AdmittedUploadBytes: 700}
	m.observeHourlyCreditProgress(peer, []model.HourlyCreditReservation{r})
	if owners := m.hourlyCreditOwners(peer, 4); len(owners) != 1 || owners[0] != owner {
		t.Fatal("busy full-speed owner waited for a blocked payload before refill", owners)
	}
	*now = now.Add(hourlyCreditIdleAfter + time.Nanosecond)
	m.refreshHourlyPolicies(context.Background())
	c.mu.Lock()
	hint := state.hints[owner]
	hint.seenAt = *now // Fresh zero-pending keepalive, no new admitted progress.
	state.hints[owner] = hint
	c.mu.Unlock()
	seal, _ := m.hourlyCreditAction(peer, r)
	if !seal {
		t.Fatal("zero keepalives indefinitely retained idle full-speed credit")
	}
}
