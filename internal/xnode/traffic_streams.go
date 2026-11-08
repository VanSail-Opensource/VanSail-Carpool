package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Stream metadata is durable accounting evidence, independently of volatile
// report queues. Legacy repositories/agents retain their original cursor path.
type TrafficStreamRepository interface {
	RegisterXNodeTrafficStream(context.Context, string, string, string) error
	XNodeStreamSequence(context.Context, string, string) (uint64, bool, error)
	SetXNodeStreamRate(context.Context, string, string, string, uint64, int64) error
}

func (m *Master) trafficBarrierCommitted(ctx context.Context, barrier wire.TrafficBarrier) (bool, error) {
	if barrier.StreamID == "" {
		sequence, err := m.service.repo.XNodeReportSequence(ctx, barrier.AgentID)
		return sequence >= barrier.Sequence, err
	}
	if barrier.LegacySequence > 0 {
		sequence, err := m.service.repo.XNodeReportSequence(ctx, barrier.AgentID)
		if err != nil || sequence < barrier.LegacySequence {
			return false, err
		}
	}
	streams, ok := m.service.repo.(TrafficStreamRepository)
	if !ok {
		return false, ErrUnavailable
	}
	sequence, closed, err := streams.XNodeStreamSequence(ctx, barrier.AgentID, barrier.StreamID)
	// Closed boot streams settle only their surviving committed usage. Reopening
	// or jumping a cursor would fabricate usage that the agent can no longer send.
	return closed || sequence >= barrier.Sequence, err
}
