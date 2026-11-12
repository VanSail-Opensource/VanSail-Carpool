package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type syncPendingRepository struct {
	cacheTestRepository
	queues                 map[string]int
	advances, periodChecks int
	period                 Period
	publicationErr         error
}

func (r *syncPendingRepository) QueueXNodeGeoIP(context.Context, string) error {
	r.queues["country"]++
	return nil
}

func (r *syncPendingRepository) EnqueueOutbox(_ context.Context, kind, _ string, _ time.Time) error {
	r.queues[kind]++
	return nil
}

func (r *syncPendingRepository) AdvanceXNodeRevision(context.Context, string, uint64) (uint64, error) {
	r.advances++
	return 0, r.publicationErr
}

func (r *syncPendingRepository) XNodePeriod(context.Context, string) (Period, error) {
	r.periodChecks++
	return r.period, nil
}

func syncPendingFixture() (*Master, *agentPeer, *syncPendingRepository) {
	repo := &syncPendingRepository{
		cacheTestRepository: cacheTestRepository{revision: 1},
		queues:              map[string]int{},
		publicationErr:      errors.New("fixture publication failed"),
	}
	master := NewMaster(NewService(repo, cacheTestSettings{}, nil, nil, nil), nil)
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}}
	peer := &agentPeer{nodeID: "node", revision: 7, compiled: &compiledState{version: 1, config: config}, pending: map[string]chan wire.Message{}}
	master.peers[peer.nodeID] = peer
	master.live[peer.nodeID] = liveSnapshot{hello: wire.AgentHello{Capabilities: []string{"source_geoip_firewall_v1", "source_geoip_stream_v1", "firewall_master_observation_v1"}}}
	return master, peer, repo
}

func pendingCountry(peer *agentPeer) {
	peer.compiled.config.Firewall = wire.FirewallPolicy{Enabled: true, SourceGeoIP: &wire.SourceGeoIPPolicy{Generation: "country-v1", Prefixes: []string{"8.8.8.0/24"}}}
}

func TestExplicitSyncCannotCompleteBeforePublication(t *testing.T) {
	for _, name := range []string{"upgrade", "country", "protected", "backoff"} {
		t.Run(name, func(t *testing.T) {
			master, peer, repo := syncPendingFixture()
			hello := master.live[peer.nodeID].hello
			switch name {
			case "upgrade":
				hello.UpgradePending = true
				master.live[peer.nodeID] = liveSnapshot{hello: hello}
			case "country":
				pendingCountry(peer)
			case "protected", "backoff":
				prepared, err := preparePeerWire(peer, peer.compiled.config, hello.Capabilities)
				if err != nil {
					t.Fatal(err)
				}
				if name == "protected" {
					master.ddnsProtectedIPs = []string{"9.9.9.9"}
					prepared.routeAddresses = map[string]bool{"9.9.9.9": true}
				} else {
					peer.failedDigest = prepared.digest
					peer.retryAt = master.service.now().Add(time.Minute)
				}
			}
			ctx := context.Background()
			if err := master.Sync(ctx, peer.nodeID); !errors.Is(err, ErrUnavailable) {
				t.Fatal("unpublished target reported sync success", err)
			}
			if repo.advances != 0 || peer.revision != 7 {
				t.Fatal("pending dependency advanced the target", repo.advances, peer.revision)
			}
			if name == "country" && repo.queues["country"] != 1 || name == "protected" && repo.queues[FirewallJob] != 1 {
				t.Fatal("dependency was not queued", repo.queues)
			}
			// Pending work must keep the background cadence instead of restoring
			// a force flag and repeatedly waking routing reconciliation.
			peer.forceSync = true
			master.syncPeer(ctx, peer, master.service.now())
			if peer.forceSync || repo.advances != 0 {
				t.Fatal("pending background sync changed retry behavior", peer.forceSync, repo.advances)
			}
			if name == "country" && repo.queues["country"] != 1 {
				t.Fatal("pending country generation was queued repeatedly", repo.queues)
			}
			// Acknowledging the prerequisite permits publication, whose actual
			// repository failure must still reach the caller unchanged.
			hello.UpgradePending = false
			master.live[peer.nodeID] = liveSnapshot{hello: hello}
			peer.geoIPGeneration = "country-v1"
			peer.protectedIPs = []string{"9.9.9.9"}
			peer.retryAt = time.Time{}
			if err := master.Sync(ctx, peer.nodeID); !errors.Is(err, repo.publicationErr) || repo.advances != 1 {
				t.Fatal("released dependency did not attempt publication", err, repo.advances)
			}
		})
	}
}

func TestPeriodClosureWaitsForCountryPublicationBeforeNativeBarrier(t *testing.T) {
	master, peer, repo := syncPendingFixture()
	pendingCountry(peer)
	repo.period = Period{ID: "closing-period", ProxyUserID: 42, Status: "closing", ClosureGeneration: 3}
	err := master.service.closePeriodNode(context.Background(), repo.period, peer.nodeID)
	if !errors.Is(err, ErrUnavailable) || !strings.HasPrefix(err.Error(), "revocation_pending:") {
		t.Fatal("unpublished revocation reached the native traffic barrier", err)
	}
	if repo.periodChecks != 1 || repo.advances != 0 || repo.queues["country"] != 1 {
		t.Fatal("pending revocation advanced period closure", repo.periodChecks, repo.advances, repo.queues)
	}
}
