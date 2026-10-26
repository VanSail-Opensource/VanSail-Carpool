package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingModelRevisionKey struct{}

func pricingCommandEffectTx(ctx context.Context, tx *sql.Tx, in PricingIntervention, actor string, now time.Time) error {
	if in.Command == "ACK_INCIDENT" {
		if in.ScopeType != "incident" {
			return ErrConflict
		}
		result, err := tx.ExecContext(ctx, `UPDATE pricing_operational_incidents SET acknowledged_by=?,acknowledged_at=? WHERE id=?`, actor, pricingStamp(now), in.ScopeID)
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
func (s *Store) ExecutePricingModelAction(ctx context.Context, id string, now time.Time) error {
	var raw, status string
	if err := s.db.QueryRowContext(ctx, `SELECT proposed_json,status FROM pricing_admin_overrides WHERE id=?`, id).Scan(&raw, &status); err != nil {
		return err
	}
	if status != "ACTIVE" {
		return nil
	}
	var in PricingIntervention
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return err
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
		return s.PricingOptimizeProduct(ctx, parts[0], parts[1], now)
	}
	if in.ScopeType == "model" {
		var kind string
		if err := s.db.QueryRowContext(ctx, `SELECT kind FROM pricing_model_versions WHERE id=?`, in.ScopeID).Scan(&kind); err != nil {
			return err
		}
		parts := strings.SplitN(kind, ":", 3)
		if len(parts) == 3 && parts[0] == "response" {
			return s.PricingOptimizeProduct(ctx, parts[1], parts[2], now)
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT product_type,product_id FROM pricing_product_settings WHERE dynamic_enabled=1 ORDER BY product_type,product_id LIMIT 512`)
	if err != nil {
		return err
	}
	type target struct{ kind, id string }
	targets := []target{}
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.kind, &t.id); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, t := range targets {
		if t.kind == "combo" {
			if err = s.TrainPricingRenewalResponse(ctx, t.id, now); err != nil {
				return err
			}
		}
		if err = s.PricingOptimizeProduct(ctx, t.kind, t.id, now); err != nil {
			return err
		}
	}
	return nil
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
