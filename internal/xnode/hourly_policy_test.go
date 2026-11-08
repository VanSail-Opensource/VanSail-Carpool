package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func masterHourlyRateFixture(t *testing.T) (*Master, *agentPeer, *hourlyPolicySocketRepository, *time.Time) {
	t.Helper()
	now := time.Now().UTC()
	repo := &hourlyPolicySocketRepository{hourlySocketRepository: masterHourlyRepo(), policies: []HourlyAccountPolicy{{OwnerID: "owner", AccountGeneration: 1, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, DailyAllowanceBytes: 1_000_000_000, NodePaidThrough: map[string]time.Time{"node": now.Add(time.Hour)}}}}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	m.service.now = func() time.Time { return now }
	m.hourly.allocator = hourly.New(now.Add(-hourly.MaxLifetime), hourly.Limits{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m.ctx = ctx
	peer := &agentPeer{nodeID: "node", agentID: masterHourlyAgent, bootID: masterHourlyBoot, hourlySession: masterHourlySession, hourlySender: newHourlyGrantSender(), ctx: ctx, cancel: cancel}
	m.peers[peer.nodeID] = peer
	if err := m.connectHourlyPeer(peer); err != nil {
		t.Fatal(err)
	}
	m.refreshHourlyPolicies(ctx)
	return m, peer, repo, &now
}

func TestHourlyPolicyCacheIsClonedExactScopedFreshAndFailClosed(t *testing.T) {
	m, _, repo, now := masterHourlyRateFixture(t)
	owner := wire.HourlyOwner{ID: "owner", Generation: 1}
	policy, ok := m.hourlyPolicy(owner, "node", *now)
	if !ok || policy.DailyAllowanceBytes != 1_000_000_000 {
		t.Fatal("valid hourly policy unavailable")
	}
	repo.policies[0].NodePaidThrough["node"] = now.Add(-time.Second)
	policy.NodePaidThrough["node"] = now.Add(-time.Second)
	if _, ok = m.hourlyPolicy(owner, "node", *now); !ok {
		t.Fatal("caller or repository mutated cached authoritative snapshot")
	}
	if _, ok = m.hourlyPolicy(wire.HourlyOwner{ID: "owner", Generation: 2}, "node", *now); ok {
		t.Fatal("wrong account generation reused cached policy")
	}
	if _, ok = m.hourlyPolicy(owner, "other-node", *now); ok {
		t.Fatal("nonparticipating node received hourly eligibility")
	}
	if _, ok = m.hourlyPolicy(owner, "node", now.Add(4*time.Second+time.Nanosecond)); ok {
		t.Fatal("stale policy enabled hourly admission")
	}
	repo.policyErr = ErrUnavailable
	m.refreshHourlyPolicies(context.Background())
	if _, ok = m.hourlyPolicy(owner, "node", *now); ok {
		t.Fatal("repository failure retained a usable policy cache")
	}
}

func TestHourlyObserveUsesOriginalReceiptAndNodePaidBoundary(t *testing.T) {
	m, peer, repo, now := masterHourlyRateFixture(t)
	repo.policies[0].NodePaidThrough["node"] = now.Add(2 * time.Second)
	m.refreshHourlyPolicies(context.Background())
	demand := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	received := now.Add(-time.Second)
	if err := m.receiveHourly(context.Background(), peer, wire.New("hourly.demand", "", 0, wire.HourlyDemandBatch{SessionID: masterHourlySession, Demands: []wire.HourlyDemand{demand}}), received); err != nil {
		t.Fatal(err)
	}
	grants := m.hourly.allocator.Tick(*now, m.hourly.allocatorPolicies(*now))
	if len(grants) != 1 || grants[0].Lifetime != 3*time.Second || !grants[0].ConservativeUntil.Equal(repo.policies[0].NodePaidThrough["node"]) {
		t.Fatal("buffer processing refreshed demand lifetime or exceeded node paid boundary")
	}
	*now = now.Add(time.Second)
	m.observeHourlyDemand(peer, demand, now.Add(-time.Second))
	reservation := m.hourly.allocator.Snapshot(*now)[0]
	if !reservation.Until.Equal(repo.policies[0].NodePaidThrough["node"]) {
		t.Fatal("duplicate receipt shifted the original grant basis")
	}
}
