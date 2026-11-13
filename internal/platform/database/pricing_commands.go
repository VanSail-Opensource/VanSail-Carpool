package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingModelRevisionKey struct{}

func pricingCommandEffectTx(ctx context.Context, tx *sql.Tx, in PricingIntervention, actor string, now time.Time) error {
	if in.Command == "ACK_INCIDENT" {
		if in.ScopeType != "incident" {
			return ErrConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE pricing_operational_incidents SET acknowledged_by=$1,acknowledged_at=$2 WHERE id=$3`, actor, pricingStamp(now), in.ScopeID)
		if err != nil {
			return err
		}
		if n, e := result.RowsAffected(); e != nil {
			return e
		} else if n != 1 {
			return ErrNotFound
		}
	}
	if in.Command == "RETRY_MODEL" || in.Command == "RECALIBRATE" {
		if in.ScopeType != "product" && in.ScopeType != "global" && in.ScopeType != "model" {
			return ErrConflict
		}
		raw, err := json.Marshal(map[string]string{"interventionId": in.ID})
		if err != nil {
			return err
		}
		return insertOutboxTx(ctx, tx, "pricing_model_action", string(raw), now, now)
	}
	return nil
}

// ExecutePricingModelAction runs from the existing durable outbox, sharing the
// scheduled intelligence compute slot without touching authoritative billing.
func (s *Store) ExecutePricingModelAction(ctx context.Context, job model.OutboxJob, now time.Time) error {
	err := s.executePricingModelAction(ctx, job, now, s.PricingOptimizeProduct)
	if errors.Is(err, ErrPricingComputePending) || errors.Is(err, pricing.WorkerResourceLimit) || errors.Is(err, context.DeadlineExceeded) {
		return &outbox.WaitError{Reason: "pricing model action is awaiting bounded compute", Cause: err, After: time.Second}
	}
	return err
}

func (s *Store) executePricingModelAction(ctx context.Context, job model.OutboxJob, now time.Time,
	optimize func(context.Context, string, string, time.Time) error) error {
	payload, err := s.pricingActionProgress(ctx, job)
	if err != nil {
		return err
	}
	var raw, status string
	if err := s.db.QueryRowContext(ctx, `SELECT proposed_json,status FROM pricing_admin_overrides WHERE id=$1`, payload.InterventionID).Scan(&raw, &status); err != nil {
		return err
	}
	if status != "ACTIVE" {
		return nil
	}
	var in PricingIntervention
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return err
	}
	if in.ID != payload.InterventionID || (in.Command != "RETRY_MODEL" && in.Command != "RECALIBRATE") {
		return ErrConflict
	}
	if !in.ExpiresAt.After(now) {
		return nil
	}
	if in.Command == "RECALIBRATE" {
		ctx = context.WithValue(ctx, pricingModelRevisionKey{}, in.ID)
	}
	if in.ScopeType == "product" {
		parts := strings.SplitN(in.ScopeID, ":", 2)
		if len(parts) != 2 {
			return ErrConflict
		}
		return optimize(ctx, parts[0], parts[1], now)
	}
	if in.ScopeType == "model" {
		var kind string
		if err := s.db.QueryRowContext(ctx, `SELECT kind FROM pricing_model_versions WHERE id=$1`, in.ScopeID).Scan(&kind); err != nil {
			return err
		}
		parts := strings.SplitN(kind, ":", 3)
		if len(parts) == 3 && parts[0] == "response" {
			return optimize(ctx, parts[1], parts[2], now)
		}
	}
	if in.ScopeType != "global" && in.ScopeType != "model" {
		return ErrConflict
	}
	return s.continuePricingModelAction(ctx, job, payload, now, optimize)
}

func (s *Store) acquirePricingCompute() (func(), error) {
	if s.pricingHealth.ApplicationPressured(time.Now()) {
		return nil, pricing.WorkerResourceLimit
	}
	select {
	case s.pricingComputeSlot <- struct{}{}:
		return func() { <-s.pricingComputeSlot }, nil
	default:
		return nil, pricing.WorkerResourceLimit
	}
}
