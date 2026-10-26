package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// TrainPricingResponse fits genuine closed exposures and stores an immutable
// model with chronological holdout diagnostics and exact source watermarks.
func (s *Store) TrainPricingResponse(ctx context.Context, kind, product string, now time.Time) (pricing.ResponseModel, string, error) {
	id := "response:" + kind + ":" + product + ":coupons-v3:" + now.UTC().Format("2006-01-02")
	if revision, ok := ctx.Value(pricingModelRevisionKey{}).(string); ok {
		id += ":" + revision
	}
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT parameters_json FROM pricing_model_versions WHERE id=?`, id).Scan(&existing); err == nil {
		var m pricing.ResponseModel
		if err = json.Unmarshal([]byte(existing), &m); err != nil {
			return m, id, err
		}
		return m, id, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return pricing.ResponseModel{}, id, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT features_json,outcome FROM pricing_offers WHERE product_type=? AND product_id=? AND outcome IS NOT NULL AND outcome_reason<>'INSUFFICIENT_FUNDS' AND json_extract(features_json,'$.couponFingerprint') IS NOT NULL AND CAST(json_extract(features_json,'$.complete') AS INTEGER)=1 AND NOT EXISTS(SELECT 1 FROM purchases p WHERE p.id=pricing_offers.purchase_id AND p.reward_renewal_price_minor IS NOT NULL) AND expires_at<=? AND exposed_at>=? ORDER BY exposed_at DESC,id DESC LIMIT 2048`, kind, product, pricingStamp(now), pricingStamp(now.AddDate(0, 0, -60)))
	if err != nil {
		return pricing.ResponseModel{}, "", err
	}
	samples := []pricing.ResponseObservation{}
	for rows.Next() {
		var raw string
		var outcome int
		if err = rows.Scan(&raw, &outcome); err != nil {
			rows.Close()
			return pricing.ResponseModel{}, "", err
		}
		var sample pricing.ResponseObservation
		if err = json.Unmarshal([]byte(raw), &sample); err != nil {
			rows.Close()
			return pricing.ResponseModel{}, "", err
		}
		sample.Converted = outcome == 1
		samples = append(samples, sample)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return pricing.ResponseModel{}, "", err
	}
	slices.Reverse(samples)
	if len(samples) == 0 {
		return pricing.ResponseModel{}, "purchase-prior-v1", nil
	}
	m, err := s.pricingFitResponse(ctx, samples)
	if err != nil {
		return pricing.ResponseModel{}, id, err
	}
	couponSamples := 0
	for _, sample := range samples[max(0, len(samples)-m.Samples):] {
		if sample.CouponPresent > 0 && sample.CouponDiscount > 0 {
			couponSamples++
		}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return m, "", err
	}
	status := "UNVALIDATED"
	if len(samples) < 20 {
		status = "PRIOR_ONLY"
	}
	if m.Validated {
		status = "VALIDATED"
	}
	diagnostics, _ := json.Marshal(map[string]any{"model": m, "couponSamples": couponSamples})
	_, err = s.db.ExecContext(ctx, `INSERT INTO pricing_model_versions(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, "response:"+kind+":"+product, pricingStamp(now), status, string(raw), `{"features":["intercept","logPrice/reference","activity","renewals","couponPresent","couponDiscount"],"fit":"regularized logistic","validation":"chronological holdout; minimum 200 samples; price variation; Brier improvement; bias below 0.1; nonpositive price slope"}`, pricingStamp(now), string(diagnostics))
	return m, id, err
}
