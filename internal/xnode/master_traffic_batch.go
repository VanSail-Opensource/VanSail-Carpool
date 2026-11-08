package xnode

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const trafficBatchDelay = 100 * time.Millisecond
const trafficBatchMemory = 32 << 20
const trafficBatchFlushBytes = 16 << 20

type trafficBatchRequest struct {
	item  TrafficCommit
	ctx   context.Context
	done  chan error
	bytes int64
}
type trafficBatcher struct {
	mu      sync.Mutex
	stopped bool
	repo    TrafficBatchRepository
	input   chan trafficBatchRequest
	bytes   atomic.Int64
}

func (m *Master) persistTraffic(ctx context.Context, item TrafficCommit, encodedBytes int) error {
	m.mu.RLock()
	batcher := m.trafficBatcher
	m.mu.RUnlock()
	if batcher == nil {
		return m.service.repo.RecordXNodeTraffic(ctx, item.NodeID, item.AgentID, item.Sequence, item.Digest, item.Report)
	}
	size := int64(encodedBytes + len(item.Report.Samples)*512 + len(item.Report.HourlySamples)*1024 + len(item.Report.Traffic)*64 + 256)
	// Previously persisted large journals keep their isolated inbound execution
	// path. They cannot consume the modern fleet's entire batch memory allowance.
	if item.Report.StreamID == "" && size > trafficBatchFlushBytes {
		return m.service.repo.RecordXNodeTraffic(ctx, item.NodeID, item.AgentID, item.Sequence, item.Digest, item.Report)
	}
	batcher.mu.Lock()
	if batcher.stopped || ctx.Err() != nil {
		batcher.mu.Unlock()
		return ErrUnavailable
	}
	if batcher.bytes.Add(size) > trafficBatchMemory {
		batcher.bytes.Add(-size)
		batcher.mu.Unlock()
		return ErrUnavailable
	}
	request := trafficBatchRequest{item: item, ctx: ctx, done: make(chan error, 1), bytes: size}
	select {
	case batcher.input <- request:
		batcher.mu.Unlock()
	default:
		batcher.bytes.Add(-size)
		batcher.mu.Unlock()
		return ErrUnavailable
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *trafficBatcher) run(ctx context.Context) {
	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.stopped = true
		for {
			select {
			case request := <-b.input:
				b.finish(request, context.Canceled)
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case first := <-b.input:
			requests := []trafficBatchRequest{first}
			bytes := first.bytes
			timer := time.NewTimer(trafficBatchDelay)
		collect:
			for len(requests) < MaxTrafficBatchReports && bytes < trafficBatchFlushBytes {
				select {
				case request := <-b.input:
					requests = append(requests, request)
					bytes += request.bytes
				case <-timer.C:
					break collect
				case <-ctx.Done():
					break collect
				}
			}
			timer.Stop()
			b.flush(ctx, requests)
		}
	}
}

func (b *trafficBatcher) flush(ctx context.Context, requests []trafficBatchRequest) {
	if err := ctx.Err(); err != nil {
		for _, request := range requests {
			b.finish(request, err)
		}
		return
	}
	active := make([]trafficBatchRequest, 0, len(requests))
	items := make([]TrafficCommit, 0, len(requests))
	for _, request := range requests {
		if err := request.ctx.Err(); err != nil {
			b.finish(request, err)
		} else {
			active, items = append(active, request), append(items, request.item)
		}
	}
	if len(items) == 0 {
		return
	}
	child, cancel := context.WithTimeout(ctx, 15*time.Second)
	results, err := b.repo.RecordXNodeTrafficBatch(child, items)
	cancel()
	if err == nil && len(results) != len(active) {
		err = ErrInvalid
	}
	for i, request := range active {
		outcome := err
		if outcome == nil {
			outcome = results[i]
		}
		b.finish(request, outcome)
	}
}

func (b *trafficBatcher) finish(request trafficBatchRequest, err error) {
	request.done <- err
	b.bytes.Add(-request.bytes)
}
