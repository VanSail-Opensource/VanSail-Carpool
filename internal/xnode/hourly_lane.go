package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Live controls never wait behind financial SQL. Both lanes retain bounded
// decoded state; overloaded evidence replays without an application ACK.
type hourlyInboundLane struct {
	live      *hourlyLiveLane
	financial *hourlyFinancialLane
}

func newHourlyInboundLane() *hourlyInboundLane {
	return &hourlyInboundLane{live: newHourlyLiveLane(), financial: newHourlyFinancialLane()}
}

func (r *peerReceivers) offerHourly(message wire.Message) error {
	if r.hourly == nil || len(message.Data) > wire.MaxHourlyFrameBytes || !r.master.currentHourlyPeer(r.peer) {
		return ErrInvalid
	}
	at := r.master.service.now()
	switch message.Type {
	case "hourly.demand":
		var batch wire.HourlyDemandBatch
		if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyDemandBatch(batch) != nil || batch.SessionID != r.peer.hourlySession {
			return ErrInvalid
		}
		if err := r.hourly.financial.offerProgress(batch.Progress); err != nil {
			return err
		}
		r.hourly.live.offerDemands(batch.Demands, at)
	case "hourly.grant.ack":
		var ack wire.HourlyGrantAck
		if wire.Decode(message.Data, &ack) != nil || wire.ValidateHourlyGrantAck(ack) != nil || ack.SessionID != r.peer.hourlySession {
			return ErrInvalid
		}
		return r.hourly.live.offerAcks([]wire.HourlyGrantAck{ack})
	case "hourly.grant.acks":
		var batch wire.HourlyGrantAckBatch
		if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyGrantAckBatch(batch) != nil || batch.SessionID != r.peer.hourlySession {
			return ErrInvalid
		}
		return r.hourly.live.offerAcks(batch.Acks)
	case "hourly.credit.proofs":
		var batch wire.HourlyCreditProofBatch
		if wire.Decode(message.Data, &batch) != nil || wire.ValidateHourlyProofBatch(batch) != nil {
			return ErrInvalid
		}
		for _, proof := range batch.Proofs {
			if proof.Install.AgentID != r.peer.agentID {
				return ErrInvalid
			}
		}
		return r.hourly.financial.offerProofs(batch.Proofs)
	default:
		return ErrInvalid
	}
	return nil
}

func (r *peerReceivers) startHourly(ctx context.Context) {
	r.wg.Add(2)
	go func() { defer r.wg.Done(); r.runHourlyLive(ctx) }()
	go func() { defer r.wg.Done(); r.runHourlyFinancial(ctx) }()
}

func (r *peerReceivers) drainHourly() {
	if r.hourly != nil {
		r.hourly.live.clear()
		r.hourly.financial.clear()
	}
}

func hourlyLaneSignal(wake chan struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}
