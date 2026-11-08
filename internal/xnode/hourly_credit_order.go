package xnode

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

var errHourlyCreditPredecessor = errors.New("earlier hourly credit reconciliation pending")

// The SQL AgentID sequence lock orders uncertain old commits before this
// reservation. Re-read lower issues after allocation so a replacement peer's
// earlier snapshot cannot hide an uninstalled obligation behind a newer grant.
func (m *Master) reconcileHourlyCreditPredecessors(ctx context.Context, peer *agentPeer, current model.HourlyCreditReservation) error {
	repo, ok := m.service.repo.(hourlyCreditProductionRepository)
	if !ok {
		return ErrUnavailable
	}
	after, changes := int64(0), 0
	for range maxPendingHourlyCreditAcks/wire.MaxHourlyItems + 1 {
		rows, err := repo.UnfinalizedHourlyCredits(ctx, peer.nodeID, peer.agentID, after, wire.MaxHourlyItems)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.IssueSequence >= current.IssueSequence {
				return nil
			}
			seal, deliver := m.hourlyCreditAction(peer, r)
			if seal || deliver {
				if changes >= maxHourlyCreditChangesPerPass {
					return errHourlyCreditPredecessor
				}
				changes++
				if seal {
					err = m.sealHourlyReservation(ctx, peer, r)
				} else {
					err = m.sendHourlyCreditInstall(ctx, peer, r)
				}
				if err != nil {
					m.markHourlyCreditUncertain(peer, r.ReservationID, err)
					return errHourlyCreditPredecessor
				}
			}
			after = r.IssueSequence
		}
		if len(rows) < wire.MaxHourlyItems {
			return nil
		}
	}
	return errHourlyCreditPredecessor
}

func (m *Master) markHourlyCreditUncertain(peer *agentPeer, id string, err error) {
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil {
		if !errors.Is(err, errHourlyCreditPredecessor) {
			state.uncertain[id] = true
		}
		state.work = true
	}
}
