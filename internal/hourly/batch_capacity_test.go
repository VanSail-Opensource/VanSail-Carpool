package hourly

import (
	"fmt"
	"testing"
)

func TestAllocatorMaximumBatchAdmitsAndAcknowledgesEveryOwner(t *testing.T) {
	a, base, peers, now := allocatorFixture(t, 1)
	demands := make([]Demand, MaxBatchItems)
	policies := make([]Policy, MaxBatchItems)
	for i := range demands {
		policies[i] = base
		policies[i].Owner.ID = fmt.Sprintf("owner-%03d", i)
		demands[i] = Demand{Owner: policies[i].Owner, Peer: peers[0], Nonce: fmt.Sprintf("%032x", i+1), ReceivedAt: now, Lifetime: MaxLifetime, PendingBytes: 100, MinAtomicBytes: 1}
	}
	results, err := a.ObserveBatch(now, demands)
	if err != nil || len(results) != MaxBatchItems {
		t.Fatal("maximum demand batch rejected", err)
	}
	for _, result := range results {
		if result != nil {
			t.Fatal("maximum demand batch dropped an owner", result)
		}
	}
	grants := a.Tick(now, policies)
	if len(grants) != MaxBatchItems {
		t.Fatal("maximum batch lost grant actions", len(grants))
	}
	acks := make([]Ack, len(grants))
	for i, grant := range grants {
		acks[i] = Ack{Owner: grant.Owner, Peer: grant.Peer, Nonce: grant.Nonce, Sequence: grant.Sequence, Applied: true}
	}
	results, err = a.AcknowledgeBatch(now, acks)
	if err != nil || len(results) != MaxBatchItems {
		t.Fatal("maximum acknowledgment batch rejected", err)
	}
	for _, result := range results {
		if result != nil {
			t.Fatal("maximum acknowledgment batch dropped an owner", result)
		}
	}
}
