package database

import (
	"context"
	"testing"
	"time"
)

type pricingSummaryFixture struct {
	ctx   context.Context
	store *Store
	at    time.Time
	day   string
}

func newPricingSummaryFixture(t *testing.T) pricingSummaryFixture {
	t.Helper()
	store := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	return pricingSummaryFixture{ctx: ctx, store: store, at: at, day: at.Format(time.DateOnly)}
}

func (f pricingSummaryFixture) insertOffer(t *testing.T) {
	t.Helper()
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,
	 price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json)
	 VALUES('summary-offer','summary-owner','combo','summary-product',100,100,$1,$2,'summary-model',0.5,'{}')`, pricingStamp(f.at), pricingStamp(f.at.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
}

func (f pricingSummaryFixture) insertSummary(t *testing.T, kind string, sealed bool) {
	t.Helper()
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_calibration_daily
	 (day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at,sealed)
	 VALUES($1,'summary-model',$2,5,10,5,3,2.5,$3,$4)`, f.day, kind, pricingStamp(f.at.Add(2*time.Hour)), sealed); err != nil {
		t.Fatal(err)
	}
}

func (f pricingSummaryFixture) requireSummary(t *testing.T, kind string, exists, sealed, stale, dirty bool) {
	t.Helper()
	var count int
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT COUNT(*) FROM pricing_calibration_daily WHERE day=$1 AND model_version='summary-model' AND kind=$2`, f.day, kind).
		Scan(&count); err != nil || (count == 1) != exists || count > 1 {
		t.Fatal("unexpected summary existence", count, exists, err)
	}
	if !exists {
		return
	}
	var actualSealed, actualStale, actualDirty bool
	var samples, observed int
	var probability, brier float64
	var updated string
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT samples,probability_sum,observed_sum,brier_sum,updated_at,sealed,stale,dirty
	 FROM pricing_calibration_daily WHERE day=$1 AND model_version='summary-model' AND kind=$2`, f.day, kind).
		Scan(&samples, &probability, &observed, &brier, &updated, &actualSealed, &actualStale, &actualDirty); err != nil || samples != 10 || probability != 5 || observed != 3 || brier != 2.5 ||
		updated != pricingStamp(f.at.Add(2*time.Hour)) || actualSealed != sealed || actualStale != stale || actualDirty != dirty {
		t.Fatal("summary state or retained evidence changed", samples, probability, observed, brier, updated, actualSealed, actualStale, actualDirty, err)
	}
}
