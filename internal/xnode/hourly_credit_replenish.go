package xnode

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const hourlyCreditQuantum int64 = 64 << 20

func (m *Master) replenishHourlyCredits(ctx context.Context, peer *agentPeer, repo hourlyCreditProductionRepository, limit int) error {
	for _, owner := range m.hourlyCreditOwners(peer, limit) {
		if ctx.Err() != nil || !m.currentHourlyCreditPeer(peer) {
			return ctx.Err()
		}
		if err := m.replenishHourlyOwner(ctx, peer, repo, owner); err != nil {
			if errors.Is(err, model.ErrHourlyCreditUnavailable) {
				continue
			}
			return err
		}
	}
	return nil
}

func (m *Master) hourlyCreditOwners(peer *agentPeer, limit int) []wire.HourlyOwner {
	now := m.service.now()
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil || state.retireAll {
		return nil
	}
	var owners []wire.HourlyOwner
	for owner, hint := range state.hints {
		if !now.Before(hourlyCreditHintLastSeen(hint).Add(hourlyCreditIdleAfter)) {
			delete(state.hints, owner)
			c.hintCount--
			continue
		}
		if !state.retire[owner] && hourlyCreditHintDemanding(hint, now) {
			owners = append(owners, owner)
		}
	}
	sort.Slice(owners, func(i, j int) bool {
		if owners[i].ID == owners[j].ID {
			return owners[i].Generation < owners[j].Generation
		}
		return owners[i].ID < owners[j].ID
	})
	if len(owners) == 0 {
		return nil
	}
	start := int(state.ownerCursor % uint64(len(owners)))
	count := min(limit, len(owners))
	result := make([]wire.HourlyOwner, 0, count)
	for n := 0; n < count; n++ {
		result = append(result, owners[(start+n)%len(owners)])
	}
	state.ownerCursor += uint64(count)
	return result
}

func (m *Master) replenishHourlyOwner(ctx context.Context, peer *agentPeer, repo hourlyCreditProductionRepository, owner wire.HourlyOwner) error {
	policy, eligible := m.hourlyPolicy(owner, peer.nodeID, m.service.now())
	if !eligible {
		return nil
	}
	c := m.hourlyCredits
	c.mu.Lock()
	state := c.peers[peer]
	if state == nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	request, hasIntent := state.intents[owner]
	c.mu.Unlock()
	if hasIntent {
		return m.resolveHourlyCreditIntent(ctx, peer, repo, owner, request)
	}
	dayUTC := m.service.now().UTC().Format(time.DateOnly)
	scope := model.HourlyCreditRequest{OwnerID: owner.ID, AccountGeneration: int64(owner.Generation), DayUTC: dayUTC, NodeID: peer.nodeID, AgentID: peer.agentID, BootID: peer.bootID, SessionID: peer.hourlySession}
	held, err := repo.HourlyCreditHeldBytes(ctx, scope)
	if err != nil {
		return err
	}
	day, err := repo.EnsureHourlyCreditDay(ctx, owner.ID, int64(owner.Generation), dayUTC, policy.DailyAllowanceBytes)
	if err != nil {
		return err
	}
	kind, available, reserved := "free", day.FreeAvailableBytes(), day.FreeReservedBytes
	if available <= 0 {
		kind, available, reserved = "extra", day.ExtraAvailableBytes(), day.ExtraReservedBytes
	}
	if available <= 0 {
		return nil
	}
	target := m.hourlyCreditTarget(peer, owner, available+reserved)
	if held > target/2 {
		return nil
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	request = scope
	request.ReservationID = id
	request.Kind, request.ByteCeiling = kind, min(target-held, available)
	c.mu.Lock()
	state = c.peers[peer]
	if state == nil || state.retireAll || state.retire[owner] {
		c.mu.Unlock()
		return ErrUnavailable
	}
	if len(state.delivered)+len(state.pending)+len(state.uncertain)+len(state.intents) >= maxPendingHourlyCreditAcks {
		c.mu.Unlock()
		return ErrUnavailable
	}
	state.intents[owner] = request
	state.work = true
	c.mu.Unlock()
	r, err := repo.ReserveHourlyCredit(ctx, request)
	if err != nil {
		return err
	}
	return m.resumeHourlyCreditIntent(ctx, peer, owner, r)
}

func (m *Master) resumeHourlyCreditIntent(ctx context.Context, peer *agentPeer, owner wire.HourlyOwner, r model.HourlyCreditReservation) error {
	if r.OwnerID != owner.ID || uint64(r.AccountGeneration) != owner.Generation || r.NodeID != peer.nodeID || r.AgentID != peer.agentID {
		return ErrConflict
	}
	if r.FinalProofDigest != "" {
		m.hourlyCreditSettled(peer, r)
		return m.queueHourlyCreditAck(peer, wire.HourlyCreditAck{ReservationID: r.ReservationID, IssueSequence: uint64(r.IssueSequence), Digest: r.FinalProofDigest})
	}
	seal, _ := m.hourlyCreditAction(peer, r)
	var err error
	if seal {
		err = m.sealHourlyReservation(ctx, peer, r)
	} else {
		err = m.installHourlyReservation(ctx, peer, r)
	}
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil {
		if err != nil {
			if !errors.Is(err, errHourlyCreditPredecessor) {
				state.uncertain[r.ReservationID] = true
			}
		} else if state.intents[owner].ReservationID == r.ReservationID {
			delete(state.intents, owner)
		}
		state.work = true
	}
	return err
}
