package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/gorilla/websocket"
)

func TestPAYGRenewalPrecedesRoutingMaintenance(t *testing.T) {
	observationFailure := errors.New("connection observation unavailable")
	for _, test := range []struct {
		name           string
		waitForExpiry  bool
		observationErr error
		forceSync      bool
	}{
		{name: "routing completes"},
		{name: "routing exhausts pass deadline", waitForExpiry: true},
		{name: "observation failure still permits maintenance", observationErr: observationFailure},
		{name: "forced renewal precedes routing wake", forceSync: true},
		{name: "forced renewal precedes stalled routing wake", forceSync: true, waitForExpiry: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, base, _, master := compileCacheFixture(t)
			now := time.Date(2026, 11, 22, 15, 0, 0, 0, time.UTC)
			service.now = func() time.Time { return now }
			repo := &renewalBeforeRoutingRepository{
				paygLeaseRepository: &paygLeaseRepository{cacheTestRepository: base},
				nodes:               []Node{{ID: "first", Enabled: true}, {ID: "second", Enabled: true}},
				funding:             PAYGFunding{ProxyUserID: 8, UnitPriceMinor: 100, BalanceMinor: 200},
				waitForExpiry:       test.waitForExpiry, observationErr: test.observationErr,
			}
			service.repo = repo
			caps := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards", "sampled_activity_v1"}
			previousUntil := now.Add(10 * time.Second)
			sockets := make(map[string]*websocket.Conn)
			for _, node := range repo.nodes {
				config := wire.Config{
					Xray:      json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`),
					Users:     []wire.User{{ID: 8, AccountingPeriod: "payg-renewal"}},
					Telemetry: DefaultGlobal().Telemetry,
				}
				peer := &agentPeer{nodeID: node.ID, revision: 1, paygReady: true, forceSync: test.forceSync, compiled: &compiledState{version: 1, config: config}}
				baseline := config
				baseline.Users = []wire.User{{ID: 8, AccountingPeriod: "payg-renewal", ValidUntil: &previousUntil}}
				prepared, err := preparePeerWire(peer, baseline, caps)
				if err != nil {
					t.Fatal(err)
				}
				peer.digest, peer.lastControls = prepared.digest, master.connectionControls(baseline)
				peer.rememberPublishedPAYG(baseline.Users)
				sockets[node.ID] = renewalRoutingSocket(t, peer)
				master.peers[node.ID] = peer
				master.live[node.ID] = liveSnapshot{receivedAt: now, hello: wire.AgentHello{Capabilities: caps}, report: wire.TelemetryReport{Ready: true, Synchronized: true, Users: wire.UserMetrics{Connections: map[int]int{8: 1}}}}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := master.reconcile(ctx); !errors.Is(err, test.observationErr) {
				t.Fatalf("reconciliation error=%v, want=%v", err, test.observationErr)
			}
			if test.waitForExpiry && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatal("routing did not consume the shared pass deadline")
			}
			if master.routingWakePending.Load() != (test.forceSync && test.waitForExpiry) {
				t.Fatal("routing wake was lost or retained after successful maintenance")
			}
			if repo.routingCalls != 1 || repo.priorRenewals != 2 || repo.fundingReads.Load() != 2 || base.reads != 0 || repo.observations == 0 {
				t.Fatalf("renewal/maintenance ordering changed: routes=%d prior renewals=%d funding reads=%d compiles=%d observations=%d", repo.routingCalls, repo.priorRenewals, repo.fundingReads.Load(), base.reads, repo.observations)
			}
			if !test.waitForExpiry && repo.planReads != 1 {
				t.Fatal("routing plans were not reconciled after renewal")
			}
			for id, socket := range sockets {
				if err := socket.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				var frame wire.Message
				if err := socket.ReadJSON(&frame); err != nil || frame.Type != "state.sync" || frame.Revision != 2 {
					t.Fatalf("node %s never received its renewal: %+v %v", id, frame, err)
				}
				var config wire.Config
				if err := wire.Decode(frame.Data, &config); err != nil || len(config.Users) != 1 || config.Users[0].ValidUntil == nil {
					t.Fatalf("node %s received no PAYG authorization: %+v %v", id, config, err)
				}
				if user := config.Users[0]; user.AccountingPeriod != "payg-renewal" || !user.ValidUntil.Equal(now.Add(time.Minute)) || !user.ValidUntil.After(previousUntil) {
					t.Fatalf("node %s renewal changed period or deadline: %+v", id, user)
				}
			}
		})
	}
}

func TestForcedRoutingWakePreservesCachedPlanReconciliation(t *testing.T) {
	service, base, _, _ := compileCacheFixture(t)
	repo := &renewalBeforeRoutingRepository{paygLeaseRepository: &paygLeaseRepository{cacheTestRepository: base}}
	service.repo = repo
	ctx := context.Background()
	for _, force := range []bool{false, false, true} {
		if err := service.reconcileRoutingPlans(ctx, force); err != nil {
			t.Fatal(err)
		}
	}
	if repo.routingCalls != 2 || repo.planReads != 1 {
		t.Fatalf("reconnect wake lost or unnecessarily rescanned plans: wakes=%d reads=%d", repo.routingCalls, repo.planReads)
	}
}
