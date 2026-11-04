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
		return wire.User{ID: id, InboundTags: []string{"in"}, Credentials: map[string]json.RawMessage{"in": json.RawMessage(`{"id":"synthetic"}`)}, BlockNew: block, ValidUntil: &deadline}
	}
	evidence := firstSyncCandidates(wire.Config{Users: []wire.User{user(11, false, until), user(22, true, until), user(33, false, expired), {ID: 44}}}, 7, now)
	if len(evidence.principals) != 1 || evidence.principals[0] != 11 {
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
