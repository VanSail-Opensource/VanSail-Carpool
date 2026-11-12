package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func TestFirstSyncRequiresExactPeerAppliedMembership(t *testing.T) {
	now := time.Now().UTC()
	service := NewService(nil, nil, nil, nil, nil)
	service.now = func() time.Time { return now }
	m := NewMaster(service, nil)
	until, expired := now.Add(time.Hour), now.Add(-time.Second)
	user := func(id int, block bool, deadline time.Time) wire.User {
		return wire.User{ID: id, AccountingPeriod: "period-captured", InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"in": json.RawMessage(`{"id":"synthetic"}`)}, BlockNew: block, ValidUntil: &deadline}
	}
	subjects := map[int64]FirstSyncSubject{
		11: {ProxyUserID: 11, PeriodID: "period-captured"},
		22: {ProxyUserID: 22, PeriodID: "period-captured"},
		33: {ProxyUserID: 33, PeriodID: "period-captured"},
	}
	evidence := firstSyncCandidates(wire.Config{Users: []wire.User{user(11, false, until), user(22, true, until), user(33, false, expired), {ID: 44}, user(55, false, until)}}, 7, now, subjects)
	if len(evidence.principals) != 1 || evidence.principals[0] != (FirstSyncSubject{ProxyUserID: 11, PeriodID: "period-captured"}) {
		t.Fatal("ineligible principal included", evidence.principals)
	}
	peer := &agentPeer{nodeID: "node", revision: 7, firstSync: evidence}
	m.peers["node"] = peer
	m.recordFirstSyncAck(peer, 6)
	if !evidence.appliedAt.IsZero() {
		t.Fatal("stale revision proved membership")
	}
	m.peers["node"] = &agentPeer{}
	m.recordFirstSyncAck(peer, 7)
	if !evidence.appliedAt.IsZero() {
		t.Fatal("replaced peer proved membership")
	}
	m.peers["node"] = peer
	m.recordFirstSyncAck(peer, 7)
	if !evidence.appliedAt.Equal(now) {
		t.Fatal("matching proof missing")
	}
}

func TestUnconfirmedMemberCannotRevealTokenWithoutMaster(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil)
	identity := Identity{UserID: "owner", SubscriptionCipher: "secret"}
	ready, err := s.SubscriptionReadiness(context.Background(), identity)
	if err != nil || ready.Ready {
		t.Fatal(ready, err)
	}
	url, err := s.SubscriptionURL(identity)
	if err != nil || url != "" {
		t.Fatal("unconfirmed token disclosed", url, err)
	}
}

func TestFirstSyncRejectsMissingPeriodAndExpiredAcknowledgment(t *testing.T) {
	now := time.Now().UTC()
	until := now.Add(time.Second)
	config := wire.Config{Users: []wire.User{
		{ID: 11, AccountingPeriod: "period-old", InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"in": json.RawMessage(`{"id":"synthetic"}`)}, ValidUntil: &until},
		{ID: 22, InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"in": json.RawMessage(`{"id":"synthetic"}`)}},
	}}
	subjects := map[int64]FirstSyncSubject{11: {ProxyUserID: 11, PeriodID: "period-old"}}
	evidence := firstSyncCandidates(config, 7, now, subjects)
	if len(evidence.principals) != 1 || evidence.principals[0].PeriodID != "period-old" {
		t.Fatal("missing accounting identity accepted", evidence.principals)
	}
	config.Users[0].AccountingPeriod = "period-new"
	subjects[11] = FirstSyncSubject{ProxyUserID: 11, PeriodID: "period-new"}
	if evidence.principals[0].PeriodID != "period-old" {
		t.Fatal("captured accounting identity changed")
	}
	service := NewService(nil, nil, nil, nil, nil)
	service.now = func() time.Time { return until }
	m := NewMaster(service, nil)
	peer := &agentPeer{nodeID: "node", revision: 7, firstSync: evidence}
	m.peers["node"] = peer
	m.recordFirstSyncAck(peer, 7)
	if len(evidence.principals) != 0 {
		t.Fatal("expired acknowledgment proved readiness", evidence.principals)
	}
}

func TestFirstSyncRepairCanForceUnchangedConfigurationPublication(t *testing.T) {
	master, peer, repo := syncPendingFixture()
	hello := master.live[peer.nodeID].hello
	prepared, err := preparePeerWire(peer, peer.compiled.config, hello.Capabilities)
	if err != nil {
		t.Fatal(err)
	}
	peer.digest = prepared.digest
	ctx := context.Background()
	published, err := master.syncNodeLocked(ctx, peer, false)
	if err != nil || !published || repo.advances != 0 {
		t.Fatal("unchanged background configuration did not reuse its revision", published, repo.advances, err)
	}
	// Readiness calls this existing path after the repair migration clears old
	// proof. Even an unchanged configuration must obtain a fresh applied ACK.
	master.requestSync(peer)
	master.syncPeer(ctx, peer, master.service.Now())
	if repo.advances != 1 {
		t.Fatal("readiness refresh could not republish unchanged native material", repo.advances)
	}
}
