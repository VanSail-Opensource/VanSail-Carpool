package hourly

import (
	"testing"
	"time"
)

func TestAllocatorDisconnectAndNewSessionRetainOldAllocationUntilExpiry(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	old := a.Tick(now, []Policy{policy})[0]
	a.Disconnect(now, peers[0])
	fresh := peers[0]
	fresh.SessionID = "ffffffffffffffffffffffffffffffff"
	if err := a.Connect(now, fresh); err != nil {
		t.Fatal(err)
	}
	now = now.Add(DecisionInterval)
	oldDemand := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}
	if err := a.Observe(now, oldDemand); err == nil {
		t.Fatal("old process/session demand crossed connection replacement")
	}
	feedAllocatorDemands(t, a, now, policy.Owner, []Peer{fresh}, 1, 2)
	for _, grant := range a.Tick(now, []Policy{policy}) {
		if grant.RateBytesPerSecond != 0 || grant.BurstBytes != 0 {
			t.Fatal("new session reclaimed an unexpired old allocation")
		}
	}
	if err := a.Acknowledge(now, Ack{Owner: old.Owner, Peer: old.Peer, Nonce: old.Nonce, Sequence: old.Sequence, Applied: true}); err == nil {
		t.Fatal("old session ACK crossed authenticated connection replacement")
	}
	if rate, _ := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 {
		t.Fatal("disconnect freed old lease")
	}
	now = old.ConservativeUntil
	feedAllocatorDemands(t, a, now, policy.Owner, []Peer{fresh}, 1, 3)
	actions := a.Tick(now, []Policy{policy})
	found := false
	for _, grant := range actions {
		found = found || grant.Peer == fresh && grant.RateBytesPerSecond == 1000
	}
	if !found {
		t.Fatal("expiry did not release retained allocation to new session")
	}
	assertAllocatorBudget(t, a, now, policy)
}

func TestAllocatorOwnerGenerationChangesShareOldReservationBudget(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	old := a.Tick(now, []Policy{policy})[0]
	policy.Owner.Generation++
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	for _, grant := range a.Tick(now, []Policy{policy}) {
		if grant.RateBytesPerSecond != 0 || grant.BurstBytes != 0 {
			t.Fatal("new owner generation minted a second full account budget")
		}
	}
	if rate, _ := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 {
		t.Fatal("old generation reservation disappeared")
	}
	if err := a.Observe(now, Demand{Owner: old.Owner, Peer: old.Peer, Nonce: "ffffffffffffffffffffffffffffffff", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}); err == nil {
		t.Fatal("superseded owner generation admitted fresh demand")
	}
	now = old.ConservativeUntil
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 3)
	found := false
	for _, grant := range a.Tick(now, []Policy{policy}) {
		found = found || grant.Owner == policy.Owner && grant.RateBytesPerSecond == 1000
	}
	if !found {
		t.Fatal("expired old generation prevented fresh allocation")
	}
	assertAllocatorBudget(t, a, now, policy)
}

func TestAllocatorRestartQuietAndPolicyReductionNeverGrantFreshExcess(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 2)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	initial := a.Tick(now, []Policy{policy})
	ackAllocatorGrants(t, a, now, initial)
	restarted := New(now, Limits{})
	for _, peer := range peers {
		if err := restarted.Connect(now, peer); err != nil {
			t.Fatal(err)
		}
	}
	feedAllocatorDemands(t, restarted, now, policy.Owner, peers, 1, 2)
	if actions := restarted.Tick(now.Add(MaxLifetime-time.Millisecond), []Policy{policy}); len(actions) != 0 {
		t.Fatal("restarted controller restored an untracked burst")
	}
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	policy.RateBytesPerSecond, policy.BurstBytes = 100, 1024
	for _, grant := range a.Tick(now, []Policy{policy}) {
		if grant.RateBytesPerSecond > 50 || grant.BurstBytes > 512 {
			t.Fatal("policy reduction increased a live allocation")
		}
	}
	rate, burst := sumAllocatorReservations(a, now, policy.Owner.ID)
	if rate != 1000 || burst != uint64(MaxBurstBytes) {
		t.Fatal("reduction silently freed undrained old reservations")
	}
}
