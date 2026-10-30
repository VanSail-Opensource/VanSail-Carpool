package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"sync/atomic"
	"time"
)

func (peer *agentPeer) send(ctx context.Context, message wire.Message) error {
	_, err := peer.sendTracked(ctx, message)
	return err
}
func (peer *agentPeer) sendTracked(ctx context.Context, message wire.Message) (bool, error) {
	// 0 is queued, 1 may have written, 2 was abandoned before any write.
	var state atomic.Int32
	queued := time.Now()
	err := upstreamqueue.Execute(ctx, peer.queue, func(callCtx context.Context) error {
		started := time.Now()
		defer func() {
			if peer.logger != nil {
				peer.logger.Debug("node outbound frame", "node_id", peer.nodeID, "type", message.Type, "queue_ms", started.Sub(queued).Milliseconds(), "write_ms", time.Since(started).Milliseconds())
			}
		}()
		if callCtx.Err() != nil {
			return callCtx.Err()
		}
		deadline := time.Now().Add(10 * time.Second)
		if value, ok := callCtx.Deadline(); ok && value.Before(deadline) {
			deadline = value
		}
		if err := peer.socket.SetWriteDeadline(deadline); err != nil {
			return err
		}
		if !state.CompareAndSwap(0, 1) {
			return context.Canceled
		}
		return peer.socket.WriteJSON(message)
	})
	if err != nil {
		state.CompareAndSwap(0, 2)
	}
	return state.Load() == 1, err
}
