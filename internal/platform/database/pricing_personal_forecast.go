package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) PersonalForecast(ctx context.Context, user string, now time.Time) (model.PersonalForecast, error) {
	if _, err := s.UserByID(ctx, user); err != nil {
		return model.PersonalForecast{}, err
	}
	return pricingCached(ctx, s, "personal-forecast:"+user+":"+now.UTC().Format(time.DateOnly), 20*time.Second, func() (model.PersonalForecast, error) { return s.personalForecast(ctx, user, now) })
}

func (s *Store) personalForecast(ctx context.Context, user string, now time.Time) (model.PersonalForecast, error) {
	day := now.UTC().Truncate(24 * time.Hour)
	// Missing history is not a stale prediction. Mark stale only after finding a
	// forecast vintage whose generation time predates the current UTC day.
	out := model.PersonalForecast{Days: []model.ForecastDay{}, Timezone: "UTC", Status: "MISSING", Confidence: model.ForecastConfidence{Calibration: "PRIOR_ONLY", NominalIntervalBPS: 8000}}
	for offset := -2; offset <= 5; offset++ {
		if offset == 0 {
			continue
		}
		out.Days = append(out.Days, model.ForecastDay{Date: day.AddDate(0, 0, offset).Format(time.DateOnly), Status: "MISSING"})
	}
	for i := range 2 {
		var value sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT SUM(d.upload_bytes+d.download_bytes) FROM xnode_daily_usage d JOIN xnode_identities i ON i.proxy_user_id=d.proxy_user_id LEFT JOIN xnode_payg_accounts a ON a.proxy_user_id=d.proxy_user_id WHERE COALESCE(i.user_id,a.user_id)=? AND d.date=?`, user, out.Days[i].Date).Scan(&value)
		if err != nil {
			return out, err
		}
		if value.Valid {
			actual := value.String
			out.Days[i].ActualBytes = &actual
			out.Days[i].Status = "OBSERVED"
		}
	}
	err := s.preparePersonalForecastFeatures(ctx, user, now)
	if errors.Is(err, ErrPricingComputePending) {
		out.Status = "PENDING"
		out.RefreshPending = true
	} else if err != nil {
		return out, err
	}
	var at, raw string
	// A newer snapshot can legitimately have no complete total observations.
	// Keep an older, still relevant vintage visible rather than hiding it behind
	// that empty snapshot; its original generation time marks it as stale.
	err = s.db.QueryRowContext(ctx, `SELECT f.watermark,f.features_json FROM pricing_features f WHERE f.entity_type='user' AND f.entity_id=? ORDER BY CASE WHEN EXISTS(SELECT 1 FROM pricing_forecast_vintages v WHERE v.entity_type='user' AND v.entity_id=f.entity_id AND v.metric='traffic_bytes' AND v.model_version='empirical-user-total-v1' AND v.generated_at=f.watermark AND v.target_from>=? AND v.target_from<?) THEN 0 ELSE 1 END,f.watermark DESC LIMIT 1`, user, pricingStamp(day.AddDate(0, 0, 1)), pricingStamp(day.AddDate(0, 0, 6))).Scan(&at, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var feature struct {
		Observed int `json:"dailyObservedDays"`
		NodeDays int `json:"observedNodeDays"`
		Expected int `json:"expectedNodeDays"`
	}
	if err = json.Unmarshal([]byte(raw), &feature); err != nil {
		return out, err
	}
	out.Confidence.ObservedDays = feature.Observed
	if feature.Expected > 0 {
		out.Confidence.ObservedCoverageBPS = int64(feature.NodeDays * 10000 / feature.Expected)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT generated_at,target_from,predicted_p10,predicted_p50,predicted_p90,model_version,calibration_status FROM pricing_forecast_vintages WHERE entity_type='user' AND entity_id=? AND metric='traffic_bytes' AND model_version='empirical-user-total-v1' AND generated_at=? AND target_from>=? AND target_from<? ORDER BY target_from`, user, at, pricingStamp(day.AddDate(0, 0, 1)), pricingStamp(day.AddDate(0, 0, 6)))
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var generated, target, modelID, calibration string
		var low, median, high float64
		if err = rows.Scan(&generated, &target, &low, &median, &high, &modelID, &calibration); err != nil {
			rows.Close()
			return out, err
		}
		generatedAt, e := parseStamp(generated)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.GeneratedAt = &generatedAt
		out.Stale = generatedAt.Before(day)
		out.ModelVersion = modelID
		out.Confidence.Calibration = calibration
		targetAt, e := parseStamp(target)
		if e != nil {
			rows.Close()
			return out, e
		}
		for i := 2; i < len(out.Days); i++ {
			if out.Days[i].Date == targetAt.Format(time.DateOnly) {
				l, m, h := strconv.FormatFloat(low, 'f', 0, 64), strconv.FormatFloat(median, 'f', 0, 64), strconv.FormatFloat(high, 'f', 0, 64)
				out.Days[i].LowerBytes = &l
				out.Days[i].PredictedBytes = &m
				out.Days[i].UpperBytes = &h
				out.Days[i].Status = "PREDICTED"
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if out.GeneratedAt != nil {
		out.Status = "AVAILABLE"
		if out.Stale {
			out.Status = "STALE"
		}
	}
	if err = s.personalForecastCalibration(ctx, user, &out); err != nil {
		return out, err
	}
	out.RenewalDecision, err = s.preparePersonalRenewalDecision(ctx, user, now)
	if err != nil {
		return out, err
	}
	return out, nil
}

// Interactive reads share the worker's bounded analytical admission. Completed
// daily features need no slot; a busy worker leaves the refresh pending.
func (s *Store) preparePersonalForecastFeatures(ctx context.Context, user string, now time.Time) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_features WHERE entity_type='user' AND entity_id=? AND day=?)`, user, now.UTC().Format(time.DateOnly)).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	release, err := s.acquirePricingCompute()
	if err != nil {
		return ErrPricingComputePending
	}
	defer release()
	return s.pricingUserFeatures(ctx, user, now)
}
