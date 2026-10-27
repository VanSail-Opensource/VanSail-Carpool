package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func (s *Store) personalForecastCalibration(ctx context.Context, user string, out *model.PersonalForecast) error {
	rows, err := s.db.QueryContext(ctx, `WITH originals AS (SELECT f.horizon_seconds,f.target_from,f.generated_at,f.predicted_p50,f.predicted_p10,f.predicted_p90,e.actual,e.status,ROW_NUMBER() OVER(PARTITION BY f.horizon_seconds,f.target_from ORDER BY f.generated_at DESC) AS vintage_rank FROM pricing_forecast_vintages f JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE f.entity_type='user' AND f.entity_id=? AND f.metric='traffic_bytes' AND f.model_version='empirical-user-total-v1' AND f.horizon_seconds IN (86400,172800,259200,345600,432000)) SELECT horizon_seconds,predicted_p50,predicted_p10,predicted_p90,actual,status FROM (SELECT *,ROW_NUMBER() OVER(PARTITION BY horizon_seconds ORDER BY target_from DESC) AS ordinal FROM originals WHERE vintage_rank=1) v WHERE ordinal<=90`, user)
	if err != nil {
		return err
	}
	values := map[int][]pricing.Evaluation{}
	for rows.Next() {
		var e pricing.Evaluation
		var seconds int
		if err = rows.Scan(&seconds, &e.Predicted, &e.P10, &e.P90, &e.Actual, &e.Status); err != nil {
			rows.Close()
			return err
		}
		values[seconds/86400] = append(values[seconds/86400], e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	applyPersonalCalibration(&out.Confidence, values)
	return nil
}

func applyPersonalCalibration(out *model.ForecastConfidence, values map[int][]pricing.Evaluation) {
	out.ByLead = make([]model.ForecastLeadConfidence, 0, 5)
	out.Calibration = "UNVALIDATED"
	allValidated := true
	for lead := 1; lead <= 5; lead++ {
		v := pricing.Evaluate(values[lead])
		item := model.ForecastLeadConfidence{LeadDays: lead, Samples: v.Samples, Calibration: "UNVALIDATED", WAPE: v.WAPE}
		if v.Samples > 0 {
			item.IntervalCoverage = &v.Coverage
		}
		if v.Samples >= 30 && v.Coverage >= .65 && v.Coverage <= .95 && v.WAPE != nil && *v.WAPE <= .35 {
			item.Calibration = "VALIDATED"
		} else {
			allValidated = false
		}
		if lead == 1 {
			out.Samples, out.EvaluatedIntervalCoverage = v.Samples, item.IntervalCoverage
		}
		out.ByLead = append(out.ByLead, item)
	}
	if allValidated {
		out.Calibration = "VALIDATED"
	}
}

func (s *Store) retryAttempt_PersonalRenewalDecision(ctx context.Context, user string, now time.Time) (*model.PricingDecision, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return s.queuedRenewalDecisionTx(ctx, tx, user, now)
}

func (s *Store) preparePersonalRenewalDecision(ctx context.Context, user string, now time.Time) (*model.PricingDecision, error) {
	var current string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC LIMIT 1`, user, pricingStamp(now)).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	err = s.prepareOwnedPricingRenewal(ctx, user, current, now)
	if errors.Is(err, ErrPricingComputePending) || errors.Is(err, ErrPricingDecisionPending) {
		return &model.PricingDecision{DynamicEnabled: true, Status: "PENDING", Reason: "CALCULATION_PENDING", PublicPrice: model.TXBMoney(0), PersonalPrice: model.TXBMoney(0), NetPrice: model.TXBMoney(0), Calibration: "UNAVAILABLE", GeneratedAt: now, ValidUntil: now.Add(5 * time.Minute)}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.PersonalRenewalDecision(ctx, user, now)
}
