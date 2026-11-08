package hourly

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAllocatorObserveBatchMatchesOrderedCallsAndOriginalReceipts(t *testing.T) {
	batched, policy, peers, now := allocatorFixture(t, 2)
	single, _, _, _ := allocatorFixture(t, 2)
	first := Demand{Owner: policy.Owner, Peer: peers[0], Nonce: "11111111111111111111111111111111", ReceivedAt: now.Add(-2 * time.Second), Lifetime: MaxLifetime, PendingBytes: 100, MinAtomicBytes: 1}
	duplicate := first
	duplicate.ReceivedAt = now.Add(-time.Second)
	invalid := first
	invalid.Nonce = "invalid"
	neighbor := first
	neighbor.Peer, neighbor.Nonce = peers[1], "22222222222222222222222222222222"
	newer := neighbor
	newer.Nonce, newer.ReceivedAt = "33333333333333333333333333333333", now.Add(-time.Second)
	replay := neighbor
	replay.ReceivedAt = now
	expired := first
	expired.ReceivedAt = now.Add(-MaxLifetime)
	future := first
	future.ReceivedAt = now.Add(time.Millisecond)
	demands := []Demand{first, invalid, duplicate, neighbor, newer, replay, expired, future}
	results, err := batched.ObserveBatch(now, demands)
	if err != nil || len(results) != len(demands) {
		t.Fatal("batch failed", results, err)
	}
	want := []error{nil, ErrInvalid, nil, nil, nil, ErrStale, ErrExpired, ErrInvalid}
	for i, demand := range demands {
		if !errors.Is(results[i], want[i]) || !errors.Is(single.Observe(now, demand), want[i]) {
			t.Fatalf("item %d changed ordered result: %v", i, results[i])
		}
	}
	if !reflect.DeepEqual(batched.accounts, single.accounts) {
		t.Fatal("batch changed demand/replay state")
	}
	grants := batched.Tick(now, []Policy{policy})
	if !reflect.DeepEqual(grants, single.Tick(now, []Policy{policy})) || len(grants) != 2 {
		t.Fatal("batch changed allocation actions")
	}
	if grants[0].ConservativeUntil != first.ReceivedAt.Add(MaxLifetime) {
		t.Fatal("duplicate demand refreshed its original deadline")
	}
}

func TestAllocatorAcknowledgeBatchMatchesOrderedCallsAndKeepsValidNeighbors(t *testing.T) {
	batched, policy, peers, now := allocatorFixture(t, 2)
	single, _, _, _ := allocatorFixture(t, 2)
	var initial, current []Grant
	for _, allocator := range []*Allocator{batched, single} {
		feedAllocatorDemands(t, allocator, now, policy.Owner, peers, 1, 1)
		initial = allocator.Tick(now, []Policy{policy})
		feedAllocatorDemands(t, allocator, now.Add(DecisionInterval), policy.Owner, peers, 1, 2)
		current = allocator.Tick(now.Add(DecisionInterval), []Policy{policy})
	}
	now = now.Add(DecisionInterval)
	ack := func(grant Grant, applied bool) Ack {
		return Ack{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: applied}
	}
	invalid := ack(current[1], true)
	invalid.Sequence = 0
	acks := []Ack{ack(initial[0], false), invalid, ack(current[0], true), ack(current[0], true), ack(initial[0], true), ack(current[1], true)}
	results, err := batched.AcknowledgeBatch(now, acks)
	if err != nil || len(results) != len(acks) {
		t.Fatal("batch failed", results, err)
	}
	want := []error{nil, ErrInvalid, nil, ErrStale, ErrStale, nil}
	for i, value := range acks {
		if !errors.Is(results[i], want[i]) || !errors.Is(single.Acknowledge(now, value), want[i]) {
			t.Fatalf("item %d changed ordered result: %v", i, results[i])
		}
	}
	if !reflect.DeepEqual(batched.accounts, single.accounts) || !reflect.DeepEqual(batched.Snapshot(now), single.Snapshot(now)) {
		t.Fatal("batch changed exact acknowledgment retention")
	}
}
