package xnode

import (
	"context"
	"errors"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) resolveHourlyCreditIntents(ctx context.Context, peer *agentPeer, repo hourlyCreditProductionRepository) (int, error) {
	c := m.hourlyCredits
	c.mu.Lock()
	requests := map[wire.HourlyOwner]model.HourlyCreditRequest{}
	var owners []wire.HourlyOwner
	if state := c.peers[peer]; state != nil {
		for owner, request := range state.intents {
			owners = append(owners, owner)
			requests[owner] = request
		}
	}
	c.mu.Unlock()
	sort.Slice(owners, func(i, j int) bool { return requests[owners[i]].ReservationID < requests[owners[j]].ReservationID })
	if len(owners) == 0 {
		return 0, nil
	}
	count := min(len(owners), maxHourlyCreditChangesPerPass)
	c.mu.Lock()
	start := 0
	if state := c.peers[peer]; state != nil {
		start = int(state.intentCursor % uint64(len(owners)))
		state.intentCursor += uint64(count)
	}
	c.mu.Unlock()
	changes := 0
	for n := 0; n < count; n++ {
		owner := owners[(start+n)%len(owners)]
		if err := m.resolveHourlyCreditIntent(ctx, peer, repo, owner, requests[owner]); err != nil {
			if errors.Is(err, model.ErrHourlyCreditUnavailable) {
				continue
			}
			return changes, err
		}
		changes++
	}
	return changes, nil
}

func (m *Master) resolveHourlyCreditIntent(ctx context.Context, peer *agentPeer, repo hourlyCreditProductionRepository, owner wire.HourlyOwner, request model.HourlyCreditRequest) error {
	stored, err := repo.ResolveHourlyCreditIntent(ctx, request)
	if errors.Is(err, model.ErrHourlyCreditNotFound) {
		m.forgetHourlyCreditIntent(peer, owner, request)
		return nil
	}
	if errors.Is(err, model.ErrHourlyCreditUnavailable) {
		stored, err = repo.ReserveHourlyCredit(ctx, request)
		if errors.Is(err, model.ErrHourlyCreditNotReserved) {
			m.forgetHourlyCreditIntent(peer, owner, request)
			return nil
		}
	}
	if err != nil {
		return err
	}
	return m.resumeHourlyCreditIntent(ctx, peer, owner, stored)
}

func (m *Master) forgetHourlyCreditIntent(peer *agentPeer, owner wire.HourlyOwner, request model.HourlyCreditRequest) {
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil && state.intents[owner] == request {
		delete(state.intents, owner)
	}
}
