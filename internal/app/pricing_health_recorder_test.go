package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type pricingHealthPersistenceStub struct {
	calls int
	state string
	err   error
}

func (s *pricingHealthPersistenceStub) RecordPricingHealth(_ context.Context, h pricing.Health) error {
	s.calls++
	s.state = h.State
	return s.err
}

func TestPricingHealthRecordsStartupTransitionsAndPeriodicReconciliation(t *testing.T) {
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	r := &pricingHealthRecorder{}
	store := &pricingHealthPersistenceStub{}
	for _, step := range []struct {
		name  string
		state string
		after time.Duration
		calls int
	}{
		{"startup", "HEALTHY", 0, 1},
		{"unchanged", "HEALTHY", 5 * time.Second, 1},
		{"transition", "DEGRADED", 10 * time.Second, 2},
		{"unchanged incident", "DEGRADED", 15 * time.Second, 2},
		{"periodic reconciliation", "DEGRADED", 70 * time.Second, 3},
		{"recovery", "HEALTHY", 75 * time.Second, 4},
	} {
		t.Run(step.name, func(t *testing.T) {
			err := r.record(context.Background(), store, pricing.Health{NodeID: "node", State: step.state, At: start.Add(step.after)})
			if err != nil || store.calls != step.calls || store.state != step.state {
				t.Fatalf("record = calls %d, state %s, error %v", store.calls, store.state, err)
			}
		})
	}
}

func TestPricingHealthRetriesAfterUncertainCommitEvenWhenStateRecovers(t *testing.T) {
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	r := &pricingHealthRecorder{}
	store := &pricingHealthPersistenceStub{}
	h := pricing.Health{NodeID: "node", State: "HEALTHY", At: start}
	if err := r.record(context.Background(), store, h); err != nil {
		t.Fatal(err)
	}
	store.err = errors.New("uncertain commit")
	h.State, h.At = "OFFLINE", start.Add(5*time.Second)
	if err := r.record(context.Background(), store, h); !errors.Is(err, store.err) {
		t.Fatalf("failed persistence error = %v", err)
	}
	store.err = nil
	h.State, h.At = "HEALTHY", start.Add(10*time.Second)
	if err := r.record(context.Background(), store, h); err != nil || store.calls != 3 || store.state != "HEALTHY" {
		t.Fatalf("recovered incident was not reconciled: calls %d, state %s, error %v", store.calls, store.state, err)
	}
}

func TestPricingHealthRecorderPrunesRemovedNodesAndReconcilesReappearance(t *testing.T) {
	r := &pricingHealthRecorder{}
	store := &pricingHealthPersistenceStub{}
	h := pricing.Health{NodeID: "node", State: "HEALTHY", At: time.Now().UTC()}
	if err := r.record(context.Background(), store, h); err != nil {
		t.Fatal(err)
	}
	r.retain([]xnode.NodeMachineMetrics{{NodeID: "node"}})
	if len(r.states) != 1 {
		t.Fatal("configured node was pruned")
	}
	r.retain(nil)
	if len(r.states) != 0 {
		t.Fatal("removed node retained recorder state")
	}
	h.At = h.At.Add(5 * time.Second)
	if err := r.record(context.Background(), store, h); err != nil || store.calls != 2 {
		t.Fatalf("reappearing node skipped initial reconciliation: calls %d, error %v", store.calls, err)
	}
}
