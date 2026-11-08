package hourly

import (
	"fmt"
	"testing"
	"time"
)

func allocatorFixture(t *testing.T, count int) (*Allocator, Policy, []Peer, time.Time) {
	t.Helper()
	start := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	a := New(start, Limits{})
	policy := Policy{Owner: Owner{ID: "8485399326", Generation: 1}, PaidThrough: start.Add(time.Hour), RateBytesPerSecond: 1000, BurstBytes: MaxBurstBytes}
	peers := make([]Peer, count)
	for i := range peers {
		peers[i] = Peer{NodeID: fmt.Sprintf("node-%02d", i), AgentID: fmt.Sprintf("%032x", i+1), BootID: fmt.Sprintf("%032x", i+101), SessionID: fmt.Sprintf("%032x", i+201)}
		if err := a.Connect(start, peers[i]); err != nil {
			t.Fatal(err)
		}
	}
	a.Tick(start, []Policy{policy})
	return a, policy, peers, start.Add(MaxLifetime)
}

func feedAllocatorDemands(t *testing.T, a *Allocator, at time.Time, owner Owner, peers []Peer, minimum uint32, epoch int) {
	t.Helper()
	for i, peer := range peers {
		demand := Demand{Owner: owner, Peer: peer, Nonce: fmt.Sprintf("%032x", epoch*1000+i+1), ReceivedAt: at, Lifetime: MaxLifetime, PendingBytes: uint64(max(minimum, 1)), MinAtomicBytes: minimum}
		if err := a.Observe(at, demand); err != nil {
			t.Fatal("demand rejected", err)
		}
	}
}

func ackAllocatorGrants(t *testing.T, a *Allocator, at time.Time, grants []Grant) {
	t.Helper()
	for _, grant := range grants {
		if err := a.Acknowledge(at, Ack{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}); err != nil {
			t.Fatal("grant acknowledgment rejected", err)
		}
	}
}

func sumAllocatorReservations(a *Allocator, at time.Time, id string) (uint64, uint64) {
	var rate, burst uint64
	for _, reservation := range a.Snapshot(at) {
		if reservation.Owner.ID == id {
			rate += reservation.RateBytesPerSecond
			burst += uint64(reservation.BurstBytes)
		}
	}
	return rate, burst
}

func assertAllocatorBudget(t *testing.T, a *Allocator, at time.Time, policy Policy) {
	t.Helper()
	rate, burst := sumAllocatorReservations(a, at, policy.Owner.ID)
	if rate > policy.RateBytesPerSecond || burst > uint64(policy.BurstBytes) {
		t.Fatalf("account overallocated rate=%d burst=%d", rate, burst)
	}
}
