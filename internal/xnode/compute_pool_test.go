package xnode

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func computeMasterFixture(t *testing.T) *Master {
	t.Helper()
	m := NewMaster(&Service{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx, m.cancel = ctx, cancel
	m.wg.Add(1)
	go m.compute.run(ctx)
	t.Cleanup(func() { cancel(); m.wg.Wait() })
	return m
}

func TestComputeReassignsDisconnectedWinnerAndCommitsOnce(t *testing.T) {
	m := computeMasterFixture(t)
	m.SetComputePressure(true)
	first := computeNodeFixture(t, m, "first", 0, true)
	second := computeNodeFixture(t, m, "second", 25*time.Millisecond, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var accepted atomic.Int32
	err := m.Compute(ctx, compute.Task{ID: "anonymous-series", Kind: "forecast_quantiles_v1", Samples: []float64{0, 10, 20}}, func(r compute.Result) error {
		accepted.Add(1)
		if r.Quantiles == nil || r.Quantiles.P50 != 10 {
			t.Error("wrong reassigned result", r)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Load() != 1 || second.Load() != 1 || accepted.Load() != 1 {
		t.Fatal("reassignment or publication count", first.Load(), second.Load(), accepted.Load())
	}
}

func TestComputeKeepsNodeReservedUntilDurableAcceptance(t *testing.T) {
	m := computeMasterFixture(t)
	m.SetComputePressure(true)
	granted := computeNodeFixture(t, m, "reserved", 0, false)
	accepting, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- m.Compute(ctx, compute.Task{ID: "first", Kind: "forecast_quantiles_v1", Samples: []float64{1}}, func(compute.Result) error { close(accepting); <-release; return nil })
	}()
	select {
	case <-accepting:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	m.compute.mu.Lock()
	busy := len(m.compute.busy)
	m.compute.mu.Unlock()
	if busy != 1 || granted.Load() != 1 {
		t.Error("result acceptance released reservation early", busy, granted.Load())
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestComputeLocalBatchPreservesEveryTask(t *testing.T) {
	m := computeMasterFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 8)
	for i := range 8 {
		go func(i int) {
			done <- m.Compute(ctx, compute.Task{ID: string(rune('a' + i)), Kind: "forecast_quantiles_v1", Samples: []float64{float64(i)}}, func(r compute.Result) error {
				if r.Quantiles == nil || r.Quantiles.Mean != float64(i) {
					t.Error("batched task result mismatch", i, r)
				}
				return nil
			})
		}(i)
	}
	for range 8 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
