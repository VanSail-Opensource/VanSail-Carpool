package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func hourlyCreditInstall(r model.HourlyCreditReservation) wire.HourlyCreditInstall {
	return wire.HourlyCreditInstall{ReservationID: r.ReservationID, IssueSequence: uint64(r.IssueSequence), Owner: wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}, DayUTC: r.DayUTC, AgentID: r.AgentID, BootID: r.BootID, SessionID: r.SessionID, ByteCeiling: uint64(r.ByteCeiling), Kind: r.Kind}
}

func (m *Master) installHourlyReservation(ctx context.Context, peer *agentPeer, r model.HourlyCreditReservation) error {
	if err := m.reconcileHourlyCreditPredecessors(ctx, peer, r); err != nil {
		return err
	}
	return m.sendHourlyCreditInstall(ctx, peer, r)
}

func (m *Master) sendHourlyCreditInstall(ctx context.Context, peer *agentPeer, r model.HourlyCreditReservation) error {
	if !m.currentHourlyCreditPeer(peer) || r.NodeID != peer.nodeID || r.AgentID != peer.agentID || r.BootID != peer.bootID || r.SessionID != peer.hourlySession || r.DayUTC != m.service.now().UTC().Format(time.DateOnly) {
		return ErrUnavailable
	}
	if _, ok := m.hourlyPolicy(wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}, peer.nodeID, m.service.now()); !ok {
		return ErrUnavailable
	}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.peers[peer]
	retired := state == nil || state.retireAll || state.retire[wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}]
	if !retired {
		if _, exists := state.observed[r.ReservationID]; !exists {
			// Admission/progress may beat the RPC reply on a fast data path or
			// a slow control link. Start the sizing baseline before delivery.
			state.observed[r.ReservationID] = hourlyCreditObserved{bytes: r.AdmittedUploadBytes + r.AdmittedDownloadBytes, at: m.service.now()}
		}
	}
	c.mu.Unlock()
	if retired {
		return ErrUnavailable
	}
	install := hourlyCreditInstall(r)
	if wire.ValidateHourlyInstall(install) != nil {
		return ErrInvalid
	}
	call, cancel := context.WithTimeout(ctx, 4*time.Second)
	started := time.Now()
	raw, err := m.commandPeer(call, peer, "", wire.HourlyCreditInstallCommand, install)
	cancel()
	var result struct {
		Installed bool `json:"installed"`
	}
	if err != nil {
		return err
	}
	if wire.Decode(raw, &result) != nil || !result.Installed {
		return ErrInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil {
		state.delivered[r.ReservationID] = r.IssueSequence
		delete(state.uncertain, r.ReservationID)
		state.work = true
		state.roundTrip = time.Since(started)
	}
	return nil
}

func (m *Master) sealHourlyReservation(ctx context.Context, peer *agentPeer, r model.HourlyCreditReservation) error {
	if !m.currentHourlyCreditPeer(peer) || r.NodeID != peer.nodeID || r.AgentID != peer.agentID {
		return ErrUnavailable
	}
	expected := hourlyCreditInstall(r)
	if wire.ValidateHourlyInstall(expected) != nil {
		return ErrInvalid
	}
	call, cancel := context.WithTimeout(ctx, 4*time.Second)
	raw, err := m.commandPeer(call, peer, "", wire.HourlyCreditSealCommand, wire.HourlyCreditSeal{ReservationID: r.ReservationID, Expected: &expected})
	cancel()
	if err != nil {
		return err
	}
	var proof wire.HourlyCreditProof
	if wire.Decode(raw, &proof) != nil || wire.ValidateHourlyProof(proof) != nil || proof.Install != expected {
		return ErrInvalid
	}
	repo, ok := m.service.repo.(hourlyCreditRepository)
	if !ok {
		return ErrUnavailable
	}
	settled, err := repo.ApplyHourlyCreditProof(ctx, peer.nodeID, peer.agentID, hourlyCreditProofModel(peer.nodeID, proof))
	if err != nil {
		return err
	}
	m.hourlyCreditSettled(peer, settled)
	return m.queueHourlyCreditAck(peer, wire.HourlyCreditAck{ReservationID: settled.ReservationID, IssueSequence: uint64(settled.IssueSequence), Digest: settled.FinalProofDigest})
}

func (m *Master) hourlyCreditSettled(peer *agentPeer, r model.HourlyCreditReservation) {
	c := m.hourlyCredits
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil {
		delete(state.delivered, r.ReservationID)
		delete(state.uncertain, r.ReservationID)
		delete(state.observed, r.ReservationID)
		owner := wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}
		if state.intents[owner].ReservationID == r.ReservationID {
			delete(state.intents, owner)
		}
		state.work = true
	}
}
