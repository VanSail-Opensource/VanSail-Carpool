package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func TestPricingComputeUsesIndependentClaimsAndDurableIdempotentReceipts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	input := compute.Task{Kind: "forecast_quantiles_v1", Samples: []float64{10, 20, 30}}
	for range 2 {
		if _, err := s.PricingCalculate(ctx, input); !errors.Is(err, ErrPricingComputePending) {
			t.Fatal(err)
		}
	}
	if err := s.EnqueueOutbox(ctx, "independent_fixture", `{"target":"integration"}`, now); err != nil {
		t.Fatal(err)
	}
	normal, err := s.ClaimOutboxJob(ctx, now.Add(time.Second))
	if err != nil || normal == nil || normal.Kind == PricingComputeJob {
		t.Fatal("normal lane consumed compute work", normal, err)
	}
	job, err := s.ClaimComputeOutboxJob(ctx, now.Add(time.Second))
	if err != nil || job == nil || job.Kind != PricingComputeJob {
		t.Fatal("compute lane unavailable beside integration", job, err)
	}
	if duplicate, e := s.ClaimComputeOutboxJob(ctx, now.Add(time.Second)); e != nil || duplicate != nil {
		t.Fatal("input was queued twice", duplicate, e)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	// Re-read through the durable interface, with no pool or transport state.
	recovered, err := s.PricingComputeInput(ctx, payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	batch := compute.Batch{Version: compute.Version, Tasks: []compute.Task{recovered}}
	out, err := compute.Run(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	bad := out.Items[0]
	bad.ID = "stale-attempt"
	if err = s.AcceptPricingCompute(ctx, recovered, bad, now); err == nil {
		t.Fatal("unbound result accepted")
	}
	for range 2 {
		if err = s.AcceptPricingCompute(ctx, recovered, out.Items[0], now); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.PricingCalculate(ctx, input)
	if err != nil || result.Quantiles == nil || result.Quantiles.P50 != 20 {
		t.Fatal("accepted receipt not reused", result, err)
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_features WHERE entity_type='compute_result' AND entity_id=?`, recovered.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate result published", count, err)
	}
}
