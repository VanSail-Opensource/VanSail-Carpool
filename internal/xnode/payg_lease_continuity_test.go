package xnode

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPublishedPAYGLeaseSurvivesOnlyUnchangedSameSessionGap(t *testing.T) {
	service, base, peer, master := compileCacheFixture(t)
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	account := PAYGAccount{ProxyUserID: 8, NodeID: peer.nodeID, PeriodID: "payg-continuous", BalanceMinor: 200, UnitPriceMinor: 100}
	repo := &paygLeaseRepository{cacheTestRepository: base, accounts: []PAYGAccount{account}}
	service.repo, peer.paygReady = repo, true
	master.peers[peer.nodeID] = peer
	master.live[peer.nodeID] = liveSnapshot{receivedAt: now, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	candidate := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{{ID: 8, AccountingPeriod: account.PeriodID}}}
	caps := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"}
	ctx := context.Background()
	fresh, err := master.overlayPAYG(ctx, peer, candidate, caps)
	if err != nil || len(fresh.Users) != 1 || fresh.Users[0].ValidUntil == nil {
		t.Fatal("fresh lease missing", fresh.Users, err)
	}
	peer.rememberPublishedPAYG(fresh.Users)
	wireConfig, err := preparePeerWire(peer, fresh, caps)
	if err != nil {
		t.Fatal(err)
	}
	peer.digest, peer.lastControls = wireConfig.digest, master.connectionControls(wireConfig.config)
	now = now.Add(20 * time.Second)
	held, err := master.overlayPAYG(ctx, peer, candidate, caps)
	if err != nil || !peer.paygHeld || len(held.Users) != 1 || !held.Users[0].ValidUntil.Equal(*fresh.Users[0].ValidUntil) {
		t.Fatal("stale gap renewed or removed an unchanged paid deadline", held.Users, err)
	}
	unchanged, err := master.prepareAuthorizationWire(peer, held, caps, false)
	if err != nil || !peer.paygHeld || unchanged.digest != peer.digest {
		t.Fatal("unchanged lease could not be left on the agent", err)
	}
	forced, err := master.prepareAuthorizationWire(peer, held, caps, true)
	if err != nil || peer.paygHeld || forced.config.Users == nil || len(forced.config.Users) != 0 {
		t.Fatal("stale lease was reissued in a requested baseline", forced, err)
	}
	next := &agentPeer{nodeID: peer.nodeID, paygReady: true}
	master.peers[peer.nodeID] = next
	reconnected, err := master.overlayPAYG(ctx, next, candidate, caps)
	if err != nil || len(reconnected.Users) != 0 {
		t.Fatal("new session inherited a lease", reconnected.Users, err)
	}
	master.peers[peer.nodeID] = peer
	now = *fresh.Users[0].ValidUntil
	expired, err := master.overlayPAYG(ctx, peer, candidate, caps)
	if err != nil || len(expired.Users) != 0 {
		t.Fatal("expired authorization held past deadline", expired.Users, err)
	}
}
