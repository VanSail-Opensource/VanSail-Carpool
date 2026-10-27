package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

var errPricingPublicationRefused = errors.New("publication operationally refused")

// PricingOptimizeProduct gates publication on evidence, capacity, health, costs
// and the current policy version. Every candidate remains exportable in its trace.
func (s *Store) PricingOptimizeProduct(ctx context.Context, kind, product string, now time.Time) error {
	ctx = pricingFreshContext(ctx)
	release, slotErr := s.acquirePricingCompute()
	if slotErr != nil {
		return slotErr
	}
	defer release()
	m, model, err := s.TrainPricingResponse(ctx, kind, product, now)
	if err != nil {
		return err
	}
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	in, err := s.pricingOptimizationSnapshot(ctx, kind, product, m, now)
	if err != nil {
		var code pricing.Code
		if errors.As(err, &code) {
			return s.recordUnchangedProduct(ctx, kind, product, model, string(code), now)
		}
		return err
	}
	if !in.Policy.DynamicEnabled {
		return nil
	}
	in.Support, err = pricingModelSupportTx(ctx, s.db, model)
	if err != nil {
		return err
	}
	if !m.Validated || in.Support.Samples < 200 {
		return s.recordUnchangedProduct(ctx, kind, product, model, "MODEL_UNVALIDATED", now)
	}
	result, err := s.PricingCalculate(ctx, compute.Task{Kind: "candidate_eval_v1", Model: m, Candidates: pricing.NumericalCandidates(in)})
	if err != nil {
		return err
	}
	best, candidates, optErr := pricing.SelectCandidates(in, result.Candidates)
	err = s.publishPricingOptimization(ctx, kind, product, model, in, best, candidates, optErr, now)
	if errors.Is(err, errPricingPublicationRefused) {
		return s.recordUnchangedProduct(ctx, kind, product, model, "OPERATIONAL_REFUSAL", now)
	}
	return err
}

func (s *Store) retryAttempt_publishPricingOptimization(ctx context.Context, kind, product, model string, in pricing.OptimizationInputs, best pricing.PriceCandidate, candidates []pricing.PriceCandidate, optErr error, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var serial int
	if err = tx.QueryRowContext(ctx, `SELECT singleton FROM pricing_worker_watermark WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return err
	}
	fixed, err := pricingReference(ctx, tx, kind, product)
	if err != nil {
		return err
	}
	p, err := loadPricingPolicy(ctx, tx, kind, product, fixed)
	if err != nil {
		return err
	}
	if !p.DynamicEnabled {
		return nil
	}
	if blocked, err := pricingPublicationFrozenTx(ctx, tx, p, now); err != nil {
		return err
	} else if blocked {
		return errPricingPublicationRefused
	}
	if err = pricingOperatorAdmissionTx(ctx, tx, kind, product, now); err != nil {
		var code pricing.Code
		if errors.As(err, &code) {
			return errPricingPublicationRefused
		}
		return err
	}
	nodes, err := pricingNodesTx(ctx, tx, kind, product)
	if err != nil {
		return err
	}
	if !s.pricingHealth.TelemetrySafe(nodes, now) {
		return errPricingPublicationRefused
	}
	if p.Version != in.Policy.Version {
		return pricing.VersionConflict
	}
	// A frozen or superseded evidence model cannot authorize publication.
	support, err := pricingModelSupportTx(ctx, tx, model)
	if err != nil {
		return err
	}
	if support.Samples < 200 {
		return errPricingPublicationRefused
	}
	current, err := pricingOptimizationInputsTx(ctx, tx, p, in.Model, now)
	if err != nil {
		return err
	}
	// New accounting, costs or capacity invalidate an obsolete numerical snapshot.
	if current.DemandGB != in.DemandGB || current.SalesUnits != in.SalesUnits || current.CostMinor != in.CostMinor || current.CapacityGB != in.CapacityGB || current.CommittedGB != in.CommittedGB || current.ForecastValidated != in.ForecastValidated || current.CapacityUnbounded != in.CapacityUnbounded || current.CostEvidence != in.CostEvidence {
		return pricing.VersionConflict
	}
	p.MaxIncreaseBPS, p.MaxDiscountBPS = pricing.CandidateLimits(fixed, best, candidates)
	id, err := ids.New()
	if err != nil {
		return err
	}
	reason := "OPTIMUM_SELECTED"
	if optErr != nil {
		reason = optErr.Error()
	}
	trace, err := json.Marshal(map[string]any{"strategyVersion": pricing.StrategyVersion, "reason": reason, "calculationComplete": true, "candidates": candidates, "demandGB": in.DemandGB, "dailyCostMinor": in.CostMinor, "capacityGB": in.CapacityGB, "capacityUnbounded": in.CapacityUnbounded, "costEvidence": in.CostEvidence, "committedGB": in.CommittedGB, "salesUnits": in.SalesUnits, "modelValidated": in.Model.Validated, "forecastValidated": in.ForecastValidated, "automaticMaxIncreaseBps": p.MaxIncreaseBPS, "automaticMaxDiscountBps": p.MaxDiscountBPS, "experimentationEnabled": false})
	if err != nil {
		return err
	}
	state := "SHADOW"
	selected := in.Current
	if optErr == nil {
		state = "PUBLISHED"
		selected = best.Price
	}
	changed := optErr == nil && best.Price != in.Current
	decisionVersion := p.Version
	if changed {
		decisionVersion++
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_decisions(id,product_type,product_id,generated_at,policy_version,model_version,reference_minor,proposed_minor,selected_minor,state,trace_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, kind, product, pricingStamp(now), decisionVersion, model, fixed, max(0, best.Price), selected, state, string(trace)); err != nil {
		return err
	}
	if optErr == nil {
		if _, err = tx.ExecContext(ctx, `INSERT INTO pricing_published_prices(id,product_type,product_id,price_minor,policy_version,model_version,source,valid_from,valid_until,created_by,created_at,reason) VALUES(?,?,?,?,?,?,'MODEL',?,?,?,?,?)`, id, kind, product, selected, decisionVersion, model, pricingStamp(now), pricingStamp(now.Add(24*time.Hour)), "pricing-worker", pricingStamp(now), "validated constrained optimization"); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE pricing_product_settings SET version=?,last_approved_price_version=?,max_increase_bps=?,max_discount_bps=?,updated_at=? WHERE product_type=? AND product_id=? AND version=?`, decisionVersion, id, p.MaxIncreaseBPS, p.MaxDiscountBPS, pricingStamp(now), kind, product, p.Version)
		if err != nil {
			return err
		}
		if n, e := result.RowsAffected(); e != nil {
			return e
		} else if n != 1 {
			return pricing.VersionConflict
		}
	}
	return tx.Commit()
}
