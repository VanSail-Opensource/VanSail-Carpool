package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type hourlyPoisonProofRepository struct {
	*hourlySocketRepository
	seen []string
}

func (r *hourlyPoisonProofRepository) ApplyHourlyCreditProof(ctx context.Context, _, _ string, proof model.HourlyCreditProof) (model.HourlyCreditReservation, error) {
	r.seen = append(r.seen, proof.ReservationID)
	if proof.ReservationID == "credit-00000" {
		<-ctx.Done()
		return model.HourlyCreditReservation{}, ctx.Err()
	}
	return model.HourlyCreditReservation{HourlyCreditRequest: proof.HourlyCreditRequest, IssueSequence: proof.IssueSequence, FinalProofDigest: proof.Digest, UnusedBytes: proof.UnusedBytes}, nil
}

func TestHourlyProofTurnRequeuesUnattemptedNeighborAheadOfLockedProof(t *testing.T) {
	repo := &hourlyPoisonProofRepository{hourlySocketRepository: masterHourlyRepo()}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	at := time.Now()
	m.service.now = func() time.Time { return at }
	peer := &agentPeer{nodeID: "node", agentID: masterHourlyAgent, bootID: masterHourlyBoot, hourlySession: masterHourlySession, ctx: context.Background()}
	m.peers[peer.nodeID] = peer
	r := &peerReceivers{master: m, peer: peer, hourly: newHourlyInboundLane()}
	q := r.hourly.financial
	poison, valid := hourlyLaneProof(0, false), hourlyLaneProof(1, false)
	if q.offerProofs([]wire.HourlyCreditProof{poison}) != nil || q.offerProofs([]wire.HourlyCreditProof{valid}) != nil {
		t.Fatal("valid capture rejected")
	}
	work := q.take(at)
	call, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	r.applyHourlyFinancial(call, work)
	cancel()
	if len(repo.seen) != 1 || repo.seen[0] != poison.Install.ReservationID || len(q.items) != 2 {
		t.Fatal("cancelled turn cleared or attempted an uncovered proof", repo.seen, len(q.items))
	}
	if state := m.hourlyCredits.peers[peer]; state != nil && len(state.pending) != 0 {
		t.Fatal("uncommitted proof was acknowledged")
	}
	at = at.Add(time.Second)
	work = q.take(at)
	if len(work) != 2 || work[0].id != valid.Install.ReservationID {
		t.Fatal("one locked proof remained ahead of its unattempted neighbor", work)
	}
	call, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	r.applyHourlyFinancial(call, work)
	cancel()
	state := m.hourlyCredits.peers[peer]
	if len(q.items) != 1 || q.items[poison.Install.ReservationID] == nil || state == nil || len(state.pending) != 1 || state.pending[valid.Install.ReservationID].Digest != valid.Digest {
		t.Fatal("valid neighbor starved or failed proof gained an ACK")
	}
}

func TestHourlyFailedProgressBatchSplitsWithoutPoisoningValidNeighbors(t *testing.T) {
	q := newHourlyFinancialLane()
	at := time.Now()
	for i := range wire.MaxHourlyItems {
		proof := hourlyLaneProof(i, true)
		if err := q.offerProgress([]wire.HourlyCreditProgress{{ReservationID: proof.Install.ReservationID, IssueSequence: proof.Install.IssueSequence, AdmittedUploadBytes: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 64 && len(q.items) > 1; attempt++ {
		work := q.take(at)
		var err error
		for _, item := range work {
			if item.id == "credit-00000" {
				err = ErrConflict
			}
		}
		q.finish(work, err, nil, at)
		at = at.Add(time.Second)
	}
	if len(q.items) != 1 || q.items["credit-00000"] == nil {
		t.Fatal("one unresolved progress identity permanently poisoned valid neighbors", len(q.items))
	}
}
