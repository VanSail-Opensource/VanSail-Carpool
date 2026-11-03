package xnode

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type trafficBatchCall struct {
	items []TrafficCommit
	at    time.Time
}
type trafficBatchFake struct {
	calls   chan trafficBatchCall
	release <-chan struct{}
}

func (r *trafficBatchFake) RecordXNodeTrafficBatch(ctx context.Context, items []TrafficCommit) ([]error, error) {
	r.calls <- trafficBatchCall{items, time.Now()}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return make([]error, len(items)), nil
}

func TestTrafficBatchFlushDeadlineAndCommitBarrier(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		repo := &trafficBatchFake{calls: make(chan trafficBatchCall, 1), release: release}
		batcher := &trafficBatcher{repo: repo, input: make(chan trafficBatchRequest, MaxTrafficBatchReports)}
		master := &Master{trafficBatcher: batcher}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); batcher.run(ctx) }()
		defer func() { cancel(); <-done }()
		results := make(chan error, 2)
		start := time.Now()
		submit := func(sequence uint64) {
			go func() {
				results <- master.persistTraffic(ctx, TrafficCommit{Sequence: sequence, Report: wire.TrafficReport{StreamID: "11111111111111111111111111111111"}}, 100)
			}()
		}
		submit(1)
		synctest.Wait()
		time.Sleep(99 * time.Millisecond)
		if len(repo.calls) != 0 {
			t.Fatal("partial batch flushed before its collection window")
		}
		submit(2)
		synctest.Wait()
		time.Sleep(time.Millisecond)
		call := <-repo.calls
		if len(call.items) != 2 || call.at.Sub(start) != trafficBatchDelay {
			t.Fatal("a later report postponed the deadline", len(call.items), call.at.Sub(start))
		}
		select {
		case <-results:
			t.Fatal("report completed before the shared commit")
		default:
		}
		close(release)
		for range 2 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		if batcher.bytes.Load() != 0 {
			t.Fatal("completed reports retained their memory allowance")
		}
	})
}

func TestFullTrafficBatchFlushesImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repo := &trafficBatchFake{calls: make(chan trafficBatchCall, 1)}
		batcher := &trafficBatcher{repo: repo, input: make(chan trafficBatchRequest, MaxTrafficBatchReports)}
		master := &Master{trafficBatcher: batcher}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); batcher.run(ctx) }()
		defer func() { cancel(); <-done }()
		results := make(chan error, MaxTrafficBatchReports)
		start := time.Now()
		for i := range MaxTrafficBatchReports {
			go func() {
				results <- master.persistTraffic(ctx, TrafficCommit{Sequence: uint64(i + 1), Report: wire.TrafficReport{StreamID: "11111111111111111111111111111111"}}, 100)
			}()
		}
		synctest.Wait()
		call := <-repo.calls
		if len(call.items) != MaxTrafficBatchReports || !call.at.Equal(start) {
			t.Fatal("full batch waited for a timer", len(call.items), call.at.Sub(start))
		}
		for range MaxTrafficBatchReports {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestTrafficQueueBudgetAndShutdownRelease(t *testing.T) {
	master := &Master{}
	receivers := &peerReceivers{master: master, traffic: newInboundLane(8), cancel: func() {}}
	message := wire.New("traffic.report", "", 0, map[string]any{})
	master.trafficInboundBytes.Store(64 << 20)
	if err := receivers.offer(message); err == nil || master.trafficInboundBytes.Load() != 64<<20 {
		t.Fatal("fleet input exceeded its memory bound")
	}
	master.trafficInboundBytes.Store(0)
	for range 2 {
		if err := receivers.offer(message); err != nil {
			t.Fatal(err)
		}
	}
	receivers.stop()
	if master.trafficInboundBytes.Load() != 0 || receivers.traffic.bytes.Load() != 0 {
		t.Fatal("receiver shutdown retained queued memory allowance")
	}
}
