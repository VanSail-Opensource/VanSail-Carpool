package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// HourlyExtraRequest is internal server-observed demand, never an HTTP input.
// The buyer resolves administrator pricing and rechecks every financial guard.
type HourlyExtraRequest struct {
	OwnerID, NodeID, DayUTC string
	AccountGeneration       int64
	PendingBytes            int64
	PendingObservedAt       time.Time
}

type HourlyExtraBuyer interface {
	BuyHourlyExtra(context.Context, HourlyExtraRequest) error
}

// Set before starting the master. Calls run in its existing credit worker.
func (s *Service) SetHourlyExtraBuyer(buyer HourlyExtraBuyer) { s.hourlyExtraBuyer = buyer }

func (m *Master) buyDemandedHourlyExtra(ctx context.Context, peer *agentPeer, owner wire.HourlyOwner, day model.HourlyCreditDay) (bool, error) {
	if !m.currentHourlyCreditPeer(peer) || m.service.hourlyExtraBuyer == nil || day.FreeAvailableBytes() > 0 || day.ExtraAvailableBytes() > 0 || day.FreeReservedBytes > 0 || day.ExtraReservedBytes > 0 {
		return false, nil
	}
	now := m.service.now()
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.peers[peer]
	if state == nil || state.retireAll || state.retire[owner] {
		c.mu.Unlock()
		return false, nil
	}
	hint := state.hints[owner]
	c.mu.Unlock()
	if hint.demand.PendingBytes == 0 || now.Before(hint.seenAt) || now.Sub(hint.seenAt) > hourlyCreditDemandFreshness {
		return false, nil
	}
	err := m.service.hourlyExtraBuyer.BuyHourlyExtra(ctx, HourlyExtraRequest{OwnerID: owner.ID, AccountGeneration: int64(owner.Generation), NodeID: peer.nodeID, DayUTC: day.DayUTC, PendingBytes: int64(hint.demand.PendingBytes), PendingObservedAt: hint.seenAt})
	return true, err
}
