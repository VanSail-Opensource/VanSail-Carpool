package hourly

import (
	"testing"
	"time"
)

func TestAllocatorColdStartOneNodeFullAndHintCadence(t *testing.T) {
	a, policy, peers, ready := allocatorFixture(t, 2)
	feedAllocatorDemands(t, a, ready.Add(-time.Millisecond), policy.Owner, peers[:1], 1, 1)
	if grants := a.Tick(ready.Add(-time.Millisecond), []Policy{policy}); len(grants) != 0 {
		t.Fatal("cold restart minted grants before old leases expired")
	}
	grants := a.Tick(ready, []Policy{policy})
	if len(grants) != 1 || grants[0].RateBytesPerSecond != policy.RateBytesPerSecond || grants[0].BurstBytes != MaxBurstBytes {
		t.Fatal("one demanding node did not receive the whole account allocation")
	}
	ackAllocatorGrants(t, a, ready, grants)
	feedAllocatorDemands(t, a, ready.Add(time.Millisecond), policy.Owner, peers, 1, 2)
	if actions := a.Tick(ready.Add(time.Millisecond), []Policy{policy}); len(actions) != 0 {
		t.Fatal("immediate hint increased allocation before decision cadence")
	}
	assertAllocatorBudget(t, a, ready.Add(time.Millisecond), policy)
}

func TestAllocatorShrinksBeforeIncreaseAndWaitsForExactDrainAck(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 2)
	feedAllocatorDemands(t, a, now, policy.Owner, peers[:1], 1, 1)
	initial := a.Tick(now, []Policy{policy})
	ackAllocatorGrants(t, a, now, initial)
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	shrink := a.Tick(now, []Policy{policy})
	var shrinking Grant
	for _, grant := range shrink {
		if grant.Peer == peers[0] && grant.RateBytesPerSecond == 500 {
			shrinking = grant
		}
		if grant.Peer == peers[1] && (grant.RateBytesPerSecond != 0 || grant.BurstBytes != 0) {
			t.Fatal("new node increased before old grant drained")
		}
	}
	if shrinking.Sequence == 0 {
		t.Fatal("old node did not receive a shrink")
	}
	rate, burst := sumAllocatorReservations(a, now, policy.Owner.ID)
	if rate != 1000 || burst != uint64(MaxBurstBytes) {
		t.Fatal("unacknowledged shrink freed old allocation")
	}
	bad := Ack{Owner: shrinking.Owner, Peer: shrinking.Peer, Nonce: initial[0].Nonce, Sequence: shrinking.Sequence, Applied: true}
	if err := a.Acknowledge(now, bad); err == nil {
		t.Fatal("mismatched drain acknowledgment accepted")
	}
	ackAllocatorGrants(t, a, now, shrink)
	if actions := a.Tick(now.Add(time.Millisecond), []Policy{policy}); len(actions) != 0 {
		t.Fatal("ACK bypassed two-second decision cadence")
	}
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 3)
	actions := a.Tick(now, []Policy{policy})
	positive := 0
	for _, grant := range actions {
		if grant.RateBytesPerSecond == 500 && grant.BurstBytes == 8192 {
			positive++
		}
	}
	if positive != 2 {
		t.Fatal("drained allocation did not resume equal sharing")
	}
	assertAllocatorBudget(t, a, now, policy)
	if err := a.Acknowledge(now, Ack{Owner: initial[0].Owner, Peer: initial[0].Peer, Nonce: initial[0].Nonce, Sequence: initial[0].Sequence, Applied: true}); err == nil {
		t.Fatal("reordered acknowledgment restored an old grant")
	}
}

func TestAllocatorZeroRateGrantCanAuthorizeReservedFullSpeedCredit(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 2)
	policy.RateBytesPerSecond, policy.BurstBytes = 0, 0
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	grants := a.Tick(now, []Policy{policy})
	if len(grants) != 2 {
		t.Fatal("full-speed-only peers received no authorization grants")
	}
	for _, grant := range grants {
		if grant.RateBytesPerSecond != 0 || grant.BurstBytes != 0 {
			t.Fatal("credit-only grant minted a reduced-rate burst")
		}
	}
	assertAllocatorBudget(t, a, now, policy)
}

func TestAllocatorRecentZeroPendingDemandKeepsBusyFullSpeedFlowRateStable(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	initial := a.Tick(now, []Policy{policy})
	ackAllocatorGrants(t, a, now, initial)
	now = now.Add(DecisionInterval)
	recent := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "ffffffffffffffffffffffffffffffff", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 0, MinAtomicBytes: 0}
	if err := a.Observe(now, recent); err != nil {
		t.Fatal(err)
	}
	refreshed := a.Tick(now, []Policy{policy})
	if len(refreshed) != 1 || refreshed[0].RateBytesPerSecond != policy.RateBytesPerSecond || refreshed[0].BurstBytes != policy.BurstBytes {
		t.Fatal("recent full-speed flow was gratuitously shrunk because no payload was waiting")
	}
	ackAllocatorGrants(t, a, now, refreshed)
	now = refreshed[0].ConservativeUntil
	if grants := a.Tick(now, []Policy{policy}); len(grants) != 0 {
		t.Fatal("idle flow gained renewed capacity without a fresh recent demand")
	}
	if rate, burst := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 0 || burst != 0 {
		t.Fatal("expired recent activity retained account budget")
	}
}
