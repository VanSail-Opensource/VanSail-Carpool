package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maxHourlyCreditChangesPerPass = 4

func (m *Master) produceHourlyCredits(peer *agentPeer) error {
	repo, ok := m.service.repo.(hourlyCreditProductionRepository)
	if !ok || !m.currentHourlyCreditPeer(peer) {
		return nil
	}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.peers[peer]
	if state == nil {
		c.mu.Unlock()
		return nil
	}
	after := state.scanIssue
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(peer.ctx, 20*time.Second)
	defer cancel()
	changes, err := m.resolveHourlyCreditIntents(ctx, peer, repo)
	if err != nil || changes >= maxHourlyCreditChangesPerPass {
		return err
	}
	rows, err := repo.UnfinalizedHourlyCredits(ctx, peer.nodeID, peer.agentID, after, wire.MaxHourlyItems)
	if err != nil {
		return err
	}
	last, visited := after, 0
	for _, r := range rows {
		seal, deliver := m.hourlyCreditAction(peer, r)
		if seal || deliver {
			if changes >= maxHourlyCreditChangesPerPass {
				break
			}
			changes++
			if seal {
				err = m.sealHourlyReservation(ctx, peer, r)
			} else {
				err = m.installHourlyReservation(ctx, peer, r)
			}
			if err != nil {
				m.markHourlyCreditUncertain(peer, r.ReservationID, err)
				return err
			}
		}
		last = r.IssueSequence
		visited++
	}
	c.mu.Lock()
	state = c.peers[peer]
	if state == nil {
		c.mu.Unlock()
		return nil
	}
	state.scanIssue = last
	if visited == len(rows) && len(rows) < wire.MaxHourlyItems {
		state.scanIssue = 0
		state.scanned = true
	}
	state.work = len(rows) > 0
	ready := state.scanned
	c.mu.Unlock()
	if ready && changes < maxHourlyCreditChangesPerPass {
		return m.replenishHourlyCredits(ctx, peer, repo, maxHourlyCreditChangesPerPass-changes)
	}
	return nil
}

func (m *Master) hourlyCreditAction(peer *agentPeer, r model.HourlyCreditReservation) (seal, deliver bool) {
	now := m.service.now()
	owner := wire.HourlyOwner{ID: r.OwnerID, Generation: uint64(r.AccountGeneration)}
	_, eligible := m.hourlyPolicy(owner, peer.nodeID, now)
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil {
		return false, false
	}
	seen := state.createdAt
	if hint, ok := state.hints[owner]; ok && !hint.activeAt.IsZero() {
		seen = hint.activeAt
	}
	idle := !now.Before(seen.Add(hourlyCreditIdleAfter))
	seal = state.retireAll || state.retire[owner] || !eligible || r.DayUTC != now.UTC().Format(time.DateOnly) || r.BootID != peer.bootID || r.SessionID != peer.hourlySession || r.ReservedBytes() == 0 || state.uncertain[r.ReservationID] || idle
	deliver = !seal && state.delivered[r.ReservationID] != r.IssueSequence
	return seal, deliver
}
