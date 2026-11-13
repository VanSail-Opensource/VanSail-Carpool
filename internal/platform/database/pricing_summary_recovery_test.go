package database

import (
	"context"
	"testing"
	"time"
)

func TestPricingCalibrationRebuildsEmptyAndAgedDirtyCohorts(t *testing.T) {
	f := newPricingSummaryFixture(t)
	f.insertOffer(t)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=1,observed_at=expires_at WHERE id='summary-offer'`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SummarizePricingCalibration(f.ctx, f.at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome_reason='INSUFFICIENT_FUNDS' WHERE id='summary-offer'`); err != nil {
		t.Fatal(err)
	}
	var dirty bool
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT dirty FROM pricing_calibration_daily`).Scan(&dirty); err != nil || !dirty {
		t.Fatal("excluded last observation lost its cohort identity", dirty, err)
	}
	if err := f.store.SummarizePricingCalibration(f.ctx, f.at.AddDate(0, 0, 91)); err != nil {
		t.Fatal(err)
	}
	var samples, observed, bucket int
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT samples,observed_sum,bucket,dirty FROM pricing_calibration_daily`).Scan(&samples, &observed, &bucket, &dirty); err != nil || samples != 0 || observed != 0 || bucket != 0 || dirty {
		t.Fatal("empty cohort retained obsolete observations or lost its marker", samples, observed, bucket, dirty, err)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome_reason='' WHERE id='summary-offer'`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SummarizePricingCalibration(f.ctx, f.at.AddDate(0, 0, 92)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT samples,observed_sum,bucket,dirty FROM pricing_calibration_daily`).Scan(&samples, &observed, &bucket, &dirty); err != nil || samples != 1 || observed != 1 || bucket != 5 || dirty {
		t.Fatal("aged dirty cohort did not rebuild from complete retained source", samples, observed, bucket, dirty, err)
	}
}

func TestPricingCalibrationMaturityCannotHideBehindNewerObservationTime(t *testing.T) {
	f := newPricingSummaryFixture(t)
	for i, hours := range []int{1, 3} {
		if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,model_version,predicted_probability,features_json) VALUES($1,$1,'combo','shared',100,100,$2,$3,1,$4,'summary-model',0.5,'{}')`, []string{"first", "later"}[i], pricingStamp(f.at), pricingStamp(f.at.Add(time.Duration(hours)*time.Hour)), pricingStamp(f.at.Add(time.Duration(4-i*3)*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct{ hours, samples int }{{2, 1}, {3, 2}} {
		if err := f.store.SummarizePricingCalibration(f.ctx, f.at.Add(time.Duration(step.hours)*time.Hour)); err != nil {
			t.Fatal(err)
		}
		var samples int
		if err := f.store.DB().QueryRowContext(f.ctx, `SELECT samples FROM pricing_calibration_daily`).Scan(&samples); err != nil || samples != step.samples {
			t.Fatal("newly mature evidence was hidden by a newer source timestamp", samples, step.samples, err)
		}
	}
}

func TestPricingValidationTracksBackdatedSourcesAndPreservesSealedMetrics(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	f := pricingForecastFixture(t, s, "first-actual", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	other := f
	other.ID = "backdated-actual"
	other.GeneratedAt = f.GeneratedAt.Add(time.Nanosecond)
	if err := s.SavePricingForecast(ctx, other); err != nil {
		t.Fatal(err)
	}
	actual := 70.0
	for _, entry := range []struct {
		id          string
		hour, count int
	}{{f.ID, 2, 1}, {other.ID, 1, 2}} {
		if err := s.EvaluatePricingForecast(ctx, entry.id, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil.Add(time.Duration(entry.hour)*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := s.SummarizePricingValidation(ctx, f.TargetUntil.Add(4*time.Hour)); err != nil {
			t.Fatal(err)
		}
		var samples int
		var watermark int64
		var dirty bool
		if err := s.db.QueryRowContext(ctx, `SELECT samples,updated_at_ns,dirty FROM pricing_model_validation_daily`).Scan(&samples, &watermark, &dirty); err != nil || samples != entry.count || watermark != f.TargetUntil.Add(2*time.Hour).UnixNano() || dirty {
			t.Fatal("backdated committed actual was not summarized exactly", samples, watermark, dirty, err)
		}
	}
	var before string
	if err := s.db.QueryRowContext(ctx, `UPDATE pricing_model_validation_daily SET sealed=true RETURNING metrics_json`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, hour := range []int{1, 2} {
		if err := s.EvaluatePricingForecast(ctx, other.ID, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil.Add(time.Duration(hour)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var stale bool
	if err := s.db.QueryRowContext(ctx, `SELECT stale FROM pricing_model_validation_daily`).Scan(&stale); err != nil || stale {
		t.Fatal("idempotent replay or unchanged re-evaluation made sealed evidence stale", stale, err)
	}
	actual = 75
	if err := s.EvaluatePricingForecast(ctx, other.ID, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.SummarizePricingValidation(ctx, f.TargetUntil.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var after string
	var dirty bool
	if err := s.db.QueryRowContext(ctx, `SELECT metrics_json,stale,dirty FROM pricing_model_validation_daily`).Scan(&after, &stale, &dirty); err != nil || before != after || !stale || dirty {
		t.Fatal("sealed metrics were rebuilt from changing source", before, after, stale, dirty, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT actual FROM pricing_forecast_actuals WHERE forecast_id=$1`, pricingForecastID(other.ID)).Scan(&actual); err != nil || actual != 75 {
		t.Fatal("sealed analytics blocked the authoritative late actual", actual, err)
	}
}
