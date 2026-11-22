package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type paygObservationRepository struct {
	*paygLeaseRepository
	coverageReads int
	observations  [][]int64
}

func (r *paygObservationRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	r.coverageReads++
	return nil, nil
}

func (r *paygObservationRepository) ObserveXNodePAYGConnections(_ context.Context, _ string, ids []int64, _ time.Time) error {
	r.observations = append(r.observations, append([]int64{}, ids...))
	return nil
}

func TestPAYGObservationsRequireCurrentFreshActivity(t *testing.T) {
	now := time.Date(2026, time.November, 22, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name          string
		change        func(*Master, *agentPeer, *liveSnapshot)
		sampleSeconds int
		wantObserved  bool
	}{
		{name: "fresh sampled activity", wantObserved: true},
		{name: "repeated old sample", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			old := now.Add(-time.Minute)
			live.report.Users.SampledAt = &old
			live.activitySampleReceivedAt = old
		}},
		{name: "old sample without receive marker", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			old := now.Add(-time.Minute)
			live.report.Users.SampledAt = &old
			live.activitySampleReceivedAt = time.Time{}
		}},
		{name: "slow sampling", sampleSeconds: 60, wantObserved: true, change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			old := now.Add(-time.Minute)
			live.report.Users.SampledAt = &old
			live.activitySampleReceivedAt = old
		}},
		{name: "sample missing after negotiation", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.report.Users.SampledAt = nil
		}},
		{name: "legacy measured activity", wantObserved: true, change: func(_ *Master, peer *agentPeer, live *liveSnapshot) {
			peer.activityReports, live.activityReports = false, false
			live.report.Users.SampledAt = nil
		}},
		{name: "unsupported activity", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.hello.Capabilities = nil
		}},
		{name: "not ready", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.report.Ready = false
		}},
		{name: "not synchronized", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.report.Synchronized = false
		}},
		{name: "pending firewall", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.firewallPending = "revision"
		}},
		{name: "stale report", change: func(_ *Master, _ *agentPeer, live *liveSnapshot) {
			live.receivedAt = now.Add(-time.Minute)
		}},
		{name: "replaced peer", change: func(master *Master, peer *agentPeer, _ *liveSnapshot) {
			master.peers[peer.nodeID] = &agentPeer{nodeID: peer.nodeID}
		}},
		{name: "disconnected peer", change: func(master *Master, peer *agentPeer, _ *liveSnapshot) {
			delete(master.peers, peer.nodeID)
		}},
		{name: "recovery pending", change: func(_ *Master, peer *agentPeer, _ *liveSnapshot) {
			peer.paygReady = false
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &paygObservationRepository{paygLeaseRepository: &paygLeaseRepository{
				cacheTestRepository: &cacheTestRepository{},
				accounts:            []PAYGAccount{{ProxyUserID: 7, UserID: "member", NodeID: "node", GroupID: "group", UnitPriceMinor: 100, BalanceMinor: 100}},
			}}
			service := NewService(repo, nil, nil, nil, nil)
			service.now = func() time.Time { return now }
			master := NewMaster(service, nil)
			peer := &agentPeer{nodeID: "node", paygReady: true, activityReports: true}
			master.peers[peer.nodeID] = peer
			sampled := now
			live := liveSnapshot{
				receivedAt: now, activityReports: true, activitySampleReceivedAt: now,
				hello:  wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}},
				report: wire.TelemetryReport{Ready: true, Synchronized: true, Users: wire.UserMetrics{SampledAt: &sampled, Connections: map[int]int{7: 1}}},
			}
			if test.change != nil {
				test.change(master, peer, &live)
			}
			master.live[peer.nodeID] = live
			if err := master.observePAYG(context.Background(), []*agentPeer{peer}, 2, max(1, test.sampleSeconds)); err != nil {
				t.Fatal(err)
			}
			if test.wantObserved {
				if repo.accountReads != 1 || repo.coverageReads != 1 || len(repo.observations) != 1 || len(repo.observations[0]) != 1 || repo.observations[0][0] != 7 {
					t.Fatal("fresh eligible activity did not reach PAYG observation", repo.accountReads, repo.coverageReads, repo.observations)
				}
			} else if repo.accountReads != 0 || repo.coverageReads != 0 || len(repo.observations) != 0 {
				t.Fatal("unavailable activity reached PAYG reads or writes", repo.accountReads, repo.coverageReads, repo.observations)
			}
		})
	}
}
