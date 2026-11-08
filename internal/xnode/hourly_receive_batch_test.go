package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyCoalescedDemandBatchKeepsEachFirstSocketReceipt(t *testing.T) {
	m, peer, repo, now := masterHourlyRateFixture(t)
	second := repo.policies[0]
	second.OwnerID = "other-owner"
	repo.policies = append(repo.policies, second)
	m.refreshHourlyPolicies(context.Background())
	first := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	other := first
	other.Owner.ID, other.Nonce = second.OwnerID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	invalid := other
	invalid.Nonce = "invalid"
	inputs := []hourlyInboundDemand{
		{demand: first, receivedAt: now.Add(-9 * time.Second)},
		{demand: invalid, receivedAt: *now},
		{demand: other, receivedAt: now.Add(-2 * time.Second)},
		{demand: first, receivedAt: *now},
	}
	m.observeHourlyDemands(peer, inputs)
	grants := m.hourly.allocator.Tick(*now, m.hourly.allocatorPolicies(*now))
	if len(grants) != 2 {
		t.Fatal("invalid neighbor discarded valid coalesced demands", len(grants))
	}
	for _, grant := range grants {
		expected := now.Add(time.Second)
		if grant.Owner.ID == second.OwnerID {
			expected = now.Add(8 * time.Second)
		}
		if !grant.ConservativeUntil.Equal(expected) {
			t.Fatal("coalescing replaced an owner's original receipt", grant.Owner.ID, grant.ConservativeUntil, expected)
		}
	}
}
