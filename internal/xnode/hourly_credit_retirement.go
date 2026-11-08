package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// HourlyCreditAdmissionReady is deliberately stronger than protocol negotiation:
// old/minimal repositories may settle evidence without issuing new credit.
func (m *Master) HourlyCreditAdmissionReady(nodeID string) bool {
	if _, ok := m.service.repo.(hourlyCreditProductionRepository); !ok {
		return false
	}
	m.mu.RLock()
	peer := m.peers[nodeID]
	m.mu.RUnlock()
	return m.currentHourlyCreditPeer(peer)
}

// ResumeHourlyCreditAdmission follows a newly paid/authorized enrollment. It
// clears only a transient owner stop; permanent node retirement and existing
// financial obligations remain unchanged. Demand alone never clears a stop.
func (m *Master) ResumeHourlyCreditAdmission(ownerID string, generation int64) error {
	owner := wire.HourlyOwner{ID: ownerID, Generation: uint64(generation)}
	if generation < 0 || wire.ValidateHourlyOwner(owner) != nil {
		return ErrInvalid
	}
	c := m.hourlyCredits
	if c == nil {
		return ErrUnavailable
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, state := range c.peers {
		delete(state.retire, owner)
		state.work = true
	}
	c.signal()
	return nil
}

// CloseHourlyCreditObligations schedules the existing bounded credit worker.
// Callers fence member admission first and retain the agent credential until
// every obligation settles; offline/unknown reservations are never refunded.
// Generation -1 settles all historical generations within owner/node scope.
func (m *Master) CloseHourlyCreditObligations(ctx context.Context, ownerID string, generation int64, nodeID string) error {
	if ownerID == "" && nodeID == "" || generation < -1 {
		return ErrInvalid
	}
	repo, ok := m.service.repo.(hourlyCreditProductionRepository)
	if !ok {
		return nil
	}
	inFlight := m.retireHourlyCreditScope(ownerID, generation, nodeID)
	rows, err := repo.OutstandingHourlyCredits(ctx, ownerID, generation, nodeID, wire.MaxHourlyItems)
	if err != nil {
		return err
	}
	code := ""
	for _, r := range rows {
		if r.FinalProofDigest != "" && r.UnknownBytes > 0 {
			code = "XNODE_HOURLY_CREDIT_RECOVERY_UNKNOWN"
			break
		}
		m.mu.RLock()
		peer := m.peers[r.NodeID]
		m.mu.RUnlock()
		if !m.currentHourlyCreditPeer(peer) || peer.agentID != r.AgentID {
			code = "XNODE_HOURLY_CREDIT_OFFLINE"
			continue
		}
		m.retireHourlyCreditRecord(peer, r)
		if code == "" {
			code = "XNODE_HOURLY_CREDIT_PROOF_PENDING"
		}
	}
	if code == "" && inFlight {
		code = "XNODE_HOURLY_CREDIT_PROOF_PENDING"
	}
	if code != "" {
		return &PolicyError{code, ErrAccessAccountingPending}
	}
	return nil
}

func (m *Master) retireHourlyCreditScope(owner string, generation int64, node string) bool {
	c := m.hourlyCredits
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	pending := false
	for peer, state := range c.peers {
		if node != "" && peer.nodeID != node {
			continue
		}
		if owner == "" {
			state.retireAll = true
		} else if generation >= 0 {
			markHourlyCreditRetired(state, wire.HourlyOwner{ID: owner, Generation: uint64(generation)})
		}
		for subject := range state.hints {
			if hourlyCreditScopeMatches(subject, owner, generation) {
				markHourlyCreditRetired(state, subject)
			}
		}
		for subject := range state.intents {
			if hourlyCreditScopeMatches(subject, owner, generation) {
				markHourlyCreditRetired(state, subject)
				pending = true
			}
		}
		state.work = true
	}
	c.signal()
	return pending
}

func (m *Master) retireHourlyCreditRecord(peer *agentPeer, r model.HourlyCreditReservation) {
	c := m.hourlyCredits
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.stateLocked(peer, m.service.now()); state != nil {
		markHourlyCreditRetired(state, wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)})
		state.work = true
		c.signal()
	}
}

func markHourlyCreditRetired(state *hourlyCreditPeerState, owner wire.HourlyOwner) {
	if !state.retire[owner] && len(state.retire) >= maxHourlyCreditDemandHints {
		state.retireAll = true
		return
	}
	state.retire[owner] = true
}

func hourlyCreditScopeMatches(subject wire.HourlyOwner, owner string, generation int64) bool {
	return (owner == "" || subject.ID == owner) && (generation == -1 || subject.Generation == uint64(generation))
}
