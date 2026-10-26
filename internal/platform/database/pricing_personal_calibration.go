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
	rows, err := s.db.QueryContext(ctx, `SELECT f.predicted_p50,f.predicted_p10,f.predicted_p90,e.actual,e.status FROM pricing_forecast_vintages f JOIN pricing_forecast_evaluations e ON e.forecast_id=f.forecast_id WHERE f.entity_type='user' AND f.entity_id=? AND f.model_version='empirical-user-total-v1' AND f.horizon_seconds=86400 ORDER BY f.target_from DESC LIMIT 90`, user)
	if err != nil {
		return err
	}
	values := []pricing.Evaluation{}
	for rows.Next() {
		var e pricing.Evaluation
		if err = rows.Scan(&e.Predicted, &e.P10, &e.P90, &e.Actual, &e.Status); err != nil {
			rows.Close()
			return err
		}
		values = append(values, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	v := pricing.Evaluate(values)
	out.Confidence.Samples = v.Samples
	if v.Samples > 0 {
		out.Confidence.EvaluatedIntervalCoverage = &v.Coverage
	}
	if v.Samples >= 30 && v.Coverage >= .65 && v.Coverage <= .95 && v.WAPE != nil && *v.WAPE <= .35 {
		out.Confidence.Calibration = "VALIDATED"
	}
	return nil
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
