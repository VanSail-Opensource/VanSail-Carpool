package hourly

import (
	"errors"
	"testing"
	"time"
)

func TestAllocatorCapacityRejectsWithoutEvictingLiveReplayEvidence(t *testing.T) {
	_, policy, peers, now := allocatorFixture(t, 2)
	a := New(now.Add(-MaxLifetime), Limits{MaxAccounts: 1, MaxPeers: 1, MaxDemandsPerAccount: 1, MaxNoncesPerAccount: 2})
	if err := a.Connect(now, peers[0]); err != nil {
		t.Fatal(err)
	}
	if err := a.Connect(now, peers[1]); !errors.Is(err, ErrCapacity) {
		t.Fatal("peer capacity exceeded", err)
	}
	feedAllocatorDemands(t, a, now, policy.Owner, peers[:1], 1, 1)
	a.Tick(now, []Policy{policy})
	feedAllocatorDemands(t, a, now.Add(time.Second), policy.Owner, peers[:1], 1, 2)
	third := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "ffffffffffffffffffffffffffffffff", ReceivedAt: now.Add(2 * time.Second), Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}
	if err := a.Observe(third.ReceivedAt, third); !errors.Is(err, ErrCapacity) {
		t.Fatal("nonce history evicted live evidence to accept another hint", err)
	}
	other := third
	other.Owner.ID = "second-owner"
	if err := a.Observe(other.ReceivedAt, other); !errors.Is(err, ErrCapacity) {
		t.Fatal("account capacity exceeded", err)
	}
	state := a.accounts[policy.Owner.ID]
	if len(state.seen) != 2 || len(state.demands) != 1 {
		t.Fatal("capacity rejection changed replay state")
	}
	later := now.Add(MaxLifetime + time.Second)
	third.ReceivedAt = later
	if err := a.Observe(later, third); err != nil {
		t.Fatal("expired nonce capacity was not reclaimed", err)
	}
}

func TestAllocatorRejectsClockRollbackAndFutureReceipt(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	grant := a.Tick(now, []Policy{policy})[0]
	if actions := a.Tick(now.Add(-time.Millisecond), []Policy{policy}); len(actions) != 0 {
		t.Fatal("clock rollback minted fresh actions")
	}
	ack := Ack{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}
	if err := a.Acknowledge(now.Add(-time.Millisecond), ack); !errors.Is(err, ErrStale) {
		t.Fatal("clock rollback changed reservations", err)
	}
	demand := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "ffffffffffffffffffffffffffffffff", ReceivedAt: now.Add(time.Second), Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}
	if err := a.Observe(now, demand); !errors.Is(err, ErrInvalid) {
		t.Fatal("future demand timestamp accepted", err)
	}
	if rate, burst := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 || burst != uint64(MaxBurstBytes) {
		t.Fatal("invalid clock input freed a live allocation")
	}
}

func TestAllocatorMissingInitialClockStillRequiresRestartQuiet(t *testing.T) {
	_, policy, peers, now := allocatorFixture(t, 1)
	a := New(time.Time{}, Limits{})
	if err := a.Connect(now, peers[0]); err != nil {
		t.Fatal(err)
	}
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	if actions := a.Tick(now.Add(MaxLifetime-time.Millisecond), []Policy{policy}); len(actions) != 0 {
		t.Fatal("zero constructor clock bypassed cold-start quiet")
	}
}
