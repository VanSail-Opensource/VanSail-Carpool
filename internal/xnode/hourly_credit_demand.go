package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const hourlyCreditIdleAfter = 10 * time.Second
const hourlyCreditDemandFreshness = 4 * time.Second

// Hints are coalesced wakeups, never authority to buy or refund traffic. A nil
// batch wakes initial reconnect reconciliation even without an eligible owner.
func (m *Master) requestHourlyCredit(peer *agentPeer, items []wire.HourlyDemand) {
	if !m.currentHourlyCreditPeer(peer) || m.hourlyCredits == nil {
		return
	}
	if _, ok := m.service.repo.(hourlyCreditProductionRepository); !ok {
		return
	}
	if len(items) > wire.MaxHourlyItems {
		return
	}
	now := m.service.now()
	eligible := make([]wire.HourlyDemand, 0, len(items))
	positive := false
	for _, item := range items {
		if wire.ValidateHourlyDemand(item) != nil {
			continue
		}
		if _, ok := m.hourlyPolicy(item.Owner, peer.nodeID, now); ok {
			eligible = append(eligible, item)
			positive = positive || item.PendingBytes > 0
		}
	}
	c := m.hourlyCredits
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.peers[peer]
	if state == nil && len(items) > 0 && !positive {
		return
	}
	state = c.stateLocked(peer, now)
	if state == nil {
		return
	}
	state.work = state.work || !state.scanned
	for owner, hint := range state.hints {
		if !now.Before(hourlyCreditHintLastSeen(hint).Add(hourlyCreditIdleAfter)) {
			delete(state.hints, owner)
			c.hintCount--
		}
	}
	for _, item := range eligible {
		if _, exists := state.hints[item.Owner]; !exists {
			if c.hintCount >= maxHourlyCreditDemandHints {
				continue
			}
			c.hintCount++
		}
		active := state.hints[item.Owner].activeAt
		if item.PendingBytes > 0 {
			active = now
		}
		state.hints[item.Owner] = hourlyCreditHint{demand: item, seenAt: now, activeAt: active}
	}
	if state.work || hourlyCreditHasDemand(state, now) || len(state.pending)+len(state.intents) > 0 {
		c.signal()
	}
}

func hourlyCreditHintLastSeen(hint hourlyCreditHint) time.Time {
	if hint.activeAt.After(hint.seenAt) {
		return hint.activeAt
	}
	return hint.seenAt
}

func hourlyCreditHintDemanding(hint hourlyCreditHint, now time.Time) bool {
	pending := hint.demand.PendingBytes > 0 && !now.Before(hint.seenAt) && now.Sub(hint.seenAt) <= hourlyCreditDemandFreshness
	active := !hint.activeAt.IsZero() && !now.Before(hint.activeAt) && now.Sub(hint.activeAt) <= hourlyCreditDemandFreshness
	return pending || active
}

func hourlyCreditHasDemand(state *hourlyCreditPeerState, now time.Time) bool {
	if state.retireAll {
		return false
	}
	for owner, hint := range state.hints {
		if !state.retire[owner] && hourlyCreditHintDemanding(hint, now) {
			return true
		}
	}
	return false
}
