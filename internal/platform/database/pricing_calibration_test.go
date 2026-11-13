package database

import (
	"context"
	"testing"
	"time"
)

func TestPricingCalibrationSummarizesMatureObservedOffers(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for _, offer := range []struct {
		id, reason  string
		probability float64
		outcome     any
		days        int
		mature      bool
	}{
		{"success", "PURCHASED", 0.8, 1, -1, true},
		{"declined", "", 0.2, 0, -1, true},
		{"unfunded", "INSUFFICIENT_FUNDS", 0.2, 0, -1, true},
		{"unobserved", "", 0.2, nil, -1, true},
		{"live", "", 0.2, 0, -1, false},
		{"too-old", "", 0.2, 0, -91, true},
	} {
		exposed := now.AddDate(0, 0, offer.days)
		expires := exposed.Add(time.Hour)
		if !offer.mature {
			expires = now.Add(time.Hour)
		}
		_, err := store.DB().ExecContext(ctx, `INSERT INTO pricing_offers
			(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,
			model_version,predicted_probability,features_json,outcome_reason)
			VALUES($1,$1,'combo','test',100,100,$2,$3,$4,$3,'test-model',$5,'{}',$6)`,
			offer.id, pricingStamp(exposed), pricingStamp(expires), offer.outcome, offer.probability, offer.reason)
		if err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := store.SummarizePricingCalibration(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var buckets, samples, observed int
	var probability, brier float64
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*),sum(samples),sum(observed_sum),sum(probability_sum),sum(brier_sum)
		FROM pricing_calibration_daily WHERE day=$1 AND kind='purchase'`, now.AddDate(0, 0, -1).Format(time.DateOnly)).
		Scan(&buckets, &samples, &observed, &probability, &brier); err != nil {
		t.Fatal(err)
	}
	if buckets != 2 || samples != 2 || observed != 1 || probability != 1 || brier < 0.079999 || brier > 0.080001 {
		t.Fatalf("calibration = buckets %d, samples %d, observed %d, probability %v, brier %v", buckets, samples, observed, probability, brier)
	}
	var stale int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM pricing_calibration_daily WHERE day=$1`, now.AddDate(0, 0, -91).Format(time.DateOnly)).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("out-of-window calibration = %d, %v", stale, err)
	}
}

func TestPricingCalibrationPublishesPurchaseAndRenewalAtomically(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	before := now.Add(-24 * time.Hour)
	user := createTestUser(t, store, 69_033)
	combo := saveTestCombo(t, store, "calibration-atomicity", 100, 1)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "seed", "seed", before); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "calibration"}, before)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO pricing_contracts
		(id,purchase_id,user_id,product_id,mode,contractual_price_minor,price_limit_minor,max_change_bps,policy_version,recorded_at,source_purchase_id)
		VALUES('contract',$1,$2,$3,'DYNAMIC',100,100,0,1,$4,$1)`, purchase.ID, user.ID, combo.ID, pricingStamp(before))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO pricing_renewal_quotes
		(id,purchase_id,contract_id,price_minor,policy_version,contract_version,state,generated_at,authorized_at,settle_after,valid_until,
		probability,contribution,health_state,model_version,trace_json)
		VALUES('renewal',$1,'contract',100,1,1,'SETTLED',$2,$2,$2,$3,0.8,100,'HEALTHY','test-model','{}')`,
		purchase.ID, pricingStamp(before), pricingStamp(before.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO pricing_offers
		(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,outcome,observed_at,model_version,predicted_probability,features_json)
		VALUES('purchase',$1,'combo',$2,100,100,$3,$4,1,$4,'test-model',0.7,'{}')`,
		user.ID, combo.ID, pricingStamp(before), pricingStamp(before.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE FUNCTION reject_renewal_calibration() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		IF NEW.kind='renewal' THEN RAISE EXCEPTION 'injected calibration failure'; END IF; RETURN NEW; END $$`,
		`CREATE TRIGGER reject_renewal_calibration BEFORE INSERT ON pricing_calibration_daily FOR EACH ROW EXECUTE FUNCTION reject_renewal_calibration()`,
	} {
		if _, err := store.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SummarizePricingCalibration(ctx, now); err == nil {
		t.Fatal("injected renewal publication failure was ignored")
	}
	var summaries int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM pricing_calibration_daily`).Scan(&summaries); err != nil || summaries != 0 {
		t.Fatalf("partial calibration survived failed publication: %d, %v", summaries, err)
	}
	if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER reject_renewal_calibration ON pricing_calibration_daily`); err != nil {
		t.Fatal(err)
	}
	if err := store.SummarizePricingCalibration(ctx, now); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"purchase", "renewal"} {
		var samples, observed int
		if err := store.DB().QueryRowContext(ctx, `SELECT samples,observed_sum FROM pricing_calibration_daily WHERE kind=$1`, kind).
			Scan(&samples, &observed); err != nil || samples != 1 || observed != 1 {
			t.Fatalf("%s calibration = %d samples, %d observed, %v", kind, samples, observed, err)
		}
	}
}
