package xnode

import (
	"context"
	"sort"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) runHourlyCredits(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-m.hourlyCredits.wake:
		}
		for _, peer := range m.hourlyCredits.ready(m.service.now()) {
			m.mu.Lock()
			live := m.ctx != nil && m.ctx.Err() == nil && m.peers[peer.nodeID] == peer
			if live {
				m.wg.Add(1)
			}
			m.mu.Unlock()
			if !live {
				m.hourlyCredits.finished(peer, true, false, m.service.now())
				continue
			}
			go func() { defer m.wg.Done(); m.deliverHourlyCreditAcks(peer) }()
		}
	}
}

func (c *hourlyCreditCoordinator) ready(now time.Time) []*agentPeer {
	c.mu.Lock()
	defer c.mu.Unlock()
	var peers []*agentPeer
	for peer, state := range c.peers {
		if peer.ctx.Err() != nil && !state.running {
			c.hintCount -= len(state.hints)
			delete(c.peers, peer)
			continue
		}
		for owner, hint := range state.hints {
			if !now.Before(hourlyCreditHintLastSeen(hint).Add(hourlyCreditIdleAfter)) {
				delete(state.hints, owner)
				c.hintCount--
			}
		}
		if !state.running && (len(state.pending) > 0 || state.work || hourlyCreditHasDemand(state, now) || len(state.intents) > 0) && !state.retryAt.After(now) {
			peers = append(peers, peer)
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].nodeID < peers[j].nodeID })
	if len(peers) == 0 {
		return nil
	}
	start := int(c.cursor % uint64(len(peers)))
	count := min(len(peers), max(0, maxHourlyCreditWorkers-c.workers))
	selected := make([]*agentPeer, 0, count)
	for n := 0; n < len(peers) && len(selected) < count; n++ {
		peer := peers[(start+n)%len(peers)]
		if c.activeAgents[peer.agentID] != nil {
			continue
		}
		c.peers[peer].running = true
		c.activeAgents[peer.agentID] = peer
		selected = append(selected, peer)
	}
	c.workers += len(selected)
	c.cursor += uint64(max(1, len(selected)))
	return selected
}

func (c *hourlyCreditCoordinator) finished(peer *agentPeer, discard, failed bool, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.peers[peer]; state != nil && state.running {
		state.running = false
		state.retryAt = time.Time{}
		if failed {
			state.retryAt = now.Add(time.Second)
		}
		c.workers--
		if c.activeAgents[peer.agentID] == peer {
			delete(c.activeAgents, peer.agentID)
		}
		if discard || len(state.pending)+len(state.hints)+len(state.intents)+len(state.retire) == 0 && !state.retireAll && !state.work {
			c.hintCount -= len(state.hints)
			delete(c.peers, peer)
		}
	}
}

func (m *Master) deliverHourlyCreditAcks(peer *agentPeer) {
	c := m.hourlyCredits
	failed := false
	defer func() { c.finished(peer, peer.ctx.Err() != nil, failed, m.service.now()) }()
	for range 8 {
		if !m.currentHourlyCreditPeer(peer) {
			return
		}
		c.mu.Lock()
		state := c.peers[peer]
		var ack wire.HourlyCreditAck
		if state != nil {
			for _, item := range state.pending {
				if ack.ReservationID == "" || item.IssueSequence < ack.IssueSequence {
					ack = item
				}
			}
		}
		c.mu.Unlock()
		if ack.ReservationID == "" {
			break
		}
		ctx, cancel := context.WithTimeout(peer.ctx, 4*time.Second)
		// A fresh RPC identity permits retry after an uncertain generic command
		// receipt. The credit store's exact digest remains the replay authority.
		raw, err := m.commandPeer(ctx, peer, "", wire.HourlyCreditAckCommand, ack)
		cancel()
		var result struct {
			Acknowledged bool `json:"acknowledged"`
		}
		if err != nil || wire.Decode(raw, &result) != nil || !result.Acknowledged {
			failed = true
			return
		}
		c.mu.Lock()
		if state = c.peers[peer]; state != nil && state.pending[ack.ReservationID] == ack {
			delete(state.pending, ack.ReservationID)
		}
		c.mu.Unlock()
	}
	failed = m.produceHourlyCredits(peer) != nil
}
