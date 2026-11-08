package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receiveHourly(ctx context.Context, peer *agentPeer, message wire.Message, receivedAt time.Time) error {
	if len(message.Data) > wire.MaxHourlyFrameBytes || !m.currentHourlyPeer(peer) {
		return ErrInvalid
	}
	switch message.Type {
	case "hourly.demand":
		var batch wire.HourlyDemandBatch
		if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyDemandBatch(batch) != nil || batch.SessionID != peer.hourlySession {
			return ErrInvalid
		}
		demands := make([]hourlyInboundDemand, len(batch.Demands))
		for i, demand := range batch.Demands {
			demands[i] = hourlyInboundDemand{demand: demand, receivedAt: receivedAt}
		}
		m.observeHourlyDemands(peer, demands)
		if len(batch.Demands) > 0 {
			m.requestHourlyCredit(peer, batch.Demands)
		}
		// Financial progress remains admissible after enrollment/owner closure.
		return m.receiveHourlyCreditProgress(ctx, peer, batch.Progress)
	case "hourly.grant.ack":
		var ack wire.HourlyGrantAck
		if wire.Decode(message.Data, &ack) != nil || wire.ValidateHourlyGrantAck(ack) != nil || ack.SessionID != peer.hourlySession {
			return ErrInvalid
		}
		m.acknowledgeHourlyGrant(peer, ack)
		return nil
	case "hourly.grant.acks":
		var batch wire.HourlyGrantAckBatch
		if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyGrantAckBatch(batch) != nil || batch.SessionID != peer.hourlySession {
			return ErrInvalid
		}
		m.acknowledgeHourlyGrants(peer, batch.Acks)
		return nil
	case "hourly.credit.proofs":
		return m.receiveHourlyCreditProofs(ctx, peer, message)
	default:
		return ErrInvalid
	}
}

func (m *Master) observeHourlyDemand(peer *agentPeer, demand wire.HourlyDemand, receivedAt time.Time) {
	m.observeHourlyDemands(peer, []hourlyInboundDemand{{demand: demand, receivedAt: receivedAt}})
}

func (m *Master) observeHourlyDemands(peer *agentPeer, inputs []hourlyInboundDemand) {
	if len(inputs) == 0 || len(inputs) > hourly.MaxBatchItems {
		return
	}
	at := m.service.now()
	demands := make([]hourly.Demand, 0, len(inputs))
	for _, input := range inputs {
		demand, receivedAt := input.demand, input.receivedAt
		policy, ok := m.hourlyPolicy(demand.Owner, peer.nodeID, at)
		if !ok {
			continue
		}
		lifetime := time.Duration(demand.LifetimeMillis) * time.Millisecond
		lifetime = min(lifetime, policy.PaidThrough.Sub(receivedAt), policy.NodePaidThrough[peer.nodeID].Sub(receivedAt)).Truncate(time.Millisecond)
		if lifetime <= 0 {
			continue
		}
		demands = append(demands, hourly.Demand{Owner: hourly.Owner{ID: demand.Owner.ID, Generation: demand.Owner.Generation}, Peer: hourlyPeer(peer), Nonce: demand.Nonce, ReceivedAt: receivedAt, Lifetime: lifetime, PendingBytes: demand.PendingBytes, MinAtomicBytes: demand.MinAtomicBytes})
	}
	co := m.hourly
	co.allocatorMu.Lock()
	defer co.allocatorMu.Unlock()
	_, _ = co.allocator.ObserveBatch(m.service.now(), demands)
}

func (m *Master) acknowledgeHourlyGrant(peer *agentPeer, ack wire.HourlyGrantAck) {
	m.acknowledgeHourlyGrants(peer, []wire.HourlyGrantAck{ack})
}

func (m *Master) acknowledgeHourlyGrants(peer *agentPeer, inputs []wire.HourlyGrantAck) {
	if len(inputs) == 0 || len(inputs) > hourly.MaxBatchItems {
		return
	}
	acks := make([]hourly.Ack, len(inputs))
	for i, ack := range inputs {
		acks[i] = hourly.Ack{Owner: hourly.Owner{ID: ack.Owner.ID, Generation: ack.Owner.Generation}, Peer: hourlyPeer(peer), Nonce: ack.Nonce, Sequence: ack.Sequence, Applied: ack.Applied}
	}
	co := m.hourly
	co.allocatorMu.Lock()
	defer co.allocatorMu.Unlock()
	_, _ = co.allocator.AcknowledgeBatch(m.service.now(), acks)
}
