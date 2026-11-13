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

func TestPricingResponseReturnsConcurrentImmutableWinner(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	features, err := json.Marshal(map[string]any{"strategyVersion": pricing.StrategyVersion, "complete": 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO pricing_offers
		(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,
		model_version,predicted_probability,features_json)
		VALUES('sample','member','combo','product',100,100,$1,$2,1,$2,'prior',0.5,$3)`,
		pricingStamp(now.Add(-24*time.Hour)), pricingStamp(now.Add(-23*time.Hour)), string(features))
	if err != nil {
		t.Fatal(err)
	}
	_, modelID, err := store.TrainPricingResponse(ctx, "combo", "product", now)
	if !errors.Is(err, ErrPricingComputePending) || modelID == "" {
		t.Fatalf("training preparation = %q, %v", modelID, err)
	}
	job, err := store.ClaimComputeOutboxJob(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || job == nil {
		t.Fatalf("training compute job = %+v, %v", job, err)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := store.PricingComputeInput(ctx, payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	output, err := compute.Run(ctx, compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptPricingCompute(ctx, task, output.Items[0], time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// The winner remains uncommitted until the other caller has fitted its own
	// different model and reached the unique-key conflict during publication.
	winner := pricing.ResponseModel{Coefficients: [6]float64{-0.5, -1}, Samples: 99, Holdout: 19}
	raw, err := json.Marshal(winner)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_model_versions
		(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json)
		VALUES($1,'response:combo:product',$2,'UNVALIDATED',$3,'{}',$2,'{}')`, modelID, pricingStamp(now), string(raw))
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		model pricing.ResponseModel
		id    string
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		model, id, err := store.TrainPricingResponse(ctx, "combo", "product", now)
		finished <- result{model: model, id: id, err: err}
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'INSERT INTO pricing_model_versions(')>0)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case got := <-finished:
			t.Fatalf("publication did not encounter concurrent winner: %+v", got)
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("publication did not reach the model conflict")
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got := <-finished
	if got.err != nil || got.id != modelID || got.model != winner {
		t.Fatalf("returned model = %+v; want stored winner %+v with ID %q", got, winner, modelID)
	}
	model, id, err := store.TrainPricingResponse(ctx, "combo", "product", now)
	if err != nil || id != got.id || model != got.model {
		t.Fatalf("replayed model differs from concurrent publication: %+v, %q, %v", model, id, err)
	}
}
