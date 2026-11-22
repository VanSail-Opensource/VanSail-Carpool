package xnode

import (
	"slices"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPAYGRenewalPriorityPrecedesInitialCompilesWithoutStarvingThem(t *testing.T) {
	master, repo, peers := syncFairnessFixture(6)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	master.service.now = func() time.Time { return now }
	for i, peer := range peers[4:] {
		peer.paygReady = true
		peer.paygPublishedUntil = now.Add(time.Duration(10+i) * time.Second)
		master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	}
	if got := cancelledSyncPair(t, master, repo, peers); !slices.Equal(got, []string{peers[4].nodeID, peers[5].nodeID}) {
		t.Fatalf("initial compiles displaced expiring PAYG renewals: %v", got)
	}
	// Both priority attempts failed. They retain normal retries, but cannot
	// repeatedly take the priority slots while unattempted peers wait.
	for pass := range 2 {
		got := cancelledSyncPair(t, master, repo, peers)
		want := []string{peers[2*pass].nodeID, peers[2*pass+1].nodeID}
		if !slices.Equal(got, want) {
			t.Fatalf("failed renewals starved ordinary pass %d: got %v want %v", pass, got, want)
		}
	}
}

func TestPAYGRenewalPriorityRequiresFreshSamePeerEvidence(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		remaining time.Duration
		adjust    func(*Master, *agentPeer, *liveSnapshot)
		priority  bool
	}{
		{name: "renewal due", remaining: 40 * time.Second, priority: true},
		{name: "not yet due", remaining: 41 * time.Second},
		{name: "expired lease", remaining: -time.Second, priority: true},
		{name: "agent ahead", remaining: 90 * time.Second, priority: true, adjust: func(_ *Master, _ *agentPeer, live *liveSnapshot) { live.report.Time = now.Add(time.Minute) }},
		{name: "untrusted clock offset", remaining: 90 * time.Second, adjust: func(_ *Master, _ *agentPeer, live *liveSnapshot) { live.report.Time = now.Add(3 * time.Minute) }},
		{name: "stale report", remaining: time.Second, adjust: func(_ *Master, _ *agentPeer, live *liveSnapshot) { live.receivedAt = now.Add(-time.Minute) }},
		{name: "journal recovery pending", remaining: time.Second, adjust: func(_ *Master, peer *agentPeer, _ *liveSnapshot) { peer.paygReady = false }},
		{name: "firewall pending", remaining: time.Second, adjust: func(_ *Master, _ *agentPeer, live *liveSnapshot) { live.firewallPending = "pending" }},
		{name: "replaced peer", remaining: time.Second, adjust: func(m *Master, peer *agentPeer, _ *liveSnapshot) {
			m.peers[peer.nodeID] = &agentPeer{nodeID: peer.nodeID}
		}},
		{name: "recent priority attempt", remaining: time.Second, adjust: func(_ *Master, peer *agentPeer, _ *liveSnapshot) { peer.paygPriorityAt = now.Add(-19 * time.Second) }},
		{name: "priority interval elapsed", remaining: time.Second, priority: true, adjust: func(_ *Master, peer *agentPeer, _ *liveSnapshot) { peer.paygPriorityAt = now.Add(-paygRenewalInterval) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			master, _, peers := syncFairnessFixture(2)
			peer := peers[1]
			peer.paygReady, peer.paygPublishedUntil = true, now.Add(test.remaining)
			live := liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
			if test.adjust != nil {
				test.adjust(master, peer, &live)
			}
			master.live[peer.nodeID] = live
			order := master.peerSyncOrder(peers, 0, now, 2)
			if len(order) != 2 || order[0].priority != test.priority || (order[0].offset == 1) != test.priority {
				t.Fatalf("unsafe or missing renewal priority: %+v", order)
			}
		})
	}
}

func TestPublishedPAYGDeadlineTracksEarliestAndClears(t *testing.T) {
	now := time.Now().UTC()
	early, later := now.Add(10*time.Second), now.Add(time.Minute)
	peer := &agentPeer{}
	peer.rememberPublishedPAYG([]wire.User{
		{ID: 1, AccountingPeriod: "combo", ValidUntil: &now},
		{ID: 2, AccountingPeriod: "payg-a", ValidUntil: &later},
		{ID: 3, AccountingPeriod: "payg-b", ValidUntil: &early},
	})
	if !peer.paygPublishedUntil.Equal(early) || len(peer.paygPublished) != 2 {
		t.Fatalf("published hint included non-PAYG expiry: %+v", peer.paygPublished)
	}
	peer.rememberPublishedPAYG(nil)
	if !peer.paygPublishedUntil.IsZero() || len(peer.paygPublished) != 0 {
		t.Fatal("withdrawn authorization retained renewal priority")
	}
}
