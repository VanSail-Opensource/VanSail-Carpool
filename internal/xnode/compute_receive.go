package xnode

import (
	"context"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (m *Master) receiveCompute(ctx context.Context, peer *agentPeer, message wire.Message) error {
	if !peer.computePool || len(message.Data) > 96<<10 {
		return ErrInvalid
	}
	m.mu.RLock()
	current := m.peers[peer.nodeID] == peer
	live := m.live[peer.nodeID]
	eligible := live.report.Ready && live.report.Synchronized && !live.receivedAt.IsZero() && time.Since(live.receivedAt) < time.Minute
	m.mu.RUnlock()
	if !current {
		return ErrConflict
	}
	var signal wire.ComputeSignal
	var chunk wire.ComputeChunk
	if message.Type == "compute.result" {
		if wire.Decode(message.Data, &chunk) != nil {
			return ErrInvalid
		}
		signal = chunk.Signal
	} else if wire.Decode(message.Data, &signal) != nil {
		return ErrInvalid
	}
	p := m.compute
	p.mu.Lock()
	a := p.attempts[signal.BatchID]
	if a == nil || a.offer.Signal != signal {
		receipt, seen := p.accepted[signal]
		p.mu.Unlock()
		if seen && receipt.peer == peer && time.Now().Before(receipt.until) {
			return peer.send(ctx, wire.New("compute.ack", signal.BatchID, 0, signal))
		}
		return nil
	}
	if message.Type == "compute.claim" {
		if eligible && a.winner == nil && p.busy[peer] == "" {
			a.winner = peer
			a.lease = time.Now().Add(60 * time.Second)
			p.busy[peer] = signal.BatchID
			a.claimed <- peer
			p.mu.Unlock()
			return nil
		}
		winner := a.winner == peer
		p.mu.Unlock()
		if !winner {
			return peer.send(ctx, wire.New("compute.cancel", signal.BatchID, 0, signal))
		}
		return nil
	}
	if a.winner != peer {
		p.mu.Unlock()
		return nil
	}
	if message.Type == "compute.cancel" {
		a.lease = time.Time{}
		p.mu.Unlock()
		return nil
	}
	if message.Type == "compute.heartbeat" {
		a.lease = time.Now().Add(60 * time.Second)
		p.mu.Unlock()
		return nil
	}
	if a.uploadComplete {
		p.mu.Unlock()
		return nil
	}
	select {
	case a.chunks <- chunk:
		p.mu.Unlock()
		return nil
	default:
		p.mu.Unlock()
		return ErrUnavailable
	}
}
