package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) enqueueHourlyGrants(grants []hourly.Grant) {
	for _, grant := range grants {
		m.mu.RLock()
		peer := m.peers[grant.Peer.NodeID]
		m.mu.RUnlock()
		if peer == nil || peer.hourlySender == nil || hourlyPeer(peer) != grant.Peer || !m.currentHourlyPeer(peer) {
			continue
		}
		if peer.hourlySender.offer(m.service.now(), grant) {
			m.mu.Lock()
			live := m.ctx != nil && m.ctx.Err() == nil && m.peers[peer.nodeID] == peer
			if live {
				m.wg.Add(1)
			}
			m.mu.Unlock()
			if live {
				go m.runHourlyGrantSender(peer)
			} else {
				peer.hourlySender.stop()
			}
		}
	}
}

func (m *Master) runHourlyGrantSender(peer *agentPeer) {
	defer m.wg.Done()
	for peer.ctx.Err() == nil {
		grant, ok := peer.hourlySender.next(m.service.now())
		if !ok {
			return
		}
		if m.sendHourlyGrant(peer, grant) != nil {
			peer.hourlySender.complete(grant)
			peer.hourlySender.stop()
			return // The allocator can retry the exact offer; no reservation is freed.
		}
		peer.hourlySender.complete(grant)
	}
	peer.hourlySender.stop()
}

func (m *Master) sendHourlyGrant(peer *agentPeer, grant hourly.Grant) error {
	if !m.currentHourlyPeer(peer) || hourlyPeer(peer) != grant.Peer {
		return ErrUnavailable
	}
	owner := wire.HourlyOwner{ID: grant.Owner.ID, Generation: grant.Owner.Generation}
	policy, ok := m.hourlyPolicy(owner, peer.nodeID, m.service.now())
	if !ok || grant.ConservativeUntil.After(policy.PaidThrough) || grant.ConservativeUntil.After(policy.NodePaidThrough[peer.nodeID]) {
		return ErrUnavailable
	}
	frame := wire.HourlyGrant{SessionID: peer.hourlySession, Owner: owner, Nonce: grant.Nonce, Sequence: grant.Sequence, DayUTC: grant.DayUTC, LifetimeMillis: uint32(grant.Lifetime / time.Millisecond), RateBytesPerSecond: grant.RateBytesPerSecond, BurstBytes: grant.BurstBytes}
	if wire.ValidateHourlyGrant(frame) != nil || !grant.ConservativeUntil.After(m.service.now()) {
		return ErrInvalid
	}
	ctx, cancel := context.WithDeadline(peer.ctx, grant.ConservativeUntil)
	defer cancel()
	// Each frame joins the existing per-node queue separately; waiting happens
	// outside all master, allocator, policy and sender locks.
	return peer.send(ctx, wire.New("hourly.grant", "", 0, frame))
}
