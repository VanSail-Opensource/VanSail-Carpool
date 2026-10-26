package xnode

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type computeCall struct {
	ctx      context.Context
	task     compute.Task
	reply    chan computeReply
	accepted chan bool
}
type computeReply struct {
	result compute.Result
	err    error
}
type computeAttempt struct {
	offer          wire.ComputeAnnouncement
	input          []byte
	winner         *agentPeer
	claimed        chan *agentPeer
	chunks         chan wire.ComputeChunk
	lease          time.Time
	uploadComplete bool // mu; duplicates wait for durable acceptance without buffering
}
type computeReceipt struct {
	peer  *agentPeer
	until time.Time
}
type computePool struct {
	master   *Master
	pressure atomic.Bool
	incoming chan *computeCall
	mu       sync.Mutex
	attempts map[string]*computeAttempt
	busy     map[*agentPeer]string
	accepted map[wire.ComputeSignal]computeReceipt
}

func newComputePool(m *Master) *computePool {
	return &computePool{master: m, incoming: make(chan *computeCall, 128), attempts: map[string]*computeAttempt{}, busy: map[*agentPeer]string{}, accepted: map[wire.ComputeSignal]computeReceipt{}}
}

// SetComputePressure changes only the process-local CPU admission decision.
func (m *Master) SetComputePressure(pressured bool) { m.compute.pressure.Store(pressured) }

// Compute queues one numerical snapshot. Acceptance must durably record its
// result before returning; remote acknowledgements follow all accepted items.
func (m *Master) Compute(ctx context.Context, task compute.Task, accept func(compute.Result) error) error {
	if accept == nil {
		return ErrInvalid
	}
	if err := compute.Validate(compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}}); err != nil {
		return err
	}
	m.mu.RLock()
	lifetime := m.ctx
	m.mu.RUnlock()
	if lifetime == nil {
		return ErrUnavailable
	}
	call := &computeCall{ctx: ctx, task: task, reply: make(chan computeReply, 1), accepted: make(chan bool, 1)}
	select {
	case m.compute.incoming <- call:
	case <-ctx.Done():
		return ctx.Err()
	case <-lifetime.Done():
		return ErrUnavailable
	}
	select {
	case reply := <-call.reply:
		if reply.err != nil {
			return reply.err
		}
		err := accept(reply.result)
		call.accepted <- err == nil
		return err
	case <-ctx.Done():
		call.accepted <- false
		return ctx.Err()
	case <-lifetime.Done():
		call.accepted <- false
		return ErrUnavailable
	}
}

func (p *computePool) fail(calls []*computeCall, err error) {
	if err == nil {
		err = errors.New("calculation unavailable")
	}
	for _, call := range calls {
		call.reply <- computeReply{err: err}
	}
}
