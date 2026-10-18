package xnode

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Command waits outside the outbound queue so results cannot block traffic acknowledgments.
func (m *Master) Command(ctx context.Context, nodeID, id, name string, args any) (json.RawMessage, error) {
	m.mu.RLock()
	peer := m.peers[nodeID]
	m.mu.RUnlock()
	if peer == nil {
		return nil, &CommandError{CommandUndelivered, ErrUnavailable}
	}
	if id == "" {
		var err error
		id, err = ids.New()
		if err != nil {
			return nil, err
		}
	}
	known := false
	for _, command := range wire.CommandNames() {
		if name == command {
			known = true
		}
	}
	if !known {
		return nil, ErrInvalid
	}
	pending := make(chan wire.Message, 4)
	peer.mu.Lock()
	if peer.pending[id] != nil {
		peer.mu.Unlock()
		return nil, ErrConflict
	}
	peer.pending[id] = pending
	peer.mu.Unlock()
	defer func() { peer.mu.Lock(); delete(peer.pending, id); peer.mu.Unlock() }()
	delivery, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	started, err := peer.sendTracked(delivery, wire.New("command", id, 0, map[string]any{"name": name, "args": args}))
	if err != nil {
		outcome := CommandUndelivered
		if started {
			outcome = CommandUncertain
		}
		return nil, &CommandError{outcome, err}
	}
	return waitCommand(ctx, peer, pending, id, name, timer.C)
}

func (m *Master) Flush(ctx context.Context, nodeID string, user int64, quiesce bool) (wire.TrafficBarrier, error) {
	return m.flush(ctx, nodeID, wire.TrafficFlush{UserID: int(user), Quiesce: quiesce})
}
func (m *Master) FlushPeriod(ctx context.Context, nodeID string, user int64, period string) (wire.TrafficBarrier, error) {
	return m.flush(ctx, nodeID, wire.TrafficFlush{UserID: int(user), Quiesce: true, AccountingPeriod: period})
}
func (m *Master) flush(ctx context.Context, nodeID string, args wire.TrafficFlush) (wire.TrafficBarrier, error) {
	var barrier wire.TrafficBarrier
	raw, err := m.Command(ctx, nodeID, "", "traffic.flush", args)
	if err != nil {
		return barrier, err
	}
	if err = wire.Decode(raw, &barrier); err != nil || args.Quiesce && !barrier.Quiesced || barrier.AccountingPeriod != args.AccountingPeriod {
		return barrier, ErrInvalid
	}
	node, err := m.service.repo.XNodeNode(ctx, nodeID)
	if err != nil {
		return barrier, err
	}
	if barrier.AgentID != node.AgentID {
		return barrier, ErrInvalid
	}
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		sequence, err := m.service.repo.XNodeReportSequence(ctx, barrier.AgentID)
		if err != nil {
			return barrier, err
		}
		if sequence >= barrier.Sequence {
			return barrier, nil
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			return barrier, ctx.Err()
		}
	}
}
