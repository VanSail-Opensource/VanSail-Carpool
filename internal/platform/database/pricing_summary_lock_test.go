package database

import (
	"database/sql"
	"testing"
	"time"
)

func TestPricingCalibrationWriterWaitsBeforeSourceTupleLock(t *testing.T) {
	t.Parallel()
	f := newPricingSummaryFixture(t)
	f.insertOffer(t)
	publisher, err := f.store.DB().BeginTx(f.ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Rollback()
	if _, err := publisher.ExecContext(f.ctx, `SELECT pg_advisory_xact_lock(hashtextextended('pricing.calibration',0))`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.store.DB().ExecContext(f.ctx, `UPDATE pricing_offers SET outcome=1,observed_at=expires_at WHERE id='summary-offer'`)
		done <- err
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := f.store.DB().QueryRowContext(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
		 WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND wait_event='advisory'
		 AND strpos(query,'UPDATE pricing_offers SET outcome=1')>0)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatal("source writer bypassed exclusive summary gate", err)
		case <-f.ctx.Done():
			t.Fatal(f.ctx.Err())
		case <-ticker.C:
		}
	}
	probe, err := f.store.DB().BeginTx(f.ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback()
	if _, err := probe.ExecContext(f.ctx, `SELECT id FROM pricing_offers WHERE id='summary-offer' FOR UPDATE NOWAIT`); err != nil {
		t.Fatal("waiting writer locked source before the shared statement gate", err)
	}
	if err := probe.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Publication occurs after the blocked UPDATE captured its outer statement
	// snapshot. Its trigger must still see and invalidate this committed cohort.
	if _, err := publisher.ExecContext(f.ctx, `INSERT INTO pricing_calibration_daily
	 (day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at,sealed)
	 VALUES($1,'summary-model','purchase',5,10,5,3,2.5,$2,true)`, f.day, pricingStamp(f.at.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	f.requireSummary(t, "purchase", true, true, true, false)
}
