package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestBufferedHourlyHintCannotPopulateAnOwnerAfterPolicyRemoval(t *testing.T) {
	m, peer, repo, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	if _, eligible := m.hourlyPolicy(owner, peer.nodeID, *now); !eligible {
		t.Fatal("original receipt lacked eligibility")
	}
	buffered := []hourly.Demand{{Owner: hourly.Owner{ID: owner.ID, Generation: owner.Generation}, Peer: hourlyPeer(peer), Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ReceivedAt: *now, Lifetime: hourly.MaxLifetime, PendingBytes: 1, MinAtomicBytes: 1}}
	repo.policies = nil
	m.refreshHourlyPolicies(context.Background())
	co := m.hourly
	co.allocatorMu.Lock()
	accepted := co.currentHourlyDemands(*now, buffered)
	results, err := co.allocator.ObserveBatch(*now, accepted)
	co.allocatorMu.Unlock()
	if err != nil || len(results) != 0 {
		t.Fatal("stale hint created an allocator owner", results, err)
	}
	policy := hourly.Policy{Owner: buffered[0].Owner, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, BurstBytes: hourly.MaxBurstBytes}
	if grants := co.allocator.Tick(*now, []hourly.Policy{policy}); len(grants) != 0 {
		t.Fatal("a later policy revived the old queued hint", grants)
	}
}
