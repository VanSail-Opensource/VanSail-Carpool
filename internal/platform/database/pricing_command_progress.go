package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type pricingActionProgress struct {
	InterventionID string `json:"interventionId"`
	AfterType      string `json:"afterType,omitempty"`
	AfterID        string `json:"afterId,omitempty"`
}

// The existing job owns continuation; no parallel queue or unbounded inventory
// is needed. Every checkpoint compares the exact lease and previous payload.
func (s *Store) pricingActionProgress(ctx context.Context, job model.OutboxJob) (pricingActionProgress, error) {
	var payload pricingActionProgress
	if job.Kind != "pricing_model_action" || job.ID == "" || json.Unmarshal([]byte(job.Payload), &payload) != nil ||
		payload.InterventionID == "" || (payload.AfterType == "") != (payload.AfterID == "") {
		return payload, ErrConflict
	}
	var current bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE id=$1 AND kind='pricing_model_action'
 AND status='processing' AND attempts=$2 AND claim_generation=$3 AND payload=$4)`,
		job.ID, job.Attempts, job.ClaimGeneration, job.Payload).Scan(&current)
	if err == nil && !current {
		err = ErrConflict
	}
	return payload, err
}

func (s *Store) continuePricingModelAction(ctx context.Context, job model.OutboxJob, progress pricingActionProgress, now time.Time,
	optimize func(context.Context, string, string, time.Time) error) error {
	const batchSize = 4
	rows, err := s.db.QueryContext(ctx, `SELECT product_type,product_id FROM pricing_product_settings
 WHERE dynamic_enabled=1 AND (product_type,product_id)>($1,$2) ORDER BY product_type,product_id LIMIT $3`,
		progress.AfterType, progress.AfterID, batchSize)
	if err != nil {
		return err
	}
	type target struct{ kind, id string }
	targets := []target{}
	for rows.Next() {
		var item target
		if err = rows.Scan(&item.kind, &item.id); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range targets {
		if _, err := s.pricingActionProgress(ctx, job); err != nil {
			return err
		}
		if err = optimize(ctx, item.kind, item.id, now); err != nil {
			return err
		}
		progress.AfterType, progress.AfterID = item.kind, item.id
		raw, err := json.Marshal(progress)
		if err != nil {
			return err
		}
		result, err := s.db.ExecContext(ctx, `UPDATE outbox_jobs SET payload=$1,updated_at=$2 WHERE id=$3
 AND kind='pricing_model_action' AND status='processing' AND attempts=$4 AND claim_generation=$5 AND payload=$6`,
			string(raw), stamp(now), job.ID, job.Attempts, job.ClaimGeneration, job.Payload)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return ErrConflict
		}
		job.Payload = string(raw)
	}
	if len(targets) == batchSize {
		return &outbox.WaitError{Reason: "pricing model action has remaining products", After: time.Second}
	}
	return nil
}
