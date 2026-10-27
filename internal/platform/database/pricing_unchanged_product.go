package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// A refusal is terminal evidence, unlike an unfinished numerical calculation.
func (s *Store) retryAttempt_recordUnchangedProduct(ctx context.Context, kind, product, model, reason string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	fixed, err := pricingReference(ctx, tx, kind, product)
	if err != nil {
		return err
	}
	p, err := loadPricingPolicy(ctx, tx, kind, product, fixed)
	if err != nil {
		return err
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_decisions WHERE product_type=? AND product_id=? AND policy_version=? AND generated_at>=?)`, kind, product, p.Version, pricingStamp(now.UTC().Truncate(time.Minute))).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return nil
	}
	selected, e := approvedPricingTx(ctx, tx, p, fixed, now)
	if e != nil {
		selected = fixed
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"strategyVersion": pricing.StrategyVersion, "status": "UNCHANGED", "reason": reason, "calculationComplete": true})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_decisions(id,product_type,product_id,generated_at,policy_version,model_version,reference_minor,proposed_minor,selected_minor,state,trace_json) VALUES(?,?,?,?,?,?,?,?,?,'SHADOW',?)`, id, kind, product, pricingStamp(now), p.Version, model, fixed, selected, selected, string(raw))
	if err != nil {
		return err
	}
	return tx.Commit()
}
