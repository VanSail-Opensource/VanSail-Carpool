package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPricingForecastAdmissionSerializesWithSealingAndPreservesLateSource(t *testing.T) {
	for _, withFeature := range []bool{false, true} {
		t.Run(map[bool]string{false: "forecast", true: "feature-and-forecast"}[withFeature], func(t *testing.T) {
			s := newTestStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			f := pricingForecastFixture(t, s, "original", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
			actual := 70.0
			if err := s.EvaluatePricingForecast(ctx, f.ID, "COMPLETE", &actual, f.TargetUntil, f.TargetUntil); err != nil {
				t.Fatal(err)
			}
			if err := s.SummarizePricingValidation(ctx, f.TargetUntil); err != nil {
				t.Fatal(err)
			}
			gate, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer gate.Rollback()
			if _, err = gate.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('pricing.validation',0))`); err != nil {
				t.Fatal(err)
			}
			late := f
			late.ID, late.GeneratedAt = "late-source", f.GeneratedAt.Add(time.Nanosecond)
			feature := pricingFeatureSnapshot{ID: "late-feature", Kind: "node", Entity: "late-node", Day: f.GeneratedAt.Format(time.DateOnly), At: f.Watermark, Coverage: 10000, Health: "HEALTHY", Raw: "{}", Hash: "late-hash"}
			if withFeature {
				late.EntityID, late.SnapshotHash = feature.Entity, feature.Hash
			}
			done := make(chan error, 1)
			go func() {
				if withFeature {
					done <- s.savePricingFeatureForecasts(ctx, feature, []pricing.Forecast{late})
				} else {
					done <- s.SavePricingForecast(ctx, late)
				}
			}()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND wait_event='advisory' AND strpos(query,'pg_advisory_xact_lock_shared')>0 AND strpos(query,'pricing.validation')>0)`).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err := <-done:
					t.Fatal("forecast source bypassed sealing gate", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-ticker.C:
				}
			}
			if withFeature {
				if _, err = gate.ExecContext(ctx, `SET LOCAL lock_timeout='250ms'`); err != nil {
					t.Fatal(err)
				}
				if _, err = gate.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,'node',$2,$3,$4,10000,'HEALTHY','{}',$5)`, feature.ID, feature.Entity, feature.Day, pricingStamp(feature.At), feature.Hash); err != nil {
					t.Fatal("source writer locked feature before the validation gate", err)
				}
			}
			if _, err = gate.ExecContext(ctx, `UPDATE pricing_model_validation_daily SET sealed=true`); err != nil {
				t.Fatal(err)
			}
			if err = gate.Commit(); err != nil {
				t.Fatal(err)
			}
			if err = <-done; err != nil {
				t.Fatal(err)
			}
			var samples int
			var sealed, stale, dirty bool
			if err = s.db.QueryRowContext(ctx, `SELECT samples,sealed,stale,dirty FROM pricing_model_validation_daily`).Scan(&samples, &sealed, &stale, &dirty); err != nil || samples != 1 || !sealed || !stale || dirty {
				t.Fatal("late source did not preserve and mark the sealed cohort", samples, sealed, stale, dirty, err)
			}
			now := f.TargetUntil.AddDate(0, 0, 400)
			counts, err := s.PricingCleanup(ctx, now)
			if err != nil || counts["forecasts"] != 1 || counts["summaries"] != 0 {
				t.Fatal("late unevaluated source disappeared behind a sealed cohort", counts, err)
			}
			if err = s.EvaluatePricingForecast(ctx, late.ID, "MISSING", nil, late.TargetUntil, now); err != nil {
				t.Fatal(err)
			}
			counts, err = s.PricingCleanup(ctx, now)
			if err != nil || counts["forecasts"] != 1 || counts["summaries"] != 1 {
				t.Fatal("observed late source did not finish bounded retention", counts, err)
			}
		})
	}
}

func TestPricingForecastReplayDoesNotMarkSealedCohortStale(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	f := pricingForecastFixture(t, s, "replay", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := s.EvaluatePricingForecast(ctx, f.ID, "MISSING", nil, f.TargetUntil, f.TargetUntil); err != nil {
		t.Fatal(err)
	}
	if err := s.SummarizePricingValidation(ctx, f.TargetUntil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE pricing_model_validation_daily SET sealed=true`); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePricingForecast(ctx, f); err != nil {
		t.Fatal(err)
	}
	var stale bool
	if err := s.db.QueryRowContext(ctx, `SELECT stale FROM pricing_model_validation_daily`).Scan(&stale); err != nil || stale {
		t.Fatal("conflict-ignored source replay changed retained evidence", stale, err)
	}
}
