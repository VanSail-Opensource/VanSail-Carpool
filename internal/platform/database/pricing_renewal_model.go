package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func (s *Store) TrainPricingRenewalResponse(ctx context.Context, product string, now time.Time) error {
	id := "renewal-response:" + product + ":coupons-v3:" + now.Format("2006-01-02")
	if revision, ok := ctx.Value(pricingModelRevisionKey{}).(string); ok {
		id += ":" + revision
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_model_versions WHERE id=?)`, id).Scan(&exists); err != nil || exists == 1 {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT q.price_minor,c.contractual_price_minor,q.state,q.trace_json FROM pricing_renewal_quotes q JOIN pricing_contracts c ON c.id=q.contract_id JOIN purchases p ON p.id=q.purchase_id WHERE c.product_id=? AND q.authorized_at IS NOT NULL AND q.health_state='HEALTHY' AND q.outcome_reason<>'INSUFFICIENT_FUNDS' AND p.reward_renewal_price_minor IS NULL AND q.state<>'CANCELLED' AND (q.state='SETTLED' OR q.valid_until<=?) AND NOT EXISTS(SELECT 1 FROM pricing_operational_incidents i WHERE i.started_at<q.valid_until AND (i.ended_at IS NULL OR i.ended_at>q.settle_after)) AND NOT EXISTS(SELECT 1 FROM pricing_contracts newer WHERE newer.purchase_id=c.purchase_id AND newer.version>c.version AND newer.recorded_at<q.valid_until) ORDER BY q.generated_at DESC LIMIT 2048`, product, pricingStamp(now))
	if err != nil {
		return err
	}
	samples := []pricing.ResponseObservation{}
	for rows.Next() {
		var price, reference int64
		var state, traceRaw string
		if err = rows.Scan(&price, &reference, &state, &traceRaw); err != nil {
			rows.Close()
			return err
		}
		var trace struct {
			Coupon      *coupons.Coupon             `json:"coupon"`
			Observation pricing.ResponseObservation `json:"observation"`
		}
		if err = json.Unmarshal([]byte(traceRaw), &trace); err != nil {
			rows.Close()
			return err
		}
		x := trace.Observation
		x.LogPrice = math.Log(float64(max(1, price)) / float64(max(1, reference)))
		x.Converted = state == "SETTLED"
		if trace.Coupon != nil {
			discount, e := attachedDiscountMinor(*trace.Coupon, price)
			if e != nil {
				rows.Close()
				return e
			}
			x.CouponPresent = 1
			x.CouponDiscount = float64(discount) / float64(max(1, price))
		}
		samples = append(samples, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	slices.Reverse(samples)
	if len(samples) == 0 {
		return nil
	}
	m, err := s.pricingFitResponse(ctx, samples)
	if err != nil {
		return err
	}
	couponSamples := 0
	for _, sample := range samples[max(0, len(samples)-m.Samples):] {
		if sample.CouponPresent > 0 && sample.CouponDiscount > 0 {
			couponSamples++
		}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	status := "PRIOR_ONLY"
	if len(samples) >= 20 {
		status = "UNVALIDATED"
	}
	if m.Validated {
		status = "VALIDATED"
	}
	diagnostics, _ := json.Marshal(map[string]any{"model": m, "couponSamples": couponSamples})
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_model_versions(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, "renewal:"+product, pricingStamp(now), status, string(raw), `{"source":"authorized completed renewal quotes; incident and superseded windows excluded","features":["intercept","logPrice/contractPrice","activity","renewals","couponPresent","couponDiscount"],"holdout":"chronological 20 percent"}`, pricingStamp(now), string(diagnostics))
	return err
}

func pricingRenewalModelTx(ctx context.Context, tx *sql.Tx, product string, now time.Time) (pricing.ResponseModel, string, error) {
	var m pricing.ResponseModel
	var id, raw string
	err := tx.QueryRowContext(ctx, `SELECT id,parameters_json FROM pricing_model_versions WHERE kind=? AND id LIKE '%:coupons-v3:%' AND created_at<=? AND frozen=0 ORDER BY created_at DESC LIMIT 1`, "renewal:"+product, pricingStamp(now)).Scan(&id, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return m, "renewal-prior-v1", nil
	}
	if err != nil {
		return m, id, err
	}
	err = json.Unmarshal([]byte(raw), &m)
	return m, id, err
}
