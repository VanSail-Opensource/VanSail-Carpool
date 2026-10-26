package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

var ErrPricingComputePending = errors.New("pricing calculation is pending")

const PricingComputeJob = "pricing_compute"

// PricingCalculate records one immutable numerical input and queues its reference
// in the existing outbox. It never waits for a node or holds a SQL transaction
// while calculating. Exact input hashes coalesce repeated requests.
func (s *Store) PricingCalculate(ctx context.Context, task compute.Task) (compute.Result, error) {
	task.ID = ""
	raw, err := json.Marshal(task)
	if err != nil {
		return compute.Result{}, err
	}
	task.ID = compute.Digest(append([]byte(compute.Version+":"), raw...))
	b := compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}}
	if err = compute.Validate(b); err != nil {
		return compute.Result{}, err
	}
	var saved string
	err = s.db.QueryRowContext(ctx, `SELECT features_json FROM pricing_features WHERE id=? AND entity_type='compute_result'`, "compute-result:"+task.ID).Scan(&saved)
	if err == nil {
		var out compute.Results
		if json.Unmarshal([]byte(saved), &out) != nil || compute.ValidateResult(b, out) != nil {
			return compute.Result{}, ErrConflict
		}
		return out.Items[0], nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return compute.Result{}, err
	}
	raw, err = json.Marshal(task)
	if err != nil {
		return compute.Result{}, err
	}
	if err = s.enqueuePricingCompute(ctx, task.ID, string(raw), time.Now().UTC()); err != nil {
		return compute.Result{}, err
	}
	return compute.Result{}, ErrPricingComputePending
}

func (s *Store) retryAttempt_enqueuePricingCompute(ctx context.Context, hash, raw string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	input := "compute-input:" + hash
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'compute_input',?,?,?,10000,'NOT_APPLICABLE',?,?) ON CONFLICT DO NOTHING`, input, hash, now.Format(time.DateOnly), pricingStamp(now), raw, compute.Digest([]byte(raw)))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"inputId": input})
	if err != nil {
		return err
	}
	if err = insertOutboxTx(ctx, tx, PricingComputeJob, string(payload), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

// PricingComputeInput returns the original snapshot, not current mutable inputs.
func (s *Store) PricingComputeInput(ctx context.Context, input string) (compute.Task, error) {
	var raw, hash string
	var task compute.Task
	err := s.db.QueryRowContext(ctx, `SELECT features_json,snapshot_hash FROM pricing_features WHERE id=? AND entity_type='compute_input'`, input).Scan(&raw, &hash)
	if err != nil {
		return task, err
	}
	if compute.Digest([]byte(raw)) != hash || json.Unmarshal([]byte(raw), &task) != nil {
		return task, ErrConflict
	}
	return task, compute.Validate(compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
}

func (s *Store) AcceptPricingCompute(ctx context.Context, task compute.Task, item compute.Result, now time.Time) error {
	b := compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}}
	_, hash, err := compute.DigestBatch(b)
	if err != nil {
		return err
	}
	out := compute.Results{Version: compute.Version, InputHash: hash, Items: []compute.Result{item}}
	if err = compute.ValidateResult(b, out); err != nil {
		return err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'compute_result',?,?,?,10000,'NOT_APPLICABLE',?,?) ON CONFLICT DO NOTHING`, "compute-result:"+task.ID, task.ID, now.Format(time.DateOnly), pricingStamp(now), string(raw), compute.Digest(raw))
	return err
}
