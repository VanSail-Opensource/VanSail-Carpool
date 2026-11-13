package database

import (
	"testing"
	"time"
)

func TestPricingCalibrationOfferChangesRetainSealedEvidence(t *testing.T) {
	t.Parallel()
	f := newPricingSummaryFixture(t)
	f.insertOffer(t)
	f.insertSummary(t, "purchase", false)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=1,observed_at=$1 WHERE id='summary-offer'`, pricingStamp(f.at)); err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "purchase", true, false, false, true)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_calibration_daily SET sealed=true WHERE kind='purchase'`); err == nil {
		t.Fatal("dirty summary was sealed before a complete rebuild")
	}
	// Simulate a successful complete rebuild followed by compaction sealing.
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_calibration_daily SET dirty=false,sealed=true WHERE kind='purchase'`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE pricing_offers SET outcome=outcome WHERE id='summary-offer'`,
		`INSERT INTO pricing_offers SELECT * FROM pricing_offers WHERE id='summary-offer' ON CONFLICT(id) DO NOTHING`,
	} {
		if _, err := f.store.DB().ExecContext(f.ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	f.requireSummary(t, "purchase", true, true, false, false)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=0 WHERE id='summary-offer'`); err != nil {
		t.Fatal("derived sealing blocked source correction", err)
	}
	f.requireSummary(t, "purchase", true, true, true, false)
	if _, err := f.store.DB().ExecContext(f.ctx, `DELETE FROM pricing_offers WHERE id='summary-offer'`); err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "purchase", true, true, true, false)
}

func TestPricingCalibrationExcludedOutcomeDoesNotInvalidate(t *testing.T) {
	t.Parallel()
	f := newPricingSummaryFixture(t)
	f.insertOffer(t)
	f.insertSummary(t, "purchase", false)
	for _, query := range []string{
		`UPDATE pricing_offers SET outcome_reason='INSUFFICIENT_FUNDS' WHERE id='summary-offer'`,
		`UPDATE pricing_offers SET outcome=0 WHERE id='summary-offer'`,
	} {
		if _, err := f.store.DB().ExecContext(f.ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	f.requireSummary(t, "purchase", true, false, false, false)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome_reason='' WHERE id='summary-offer'`); err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "purchase", true, false, false, true)
}

func TestPricingCalibrationLateRenewalCleanupMarksStale(t *testing.T) {
	t.Parallel()
	f := newPricingSummaryFixture(t)
	owner := createTestUser(t, f.store, 900_606)
	combo := saveTestCombo(t, f.store, "summary-renewal", 100, 1)
	if _, err := f.store.AdjustBalance(f.ctx, owner.ID, 100, "seed", "seed", f.at); err != nil {
		t.Fatal(err)
	}
	purchase, err := f.store.CreatePurchase(f.ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, IdempotencyKey: "summary-renewal"}, f.at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_contracts
	 (id,purchase_id,user_id,product_id,mode,contractual_price_minor,price_limit_minor,max_change_bps,policy_version,recorded_at,source_purchase_id)
	 VALUES('summary-contract',$1,$2,$3,'DYNAMIC',100,100,0,1,$4,$1)`, purchase.ID, owner.ID, combo.ID, pricingStamp(f.at)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_renewal_quotes
	 (id,purchase_id,contract_id,price_minor,policy_version,contract_version,state,generated_at,authorized_at,settle_after,valid_until,
	 probability,contribution,health_state,model_version,trace_json)
	 VALUES('summary-quote',$1,'summary-contract',100,1,1,'AUTHORIZED',$2,$2,$2,$3,0.5,100,'HEALTHY','summary-model','{}')`,
		purchase.ID, pricingStamp(f.at), pricingStamp(f.at.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	f.insertSummary(t, "renewal", false)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='INSUFFICIENT_FUNDS' WHERE id='summary-quote'`); err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "renewal", true, false, false, true)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='' WHERE id='summary-quote'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_calibration_daily SET dirty=false,sealed=true WHERE kind='renewal'`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE pricing_renewal_quotes SET state=state WHERE id='summary-quote'`,
		`INSERT INTO pricing_renewal_quotes SELECT * FROM pricing_renewal_quotes WHERE id='summary-quote' ON CONFLICT(id) DO NOTHING`,
	} {
		if _, err := f.store.DB().ExecContext(f.ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	f.requireSummary(t, "renewal", true, true, false, false)
	// Every operational state below contributes the same eligible zero. The
	// projection compares that contribution, not the transition's spelling.
	for _, state := range []string{"DEFERRED_HEALTH", "READY_TO_SETTLE", "EXPIRED", "AUTHORIZED"} {
		if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_renewal_quotes SET state=$1 WHERE id='summary-quote'`, state); err != nil {
			t.Fatal(err)
		}
		f.requireSummary(t, "renewal", true, true, false, false)
	}
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='REVIEWED',authorized_at=$1 WHERE id='summary-quote'`, pricingStamp(f.at.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "renewal", true, true, false, false)
	if _, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_renewal_quotes SET state='CANCELLED' WHERE id='summary-quote'`); err != nil {
		t.Fatal("sealed analytics blocked ordinary late renewal cleanup", err)
	}
	f.requireSummary(t, "renewal", true, true, true, false)
	var status string
	if err := f.store.DB().QueryRowContext(f.ctx, `SELECT state FROM pricing_renewal_quotes WHERE id='summary-quote'`).Scan(&status); err != nil || status != "CANCELLED" {
		t.Fatal("source outcome was not retained", status, err)
	}
}

func TestPricingCalibrationLateInsertMarksSealedCohortStale(t *testing.T) {
	t.Parallel()
	f := newPricingSummaryFixture(t)
	f.insertSummary(t, "purchase", true)
	if _, err := f.store.DB().ExecContext(f.ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,
	 price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json,outcome,observed_at)
	 VALUES('late-offer','late-owner','combo','summary-product',100,100,$1,$2,'summary-model',0.5,'{}',1,$1)`,
		pricingStamp(f.at), pricingStamp(f.at.Add(time.Hour))); err != nil {
		t.Fatal("sealed metrics blocked a late source insert", err)
	}
	f.requireSummary(t, "purchase", true, true, true, false)
}
