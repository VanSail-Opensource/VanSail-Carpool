package hourly

import (
	"errors"
	"testing"
	"time"
)

func TestAllocatorDuplicateNonceCannotRefreshDeadlineOrReuseSupersededDemand(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	demand := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "11111111111111111111111111111111", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 10, MinAtomicBytes: 1}
	if err := a.Observe(now, demand); err != nil {
		t.Fatal(err)
	}
	grant := a.Tick(now, []Policy{policy})[0]
	later := now.Add(time.Second)
	repeated := demand
	repeated.ReceivedAt = later
	if err := a.Observe(later, repeated); err != nil {
		t.Fatal("idempotent hint rejected", err)
	}
	if got := a.accounts[policy.Owner.ID].demands[demandKey{policy.Owner, peers[0].NodeID}].ReceivedAt; !got.Equal(now) {
		t.Fatal("duplicate hint refreshed original receipt")
	}
	newer := demand
	newer.ReceivedAt, newer.Nonce = later, "22222222222222222222222222222222"
	if err := a.Observe(later, newer); err != nil {
		t.Fatal(err)
	}
	repeated.ReceivedAt = later.Add(time.Millisecond)
	if err := a.Observe(later.Add(time.Millisecond), repeated); !errors.Is(err, ErrStale) {
		t.Fatal("superseded nonce became a fresh demand", err)
	}
	late := grant.ConservativeUntil.Add(time.Millisecond)
	if err := a.Observe(late, demand); !errors.Is(err, ErrExpired) {
		t.Fatal("old ordered demand revived after original expiry", err)
	}
	if rate, burst := sumAllocatorReservations(a, late, policy.Owner.ID); rate != 0 || burst != 0 {
		t.Fatal("expired reservation remained usable")
	}
	if err := a.Acknowledge(late, Ack{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}); err == nil {
		t.Fatal("expired grant acknowledgment accepted")
	}
}

func TestAllocatorLateSameNonceDoesNotRefreshAfterExpiry(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	demand := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "11111111111111111111111111111111", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 10, MinAtomicBytes: 1}
	if err := a.Observe(now, demand); err != nil {
		t.Fatal(err)
	}
	a.Tick(now, []Policy{policy})
	later := now.Add(MaxLifetime + time.Second)
	demand.ReceivedAt = later
	if err := a.Observe(later, demand); !errors.Is(err, ErrExpired) {
		t.Fatal("duplicate nonce gained another lifetime after expiry", err)
	}
}

func TestAllocatorWrongAndReorderedAcknowledgmentsCannotReleaseNewerOffers(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	initial := a.Tick(now, []Policy{policy})[0]
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 2)
	reduced := policy
	reduced.RateBytesPerSecond = 100
	shrink := a.Tick(now, []Policy{reduced})[0]
	now = now.Add(DecisionInterval)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 3)
	larger := a.Tick(now, []Policy{policy})[0]
	if larger.Sequence <= shrink.Sequence || larger.RateBytesPerSecond != 1000 {
		t.Fatal("fixture did not issue a newer offer")
	}
	if err := a.Acknowledge(now, Ack{Owner: shrink.Owner, Peer: shrink.Peer, Nonce: shrink.Nonce, Sequence: shrink.Sequence, Applied: true}); err != nil {
		t.Fatal(err)
	}
	if rate, _ := sumAllocatorReservations(a, now, policy.Owner.ID); rate != 1000 {
		t.Fatal("old shrink ACK freed newer possibly-applied allocation")
	}
	if err := a.Acknowledge(now, Ack{Owner: initial.Owner, Peer: initial.Peer, Nonce: initial.Nonce, Sequence: initial.Sequence, Applied: true}); err == nil {
		t.Fatal("reordered ACK accepted")
	}
	wrong := Ack{Owner: larger.Owner, Peer: larger.Peer, Nonce: larger.Nonce, Sequence: larger.Sequence, Applied: true}
	wrong.Owner.Generation++
	if err := a.Acknowledge(now, wrong); err == nil {
		t.Fatal("cross-generation ACK released allocation")
	}
	ackAllocatorGrants(t, a, now, []Grant{larger})
}

func TestAllocatorPaidBoundaryAndUTCMidnightCapOriginalLifetime(t *testing.T) {
	start := time.Date(2026, 11, 9, 23, 59, 40, 0, time.UTC)
	a := New(start, Limits{})
	peer := Peer{NodeID: "node", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222", SessionID: "33333333333333333333333333333333"}
	if err := a.Connect(start, peer); err != nil {
		t.Fatal(err)
	}
	policy := Policy{Owner: Owner{ID: "ordinary-user", Generation: 1}, PaidThrough: start.Add(time.Minute), RateBytesPerSecond: 1000, BurstBytes: MaxBurstBytes}
	now := start.Add(19 * time.Second)
	feedAllocatorDemands(t, a, now, policy.Owner, []Peer{peer}, 1, 1)
	grant := a.Tick(now, []Policy{policy})[0]
	if grant.Lifetime != time.Second || grant.DayUTC != "2026-11-09" || grant.ConservativeUntil.UTC().Hour() != 0 {
		t.Fatal("grant crossed UTC midnight")
	}
	now = now.Add(DecisionInterval)
	policy.PaidThrough = now.Add(1500 * time.Microsecond)
	feedAllocatorDemands(t, a, now, policy.Owner, []Peer{peer}, 1, 2)
	grant = a.Tick(now, []Policy{policy})[0]
	if grant.Lifetime != time.Millisecond || grant.ConservativeUntil.After(policy.PaidThrough) || grant.DayUTC != "2026-11-10" {
		t.Fatal("paid boundary was rounded upward or old UTC day retained")
	}
}

func TestAllocatorDelayedDemandCannotCarryOriginalNonceAcrossMidnight(t *testing.T) {
	start := time.Date(2026, 11, 9, 23, 59, 40, 0, time.UTC)
	a := New(start, Limits{})
	peer := Peer{NodeID: "node", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222", SessionID: "33333333333333333333333333333333"}
	if err := a.Connect(start, peer); err != nil {
		t.Fatal(err)
	}
	policy := Policy{Owner: Owner{ID: "ordinary-user", Generation: 1}, PaidThrough: start.Add(time.Minute), RateBytesPerSecond: 1000, BurstBytes: MaxBurstBytes}
	received := start.Add(19 * time.Second)
	feedAllocatorDemands(t, a, received, policy.Owner, []Peer{peer}, 1, 1)
	if grants := a.Tick(received.Add(2*time.Second), []Policy{policy}); len(grants) != 0 {
		t.Fatal("delayed original demand acquired a fresh UTC day grant")
	}
}
