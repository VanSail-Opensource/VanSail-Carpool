package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func TestPricingResponseSupportUsesOnlyWireBudgetTrainingRows(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	now := time.Now().UTC()
	// Long decimal features force the real 256 KiB compute budget to trim the
	// oldest rows. Only those discarded rows include the negative price range.
	_, err := s.db.ExecContext(ctx, `INSERT INTO pricing_offers
 (id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,model_version,predicted_probability,features_json)
 SELECT 'sample-'||i,'member-'||i,'combo','support-product',100,100,
 to_char(($1::timestamptz+i*interval '1 second') AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US')||'000Z',
 $2,i%2,$2,'prior',0.5,jsonb_build_object('strategyVersion',$3::text,'complete',1,
 'logPrice',CASE WHEN i<=256 THEN -1.1234567890123456 ELSE 0.1234567890123456+i::float8/10000000 END,
 'activity',0.1234567890123456,'renewals',0.9876543210987654,'couponPresent',1,'couponDiscount',0.4567890123456789)::text
 FROM generate_series(1,2048) i`, pricingStamp(now.Add(-48*time.Hour)), pricingStamp(now.Add(-24*time.Hour)), pricing.StrategyVersion)
	if err != nil {
		t.Fatal(err)
	}
	_, modelID, err := s.TrainPricingResponse(ctx, "combo", "support-product", now)
	if !errors.Is(err, ErrPricingComputePending) || modelID == "" {
		t.Fatal("training did not enqueue bounded work", modelID, err)
	}
	job, err := s.ClaimComputeOutboxJob(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := s.PricingComputeInput(ctx, payload.ID)
	if err != nil || len(task.Observations) == 0 || len(task.Observations) >= 2048 || task.Observations[0].LogPrice <= 0 {
		t.Fatal("fixture did not trim the unsupported historical range", len(task.Observations), err)
	}
	output, err := compute.Run(ctx, compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptPricingCompute(ctx, task, output.Items[0], now); err != nil {
		t.Fatal(err)
	}
	model, id, err := s.TrainPricingResponse(ctx, "combo", "support-product", now)
	if err != nil || id != modelID || model.Samples != len(task.Observations) {
		t.Fatal("published model lost compute lineage", model, id, err)
	}
	support, err := pricingModelSupportTx(ctx, s.db, id)
	if err != nil || support.Samples != model.Samples || support.Low <= 0 || support.CouponLow <= 0 || support.CouponSamples != model.Samples*4/5 {
		t.Fatal("discarded observations widened fitted price support", support, model, err)
	}
	if support.High >= task.Observations[len(task.Observations)-1].LogPrice {
		t.Fatal("future holdout observations widened training support", support)
	}
}
