package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

// PreparePricingRenewal never holds a write transaction while numerical work is
// queued. Fixed prices and refused adjustments produce terminal unchanged quotes.
func (s *Store) PreparePricingRenewal(ctx context.Context, purchase string, now time.Time) (renewal.Quote, error) {
	ctx = pricingFreshContext(ctx)
	var product string
	if err := s.db.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE id=?`, purchase).Scan(&product); err != nil {
		return renewal.Quote{}, err
	}
	policy, err := s.PricingPolicy(ctx, "combo", product)
	if err != nil {
		return renewal.Quote{}, err
	}
	if policy.DynamicEnabled && policy.RenewalMode == "DYNAMIC" {
		if err = s.TrainPricingRenewalResponse(ctx, product, now); errors.Is(err, ErrPricingComputePending) {
			return renewal.Quote{}, ErrPricingDecisionPending
		} else if err != nil {
			return renewal.Quote{}, err
		}
	}
	in, existing, err := s.prepareRenewalPriceSnapshot(ctx, purchase, now)
	if err != nil {
		return renewal.Quote{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	best, curve, err := s.calculateRenewalPrice(ctx, in)
	if err != nil {
		return renewal.Quote{}, err
	}
	return s.publishRenewalPrice(ctx, in, best, curve, now)
}

func (s *Store) retryAttempt_prepareRenewalPriceSnapshot(ctx context.Context, purchase string, now time.Time) (renewalPriceSnapshot, *renewal.Quote, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return renewalPriceSnapshot{}, nil, err
	}
	defer tx.Rollback()
	c, err := ensurePricingContractTx(ctx, tx, purchase, now)
	if err != nil {
		return renewalPriceSnapshot{}, nil, err
	}
	in, err := s.renewalPriceSnapshotTx(ctx, tx, c, now)
	if err != nil {
		return in, nil, err
	}
	q, err := existingRenewalSnapshotTx(ctx, tx, in, now)
	if err != nil {
		return in, nil, err
	}
	return in, q, tx.Commit()
}

func existingRenewalSnapshotTx(ctx context.Context, tx *sql.Tx, in renewalPriceSnapshot, now time.Time) (*renewal.Quote, error) {
	q, err := scanPricingQuote(tx.QueryRowContext(ctx, pricingQuoteSelect+` WHERE contract_id=? AND state NOT IN ('CANCELLED','EXPIRED') AND valid_until>? ORDER BY generated_at DESC LIMIT 1`, in.Contract.ID, pricingStamp(now)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var trace struct {
		Hash string `json:"snapshotHash"`
	}
	if json.Unmarshal([]byte(q.Trace), &trace) == nil && trace.Hash == in.Hash && q.PolicyVersion == in.Policy.Version {
		return &q, nil
	}
	return nil, nil
}

func (s *Store) retryAttempt_publishRenewalPrice(ctx context.Context, in renewalPriceSnapshot, best renewal.Candidate, curve []renewal.Candidate, now time.Time) (renewal.Quote, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return renewal.Quote{}, err
	}
	defer tx.Rollback()
	c, err := pricingContractTx(ctx, tx, in.Contract.PurchaseID)
	if err != nil {
		return renewal.Quote{}, err
	}
	current, err := s.renewalPriceSnapshotTx(ctx, tx, c, now)
	if err != nil {
		return renewal.Quote{}, err
	}
	if current.Hash != in.Hash {
		return renewal.Quote{}, pricing.VersionConflict
	}
	if q, e := existingRenewalSnapshotTx(ctx, tx, in, now); e != nil {
		return renewal.Quote{}, e
	} else if q != nil {
		return *q, nil
	}
	if err = invalidatePricingRenewalBasketTx(ctx, tx, c.PurchaseID); err != nil {
		return renewal.Quote{}, err
	}
	id, err := ids.New()
	if err != nil {
		return renewal.Quote{}, err
	}
	if in.Coupon == nil {
		var addonBasis int64
		for _, a := range c.Addons {
			addonBasis += a.Price
		}
		core := best.Price - addonBasis
		personal, e := personalPricingTx(ctx, tx, c.UserID, "combo", c.ProductID, core, 1, "", false, now)
		if e != nil {
			return renewal.Quote{}, e
		}
		if personal < core {
			personal, e = personalPricingTx(ctx, tx, c.UserID, "combo", c.ProductID, core, 1, "renewal:"+id, true, now)
			if e != nil {
				return renewal.Quote{}, e
			}
			best.Price = personal + addonBasis
			best.Contribution = best.Probability * (float64(best.Price) - in.Cost)
		}
	}
	calibration := "PRIOR_ONLY"
	if in.Response.Samples >= 20 {
		calibration = "UNVALIDATED"
	}
	if in.Validated {
		calibration = "VALIDATED"
	}
	reason := "MODEL_UNVALIDATED"
	if c.Mode == "FIXED" {
		reason = "FIXED_PRICE"
	}
	if in.Validated {
		reason = "NET_CONTRIBUTION"
	}
	status := "UNCHANGED"
	if best.Price != c.Price {
		status = "ADJUSTED"
	}
	raw, err := json.Marshal(map[string]any{"strategyVersion": pricing.StrategyVersion, "snapshotHash": in.Hash, "status": status, "reason": reason, "calibrationStatus": calibration, "candidates": curve, "coupon": in.Coupon, "observation": in.Observation, "expectedProviderCostMinor": in.Cost})
	if err != nil {
		return renewal.Quote{}, err
	}
	q := renewal.Quote{ID: id, PurchaseID: c.PurchaseID, ContractID: c.ID, Price: best.Price, PolicyVersion: in.Policy.Version, ContractVersion: c.Version, State: "AUTHORIZED", AuthorizedAt: &now, GeneratedAt: now, SettleAfter: in.Due, ValidUntil: in.Due.Add(72 * time.Hour), Probability: best.Probability, Contribution: best.Contribution, HealthState: in.Health, ModelVersion: in.Model, Trace: string(raw)}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_renewal_quotes(id,purchase_id,contract_id,price_minor,policy_version,contract_version,state,generated_at,authorized_at,settle_after,valid_until,probability,contribution,health_state,model_version,trace_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, q.PurchaseID, c.ID, q.Price, q.PolicyVersion, c.Version, q.State, pricingStamp(now), pricingStamp(now), pricingStamp(in.Due), pricingStamp(q.ValidUntil), q.Probability, q.Contribution, q.HealthState, q.ModelVersion, q.Trace)
	if err != nil {
		return q, err
	}
	return q, tx.Commit()
}
