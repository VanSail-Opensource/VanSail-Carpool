package xnode

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type inboundLane struct {
	frames chan wire.Message
	bytes  atomic.Int64
}

func newInboundLane(capacity int) *inboundLane {
	return &inboundLane{frames: make(chan wire.Message, capacity)}
}

func (lane *inboundLane) offer(message wire.Message) bool {
	size := int64(len(message.Data) + 128)
	if lane.bytes.Add(size) <= 32<<20 {
		select {
		case lane.frames <- message:
			return true
		default:
		}
	}
	lane.bytes.Add(-size)
	return false
}

// Independent bounded consumers keep database accounting, disposable telemetry,
// logs and command results from blocking one another at the WebSocket reader.
type peerReceivers struct {
	master    *Master
	peer      *agentPeer
	compute   *inboundLane
	control   *inboundLane
	traffic   *inboundLane
	logs      *inboundLane
	telemetry chan wire.Message
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func (m *Master) startPeerReceivers(peer *agentPeer) *peerReceivers {
	ctx, cancel := context.WithCancel(peer.ctx)
	r := &peerReceivers{master: m, peer: peer, compute: newInboundLane(24), control: newInboundLane(64), traffic: newInboundLane(8), logs: newInboundLane(8), telemetry: make(chan wire.Message, 1), cancel: cancel}
	start := func(frames <-chan wire.Message, lane *inboundLane) {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case message := <-frames:
					if lane != nil {
						lane.bytes.Add(-int64(len(message.Data) + 128))
					}
					if ctx.Err() != nil {
						return
					}
					if err := m.receive(ctx, peer, message); err != nil {
						m.rejectPeerFrame(peer, message, err)
					}
				}
			}
		}()
	}
	start(r.compute.frames, r.compute)
	start(r.control.frames, r.control)
	start(r.traffic.frames, r.traffic)
	start(r.logs.frames, r.logs)
	start(r.telemetry, nil)
	return r
}

func (r *peerReceivers) stop() { r.cancel(); r.wg.Wait() }

func (r *peerReceivers) offer(message wire.Message) error {
	switch message.Type {
	case "compute.claim", "compute.heartbeat", "compute.result", "compute.cancel":
		if len(message.Data) > 96<<10 {
			return ErrInvalid
		}
		if r.compute.offer(message) {
			return nil
		}
	case "telemetry.report", "telemetry.compact":
		select {
		case <-r.telemetry:
		default:
		}
		r.telemetry <- message
		return nil
	case "traffic.report":
		if r.traffic.offer(message) {
			return nil
		}
	case "logs.report":
		if len(message.Data) > 256<<10 {
			r.master.rejectPeerFrame(r.peer, message, ErrInvalid)
			return nil
		}
		if !r.logs.offer(message) {
			r.discardLogFrame(message)
		}
		return nil
	default:
		if r.control.offer(message) {
			return nil
		}
	}
	// Never acknowledge an unqueued durable frame. Closing causes the agent's
	// journal/command reconciliation to replay it after an authenticated reconnect.
	return ErrUnavailable
}

func (r *peerReceivers) discardLogFrame(message wire.Message) {
	var report struct {
		Entries []json.RawMessage `json:"entries"`
		Dropped uint64            `json:"dropped"`
	}
	if wire.Decode(message.Data, &report) != nil || len(report.Entries) > 500 {
		return
	}
	r.master.mu.Lock()
	defer r.master.mu.Unlock()
	if r.master.peers[r.peer.nodeID] != r.peer {
		return
	}
	snapshot := r.master.live[r.peer.nodeID]
	count := min(report.Dropped, ^uint64(0)-uint64(len(report.Entries))) + uint64(len(report.Entries))
	snapshot.logsDropped += min(count, ^uint64(0)-snapshot.logsDropped)
	r.master.live[r.peer.nodeID] = snapshot
}

func (m *Master) rejectPeerFrame(peer *agentPeer, message wire.Message, err error) {
	if m.logger == nil {
		return
	}
	fields := []any{"node_id", peer.nodeID, "type", message.Type, "error_code", safeFrameErrorCode(err), "cause", frameErrorCause(err)}
	m.logger.Warn("agent frame rejected", append(fields, frameErrorDetails(err)...)...)
}
