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
	deleted     int
	adminDenied bool
}

func (*revokedPeerRepository) RotateXNodeToken(context.Context, string, []byte) error { return nil }
func (r *revokedPeerRepository) DeleteXNodeNode(context.Context, string) error {
	r.deleted++
	return nil
}

func (r *revokedPeerRepository) RotateAdminXNodeToken(context.Context, string, string, []byte) error {
	if r.adminDenied {
		return ErrConflict
	}
	return nil
}

func (r *revokedPeerRepository) DeleteAdminXNodeNode(ctx context.Context, _, id string) error {
	if r.adminDenied {
		return ErrConflict
	}
	return r.DeleteXNodeNode(ctx, id)
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

func TestRejectedAdministratorMutationPreservesConnectedPeerAndNode(t *testing.T) {
	repo := &revokedPeerRepository{adminDenied: true}
	s := NewService(repo, nil, nil, nil, nil)
	m := NewMaster(s, nil)
	peer := &agentPeer{nodeID: "node"}
	m.peers[peer.nodeID] = peer
	if err := m.rotateEnrollmentAs(context.Background(), peer.nodeID, []byte("replacement"), "demoted"); !errors.Is(err, ErrConflict) {
		t.Fatal("rejected rotation did not preserve store refusal", err)
	}
	if !m.Connected(peer.nodeID) {
		t.Fatal("uncommitted token rotation disconnected the authorized peer")
	}
	delete(m.peers, peer.nodeID)
	if err := s.DeleteNodeAs(context.Background(), "demoted", peer.nodeID); !errors.Is(err, ErrConflict) || repo.deleted != 0 {
		t.Fatal("rejected administrator deletion used a raw writer", err, repo.deleted)
	}
	repo.adminDenied = false
	if err := s.DeleteNodeAs(context.Background(), "restored", peer.nodeID); err != nil || repo.deleted != 1 {
		t.Fatal("restored administrator could not delete the drained node", err, repo.deleted)
	}
}
