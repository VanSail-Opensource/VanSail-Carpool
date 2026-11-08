package xnode

import (
	"context"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Credit evidence remains owner/generation scoped after access or account
// closure. Policy admission belongs to sales/reservation, never settlement.
type hourlyCreditRepository interface {
	RecordHourlyCreditProgressBatch(context.Context, string, string, []model.HourlyCreditProgress) ([]model.HourlyCreditReservation, error)
	ApplyHourlyCreditProof(context.Context, string, string, model.HourlyCreditProof) (model.HourlyCreditReservation, error)
}

func (m *Master) hourlyCreditSupported() bool {
	_, ok := m.service.repo.(hourlyCreditRepository)
	return ok
}

func (m *Master) currentHourlyCreditPeer(peer *agentPeer) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return peer != nil && m.peers[peer.nodeID] == peer && peer.hourlySession != "" && peer.ctx != nil && peer.ctx.Err() == nil
}

func (m *Master) receiveHourlyCreditProgress(ctx context.Context, peer *agentPeer, items []wire.HourlyCreditProgress) error {
	if len(items) == 0 {
		return nil
	}
	if len(items) > wire.MaxHourlyItems || !m.currentHourlyCreditPeer(peer) {
		return ErrInvalid
	}
	repo, ok := m.service.repo.(hourlyCreditRepository)
	if !ok {
		return ErrUnavailable
	}
	progress := make([]model.HourlyCreditProgress, 0, len(items))
	for _, item := range items {
		if wire.ValidateHourlyProgress(item) != nil {
			return ErrInvalid
		}
		progress = append(progress, model.HourlyCreditProgress{ReservationID: item.ReservationID, IssueSequence: int64(item.IssueSequence), AdmittedUploadBytes: int64(item.AdmittedUploadBytes), AdmittedDownloadBytes: int64(item.AdmittedDownloadBytes)})
	}
	rows, err := repo.RecordHourlyCreditProgressBatch(ctx, peer.nodeID, peer.agentID, progress)
	if err == nil {
		m.observeHourlyCreditProgress(peer, rows)
		for _, row := range rows {
			if row.FinalProofDigest == "" {
				m.requestHourlyCredit(peer, nil)
				break
			}
		}
	}
	return err
}

func (m *Master) receiveHourlyCreditProofs(ctx context.Context, peer *agentPeer, message wire.Message) error {
	if len(message.Data) > wire.MaxHourlyFrameBytes || !m.currentHourlyCreditPeer(peer) {
		return ErrInvalid
	}
	var batch wire.HourlyCreditProofBatch
	if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyProofBatch(batch) != nil {
		return ErrInvalid
	}
	repo, ok := m.service.repo.(hourlyCreditRepository)
	if !ok {
		return ErrUnavailable
	}
	for _, proof := range batch.Proofs {
		if proof.Install.AgentID != peer.agentID {
			return ErrInvalid
		}
		converted := hourlyCreditProofModel(peer.nodeID, proof)
		settled, err := repo.ApplyHourlyCreditProof(ctx, peer.nodeID, peer.agentID, converted)
		if err != nil {
			return err
		}
		m.hourlyCreditSettled(peer, settled)
		ack := wire.HourlyCreditAck{ReservationID: settled.ReservationID, IssueSequence: uint64(settled.IssueSequence), Digest: settled.FinalProofDigest}
		if err = wire.ValidateHourlyAck(ack); err != nil {
			return ErrInvalid
		}
		if err = m.queueHourlyCreditAck(peer, ack); err != nil {
			return err
		}
	}
	return nil
}

func hourlyCreditProofModel(node string, proof wire.HourlyCreditProof) model.HourlyCreditProof {
	i := proof.Install
	// Validators bound every wire quantity before these exact integer casts.
	if i.Owner.Generation > math.MaxInt64 {
		return model.HourlyCreditProof{}
	}
	return model.HourlyCreditProof{HourlyCreditRequest: model.HourlyCreditRequest{ReservationID: i.ReservationID, OwnerID: i.Owner.ID, AccountGeneration: int64(i.Owner.Generation), DayUTC: i.DayUTC, NodeID: node, AgentID: i.AgentID, BootID: i.BootID, SessionID: i.SessionID, Kind: i.Kind, ByteCeiling: int64(i.ByteCeiling)}, IssueSequence: int64(i.IssueSequence), AdmittedUploadBytes: int64(proof.AdmittedUploadBytes), AdmittedDownloadBytes: int64(proof.AdmittedDownloadBytes), UnusedBytes: int64(proof.UnusedBytes), UnknownBytes: int64(proof.UnknownBytes), Recovery: proof.Recovery, Digest: proof.Digest}
}
