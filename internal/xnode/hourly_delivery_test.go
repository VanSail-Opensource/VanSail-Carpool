package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyGrantUsesExistingTLSWriterWithExactPeerAndWireTerms(t *testing.T) {
	now := time.Now()
	repo := &hourlyPolicySocketRepository{hourlySocketRepository: masterHourlyRepo(), policies: []HourlyAccountPolicy{{OwnerID: "owner", AccountGeneration: 1, PaidThrough: now.Add(time.Hour), RateBytesPerSecond: 1000, NodePaidThrough: map[string]time.Time{"node": now.Add(time.Hour)}}}}
	m, server, token := masterHourlyFixture(t, repo)
	m.hourly.allocator = hourly.New(now.Add(-hourly.MaxLifetime), hourly.Limits{})
	socket, _ := masterHourlySocket(t, server, token, masterHourlySession, wire.HourlyFormat, true)
	defer socket.Close()
	peer := waitMasterHourlyPeer(t, m)
	m.refreshHourlyPolicies(context.Background())
	demand := wire.HourlyDemand{Owner: wire.HourlyOwner{ID: "owner", Generation: 1}, Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LifetimeMillis: 10000, PendingBytes: 1, MinAtomicBytes: 1}
	m.observeHourlyDemand(peer, demand, m.service.now())
	co := m.hourly
	co.allocatorMu.Lock()
	at := m.service.now()
	actions := co.allocator.Tick(at, co.allocatorPolicies(at))
	co.allocatorMu.Unlock()
	if len(actions) != 1 {
		t.Fatal("eligible demand produced no rate grant")
	}
	m.enqueueHourlyGrants(actions)
	if err := socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var frame wire.Message
	if err := socket.ReadJSON(&frame); err != nil || frame.Type != "hourly.grant" {
		t.Fatal("rate grant did not use the authenticated serialized writer", err)
	}
	var grant wire.HourlyGrant
	if wire.Decode(frame.Data, &grant) != nil || wire.ValidateHourlyGrant(grant) != nil || grant.SessionID != masterHourlySession || grant.Nonce != demand.Nonce || grant.RateBytesPerSecond != 1000 || grant.BurstBytes != hourly.MaxBurstBytes || grant.Sequence != actions[0].Sequence {
		t.Fatal("wire rate grant changed immutable allocator terms")
	}
}
