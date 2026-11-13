package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestPricingCleanupSealsBeforePartialPurgeAndDefersForSourceWriters(t *testing.T) {
	f := newPricingSummaryFixture(t)
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,model_version,predicted_probability,features_json) SELECT 'retained-'||n,'member-'||n,'combo','shared',100,100,$1,$2,1,$2,'summary-model',0.5,'{}' FROM generate_series(1,1001) n`, pricingStamp(f.at), pricingStamp(f.at.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SummarizePricingCalibration(f.ctx, f.at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	writer, err := f.store.DB().BeginTx(f.ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err = writer.ExecContext(f.ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended('pricing.calibration',0))`); err != nil {
		t.Fatal(err)
	}
	now := f.at.AddDate(0, 0, 400)
	counts, err := f.store.PricingCleanup(f.ctx, now)
	if err != nil || len(counts) != 0 {
		t.Fatal("cleanup did not defer while a source writer was active", counts, err)
	}
	if err = writer.Commit(); err != nil {
		t.Fatal(err)
	}
	counts, err = f.store.PricingCleanup(f.ctx, now)
	if err != nil || counts["offers"] != 1000 || counts["calibration"] != 0 {
		t.Fatal("bounded purge lost cohort identity before remaining source", counts, err)
	}
	var samples, observed, remaining int
	var sealed, stale, dirty bool
	if err = f.store.DB().QueryRowContext(f.ctx, `SELECT samples,observed_sum,sealed,stale,dirty FROM pricing_calibration_daily`).Scan(&samples, &observed, &sealed, &stale, &dirty); err != nil || samples != 1001 || observed != 1001 || !sealed || stale || dirty {
		t.Fatal("first purge did not seal the complete original aggregate", samples, observed, sealed, stale, dirty, err)
	}
	if err = f.store.DB().QueryRowContext(f.ctx, `SELECT COUNT(*) FROM pricing_offers`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal(remaining, err)
	}
	if _, err = f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=0`); err != nil {
		t.Fatal(err)
	}
	if err = f.store.SummarizePricingCalibration(f.ctx, now); err != nil {
		t.Fatal(err)
	}
	if err = f.store.DB().QueryRowContext(f.ctx, `SELECT samples,observed_sum,stale FROM pricing_calibration_daily`).Scan(&samples, &observed, &stale); err != nil || samples != 1001 || observed != 1001 || !stale {
		t.Fatal("late correction replaced compacted history with one remaining row", samples, observed, stale, err)
	}
	counts, err = f.store.PricingCleanup(f.ctx, now)
	if err != nil || counts["offers"] != 1 || counts["calibration"] != 1 {
		t.Fatal("completed source purge did not release expired cohort metadata", counts, err)
	}
}

func TestPricingCleanupKeepsUnevaluatedForecastSources(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	f := pricingForecastFixture(t, s, "observed-retained", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	other := f
	other.ID = "unobserved-retained"
	other.GeneratedAt = f.GeneratedAt.Add(time.Nanosecond)
	if err := s.SavePricingForecast(ctx, other); err != nil {
		t.Fatal(err)
	}
	actual := 70.0
	if err := s.EvaluatePricingForecast(ctx, f.ID, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	now := f.TargetUntil.AddDate(0, 0, 400)
	counts, err := s.PricingCleanup(ctx, now)
	if err != nil || counts["forecasts"] != 0 {
		t.Fatal("cleanup removed a source cohort before all actuals were evaluated", counts, err)
	}
	var sealed bool
	if err := s.db.QueryRowContext(ctx, `SELECT sealed FROM pricing_model_validation_daily`).Scan(&sealed); err != nil || sealed {
		t.Fatal("incomplete cohort was marked sealed", sealed, err)
	}
	if err := s.EvaluatePricingForecast(ctx, other.ID, "MISSING", nil, f.TargetUntil, now); err != nil {
		t.Fatal(err)
	}
	counts, err = s.PricingCleanup(ctx, now)
	if err != nil || counts["forecasts"] != 2 || counts["summaries"] != 1 {
		t.Fatal("explicit missing observation did not permit complete-cohort retention", counts, err)
	}
}

func TestPricingInitiallyExcludedCohortCanCompleteRetention(t *testing.T) {
	f := newPricingSummaryFixture(t)
	f.insertOffer(t)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=0,outcome_reason='INSUFFICIENT_FUNDS',observed_at=expires_at`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SummarizePricingCalibration(f.ctx, f.at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var samples, bucket int
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT samples,bucket FROM pricing_calibration_daily`).Scan(&samples, &bucket); err != nil || samples != 0 || bucket != 0 {
		t.Fatal("initially excluded source never gained a cohort identity", samples, bucket, err)
	}
	counts, err := f.store.PricingCleanup(f.ctx, f.at.AddDate(0, 0, 91))
	if err != nil || counts["offers"] != 1 {
		t.Fatal("fully excluded completed source could not be retained safely", counts, err)
	}
}

func TestPricingFinancialExpirySurvivesSummaryFailure(t *testing.T) {
	s, user, actor, purchase, policy, at := pricingRenewalFixture(t)
	ctx := context.Background()
	quote, err := s.PreparePricingRenewal(ctx, purchase.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	now := quote.ValidUntil.Add(time.Hour)
	if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_admin_overrides(id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at) VALUES('expiry','product',$1,'MANUAL_PRICE','retained fixture',$2,$3,$4,$5,'{}','{}','ACTIVE',$3)`, "combo:"+purchase.ComboID, actor.ID, pricingStamp(at), pricingStamp(quote.ValidUntil), policy.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE pricing_product_settings SET override_status='MANUAL' WHERE product_type='combo' AND product_id=$1`, purchase.ComboID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO pricing_discount_spend(override_id,reference_id,user_id,discount_minor,created_at,status) VALUES('expiry',$1,$2,100,$3,'RESERVED')`, "renewal:"+quote.ID, user.ID, pricingStamp(at)); err != nil {
		t.Fatal(err)
	}
	f := pricingForecastFixture(t, s, "expiry-failure", at.AddDate(0, 0, -3))
	if err = s.EvaluatePricingForecast(ctx, f.ID, "MISSING", nil, f.TargetUntil, at); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE FUNCTION reject_validation_publication() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected summary failure'; END $$`,
		`CREATE TRIGGER reject_validation_publication BEFORE INSERT ON pricing_model_validation_daily FOR EACH ROW EXECUTE FUNCTION reject_validation_publication()`,
	} {
		if _, err = s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.PricingCleanup(ctx, now); err == nil {
		t.Fatal("injected analytical failure was ignored")
	}
	var reservation, intervention, status string
	if err = s.db.QueryRowContext(ctx, `SELECT d.status,o.status,p.override_status FROM pricing_discount_spend d JOIN pricing_admin_overrides o ON o.id=d.override_id JOIN pricing_product_settings p ON p.product_type='combo' AND p.product_id=$1 WHERE d.override_id='expiry'`, purchase.ComboID).Scan(&reservation, &intervention, &status); err != nil || reservation != "RELEASED" || intervention != "EXPIRED" || status != "NONE" {
		t.Fatal("analytical failure prevented independent financial expiry", reservation, intervention, status, err)
	}
}
