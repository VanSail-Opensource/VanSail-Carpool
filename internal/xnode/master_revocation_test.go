package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type revokedPeerRepository struct {
	Repository
	deleted int
}

func (*revokedPeerRepository) RotateXNodeToken(context.Context, string, []byte) error { return nil }
func (r *revokedPeerRepository) DeleteXNodeNode(context.Context, string) error {
	r.deleted++
	return nil
}

func TestRevokedPeerCannotCreateLeasesOrSurviveDeletion(t *testing.T) {
	repo := &revokedPeerRepository{}
	s := NewService(repo, nil, nil, nil, nil)
	m := NewMaster(s, nil)
	peer := &agentPeer{nodeID: "node", agentID: "agent", bootID: "boot", activityReports: true}
	m.peers[peer.nodeID] = peer
	if err := s.DeleteNode(context.Background(), peer.nodeID); !errors.Is(err, ErrConflict) || repo.deleted != 0 {
		t.Fatal("connected node was not protected from deletion", err)
	}
	if err := m.rotateEnrollment(context.Background(), peer.nodeID, []byte("new-token")); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	message := wire.New("telemetry.report", "", 0, wire.TelemetryReport{AgentID: peer.agentID, BootID: peer.bootID, Time: at, Users: wire.UserMetrics{SampledAt: &at, ActiveIPObservations: []wire.IPActivity{{UserID: 1, IP: "8.8.8.8", LastActive: at, Observations: 1}}}})
	if err := m.receiveActivity(peer, message); !errors.Is(err, ErrConflict) || len(m.automaticLeases.leases) != 0 {
		t.Fatal("revoked peer created an automatic lease", err)
	}
	if err := s.DeleteNode(context.Background(), peer.nodeID); err != nil || repo.deleted != 1 {
		t.Fatal("drained node could not be deleted", err)
	}
	if m.Connected(peer.nodeID) {
		t.Fatal("revoked peer remained registered")
	}
}
