package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

var ErrPricingDecisionPending = errors.New("purchase pricing decision is pending or stale")

func (s *Store) PreparePersonalPricing(ctx context.Context, user, kind, product, combo string, squads []string, now time.Time) (model.PricingDecision, error) {
	if _, ok := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch); !ok {
		var err error
		ctx, err = s.pricingReadContext(ctx, user, now)
		if err != nil {
			return model.PricingDecision{}, err
		}
	}
	full, _ := ctx.Value(pricingBasketKey{}).(bool)
	key, err := json.Marshal([]any{user, kind, product, combo, uniqueSorted(squads), full, now.UTC().Truncate(time.Minute)})
	if err != nil {
		return model.PricingDecision{}, err
	}
	// A zero TTL shares in-flight preparation only. Settlement never uses it.
	return pricingCached(ctx, s, "prepare:"+string(key), 0, func() (model.PricingDecision, error) {
		return s.preparePersonalPricing(ctx, user, kind, product, combo, squads, now, false)
	})
}

func (s *Store) preparePersonalPricing(ctx context.Context, user, kind, product, combo string, squads []string, now time.Time, attempted bool) (model.PricingDecision, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.PricingDecision{}, err
	}
	in, err := personalPriceSnapshotTx(ctx, tx, user, kind, product, combo, squads, now)
	if err == nil && fixedPersonalPricing(in) {
		tx.Rollback()
		return fixedPersonalDecision(in, now), nil
	}
	if err == nil && !in.GlobalPending {
		var found model.PricingDecision
		found, err = personalDecisionTx(ctx, tx, in.Version, now)
		if err == nil {
			tx.Rollback()
			found.DynamicEnabled = in.Policy.DynamicEnabled
			setPricingBasket(&found, in)
			return found, nil
		}
	}
	tx.Rollback()
	if err == nil && in.GlobalPending {
		err = sql.ErrNoRows
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.PricingDecision{}, err
	}
	d := model.PricingDecision{CouponFingerprint: couponFingerprint(in.Coupons), DynamicEnabled: in.Policy.DynamicEnabled, Version: in.Version, Status: "UNCHANGED", Reason: "FIXED_PRICE", PublicPrice: model.TXBMoney(in.Public), PersonalPrice: model.TXBMoney(in.Public), NetPrice: model.TXBMoney(in.Public), GeneratedAt: now, ValidUntil: in.Until, ModelVersion: in.ModelID, Calibration: in.Calibration}
	if in.GlobalPending {
		if !attempted {
			e := s.PricingOptimizeProduct(ctx, kind, product, now)
			if e == nil {
				return s.preparePersonalPricing(pricingFreshContext(ctx), user, kind, product, combo, squads, now, true)
			}
			if !errors.Is(e, ErrPricingComputePending) {
				return d, e
			}
		}
		d.Status = "PENDING"
		d.Reason = "CALCULATION_PENDING"
		return d, nil
	}
	if in.Unavailable != "" {
		d.Status = "STALE"
		d.Reason = in.Unavailable
		return d, nil
	}
	observation := in.Observation
	observation.LogPrice = math.Log(float64(max(1, in.Public)) / float64(max(1, in.Policy.FixedPrice)))
	potential, e := personalBasketDiscount(in, in.Public)
	if e != nil {
		return d, e
	}
	if len(in.Coupons) > 0 {
		observation.CouponPresent = 1
		observation.CouponDiscount = float64(potential) / float64(max(1, in.Public+in.BasketOther))
	}
	probability := in.Model.Predict(observation)
	d.Probability = &probability
	selected := in.Public
	if in.Policy.DynamicEnabled && in.Policy.PurchaseMode == "DYNAMIC" && len(in.Coupons) > 0 {
		d.Reason = "MODEL_UNVALIDATED"
		if !in.GlobalValidated {
			d.Reason = "GLOBAL_EVIDENCE_MISSING"
		}
		if in.Model.Validated && in.CouponSamples < 30 {
			d.Reason = "COUPON_EVIDENCE_MISSING"
		}
		if !in.CostsComplete {
			d.Reason = "COST_EVIDENCE_MISSING"
		}
		if in.Model.Validated && in.CouponSamples >= 30 && in.GlobalValidated && in.CostsComplete && in.Support != nil && in.Support.Allows(in.Public, in.Policy.FixedPrice, true) {
			candidates, e := personalCandidates(in)
			if e != nil {
				return d, e
			}
			out, e := s.PricingCalculate(ctx, compute.Task{Kind: "candidate_eval_v1", Model: in.Model, Candidates: candidates})
			if errors.Is(e, ErrPricingComputePending) {
				d.Status = "PENDING"
				d.Reason = "CALCULATION_PENDING"
				return d, nil
			}
			if e != nil {
				return d, e
			}
			scores := []pricing.PriceCandidate{}
			for _, c := range out.Candidates {
				scores = append(scores, pricing.PriceCandidate{Price: c.Price - in.BasketOther, Probability: c.Probability, Contribution: c.Contribution, Eligible: c.Contribution >= 0})
			}
			if best, ok := pricing.StableCandidate(in.Public, scores); ok {
				selected = best.Price
				probability := best.Probability
				d.Probability = &probability
			}

			d.Reason = "COUPON_NET_REVENUE"
			if selected != in.Public {
				d.Status = "ADJUSTED"
			}
		}
	} else if in.Policy.DynamicEnabled {
		d.Reason = "PUBLIC_PRICE_ACCEPTED"
	}
	discount, err := personalBasketDiscount(in, selected)
	if err != nil {
		return d, err
	}
	d.PersonalPrice, d.NetPrice = model.TXBMoney(selected), model.TXBMoney(selected+in.BasketOther-discount)
	setPricingBasket(&d, in)
	raw, err := json.Marshal(in.Coupons)
	if err != nil {
		return d, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_personal_decisions(version,user_id,product_type,product_id,context_key,policy_version,public_minor,personal_minor,net_minor,status,reason,model_version,probability,calibration,generated_at,valid_until,coupon_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(version) DO NOTHING`, d.Version, user, kind, product, in.Context, in.Policy.Version, in.Public, selected, selected+in.BasketOther-discount, d.Status, d.Reason, d.ModelVersion, d.Probability, d.Calibration, pricingStamp(now), pricingStamp(in.Until), string(raw))
	return d, err
}

func setPricingBasket(d *model.PricingDecision, in personalPriceInput) {
	if in.BasketOther > 0 {
		total := model.TXBMoney(moneyMinor(d.PersonalPrice) + in.BasketOther)
		d.BasketGrossPrice = &total
	}
}

func personalCandidates(in personalPriceInput) ([]compute.CandidateInput, error) {
	high := in.Public
	if in.Support != nil {
		high = min(pricing.MaxPrice-in.BasketOther, int64(math.Floor(float64(in.Policy.FixedPrice)*math.Exp(in.Support.CouponHigh))))
	}
	high = max(in.Public, high)
	rows := make([]compute.CandidateInput, 0, 102)
	prices := []int64{in.Public}
	for step := int64(1); step <= 100; step++ {
		prices = append(prices, in.Public+(high-in.Public)*step/100)
	}
	seen := map[int64]bool{}
	for _, price := range prices {
		if seen[price] {
			continue
		}
		seen[price] = true
		discount, err := personalBasketDiscount(in, price)
		if err != nil {
			return nil, err
		}
		x := in.Observation
		x.LogPrice = math.Log(float64(max(1, price)) / float64(max(1, in.Policy.FixedPrice)))
		x.CouponPresent = 1
		x.CouponDiscount = float64(discount) / float64(max(1, price+in.BasketOther))
		rows = append(rows, compute.CandidateInput{Price: price + in.BasketOther, NetPrice: price + in.BasketOther - discount, Units: 1, Cost: in.Model.Predict(x) * in.BasketCost, Features: x})
	}
	return rows, nil
}
