package xnode

import (
	"context"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPAYGRecoveryCannotOverwriteNewerJournalFailure(t *testing.T) {
	repo := &rejectedTrafficRepository{}
	master := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	peer := &agentPeer{nodeID: "node", agentID: "agent", paygRecoveryQueued: true}
	master.peers[peer.nodeID] = peer
	barrierVersion := peer.paygFailureVersion
	message := wire.New("traffic.report", "agent:2", 0, wire.TrafficReport{ID: "agent:2"})
	if err := master.receiveTraffic(context.Background(), peer, message); !errors.Is(err, ErrTrafficSequenceGap) {
		t.Fatal("fixture did not reject the later journal", err)
	}
	if peer.paygFailureVersion == barrierVersion {
		t.Fatal("journal rejection did not advance recovery fence")
	}
	if err := master.completePAYGRecovery(peer, barrierVersion); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old recovery completed across newer journal failure", err)
	}
	if peer.paygReady || !peer.paygRecoveryQueued || peer.forceSync || len(master.stateWake) != 0 {
		t.Fatal("old barrier restored authorization or discarded recovery")
	}
	if err := master.completePAYGRecovery(peer, peer.paygFailureVersion); err != nil {
		t.Fatal("current committed barrier could not recover", err)
	}
	if !peer.paygReady || peer.paygRecoveryQueued || !peer.forceSync || len(master.stateWake) != 1 {
		t.Fatal("current recovery did not wake ordinary authorization publication")
	}
}

func TestPAYGRecoveryCannotAuthorizeReplacementConnection(t *testing.T) {
	master := NewMaster(NewService(&rejectedTrafficRepository{}, nil, nil, nil, nil), nil)
	old := &agentPeer{nodeID: "node", paygRecoveryQueued: true}
	next := &agentPeer{nodeID: old.nodeID}
	master.peers[old.nodeID] = next
	if err := master.completePAYGRecovery(old, 0); err != nil {
		t.Fatal(err)
	}
	if old.paygReady || next.paygReady || len(master.stateWake) != 0 {
		t.Fatal("replaced connection inherited old recovery evidence")
	}
}
