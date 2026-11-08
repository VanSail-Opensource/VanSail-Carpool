package hourly

import (
	"errors"
	"testing"
	"time"
)

func TestAllocatorBatchBoundsAndEmptyInputDoNotAdvanceOrPrune(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	grant := a.Tick(now, []Policy{policy})[0]
	later := now.Add(MaxLifetime)
	if result, err := a.ObserveBatch(later, make([]Demand, MaxBatchItems+1)); !errors.Is(err, ErrInvalid) || result != nil {
		t.Fatal("oversized demand batch accepted", err)
	}
	if result, err := a.AcknowledgeBatch(later, make([]Ack, MaxBatchItems+1)); !errors.Is(err, ErrInvalid) || result != nil {
		t.Fatal("oversized acknowledgment batch accepted", err)
	}
	if _, err := a.ObserveBatch(later, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AcknowledgeBatch(later, nil); err != nil {
		t.Fatal(err)
	}
	if !a.lastNow.Equal(now) || len(a.accounts[policy.Owner.ID].allocations) != 1 {
		t.Fatal("rejected/empty batches advanced time or pruned live state")
	}
	acks := []Ack{{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}}
	results, err := a.AcknowledgeBatch(later, acks)
	if err != nil || !errors.Is(results[0], ErrStale) || len(a.accounts[policy.Owner.ID].allocations) != 0 {
		t.Fatal("valid batch did not prune exactly at expiry", results, err)
	}
}

func TestAllocatorBatchClockRollbackRejectsEveryItemWithoutMutation(t *testing.T) {
	a, policy, peers, now := allocatorFixture(t, 1)
	feedAllocatorDemands(t, a, now, policy.Owner, peers, 1, 1)
	grant := a.Tick(now, []Policy{policy})[0]
	before := now.Add(-time.Millisecond)
	demands := []Demand{{Owner: policy.Owner, Peer: peers[0], Nonce: "invalid"}, {Owner: policy.Owner, Peer: peers[0], Nonce: "22222222222222222222222222222222", ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}}
	results, err := a.ObserveBatch(before, demands)
	if err != nil || len(results) != 2 || !errors.Is(results[0], ErrStale) || !errors.Is(results[1], ErrStale) {
		t.Fatal("rollback demand batch changed per-item ordering", results, err)
	}
	acks := []Ack{{}, {Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}}
	results, err = a.AcknowledgeBatch(before, acks)
	if err != nil || len(results) != 2 || !errors.Is(results[0], ErrStale) || !errors.Is(results[1], ErrStale) {
		t.Fatal("rollback acknowledgment batch changed per-item ordering", results, err)
	}
	if !a.lastNow.Equal(now) || len(a.accounts[policy.Owner.ID].seen) != 1 || a.accounts[policy.Owner.ID].allocations[allocationKey{policy.Owner, peers[0]}].lastAck != 0 {
		t.Fatal("rollback batch changed live state")
	}
}
