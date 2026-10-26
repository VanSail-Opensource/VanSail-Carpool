package pricing

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnalysisCacheSharesWorkCopiesResultsAndInvalidates(t *testing.T) {
	c := NewAnalysisCache()
	var calls atomic.Int32
	start := make(chan struct{})
	entered := make(chan struct{})
	load := func() ([]byte, error) { calls.Add(1); close(entered); <-start; return []byte("original"), nil }
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := c.Load(context.Background(), "same", time.Hour, load); err != nil {
			t.Error(err)
		}
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Load(ctx, "same", time.Hour, load); err != context.Canceled {
		t.Fatal(err)
	}
	close(start)
	wg.Wait()
	v, err := c.Load(context.Background(), "same", time.Hour, load)
	if err != nil {
		t.Fatal(err)
	}
	v[0] = 'X'
	v, _ = c.Load(context.Background(), "same", time.Hour, load)
	if string(v) != "original" || calls.Load() != 1 {
		t.Fatal("cache aliasing or repeated calculation", string(v), calls.Load())
	}
	c.Clear()
	v, err = c.Load(context.Background(), "same", time.Hour, func() ([]byte, error) { calls.Add(1); return []byte("new"), nil })
	if err != nil || string(v) != "new" || calls.Load() != 2 {
		t.Fatal(v, err, calls.Load())
	}
}

func TestAutomaticLimitsNeedValidatedEvidence(t *testing.T) {
	in := OptimizationInputs{Policy: DefaultPolicy("combo", "p", 1000), Current: 1000, DemandGB: 1, SalesUnits: 10, CostMinor: 10, CapacityGB: 100, ForecastValidated: true, Model: ResponseModel{Validated: true, Coefficients: [6]float64{2, -5, 0, 0}}}
	up, down := OptimalLimits(in)
	if up < 0 || up > Scale || down <= 0 || down > Scale {
		t.Fatal("missing bounded economic optimum", up, down)
	}
	in.Model.Validated = false
	if up, down = OptimalLimits(in); up != 0 || down != 0 {
		t.Fatal("prior invented optimal elasticity", up, down)
	}
	in.Model.Validated = true
	in.CapacityGB = .5
	if up, down = OptimalLimits(in); up != 0 || down != 0 {
		t.Fatal("unsafe capacity became a higher price", up, down)
	}
}
