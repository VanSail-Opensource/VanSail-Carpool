package database

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPricingSummaryPreservesSignedAggregateRange(t *testing.T) {
	t.Parallel()
	for count, amounts := range map[int][3]string{
		1: {"-9223372036854775807", "9223372036854775807", "-18446744073709551614"},
		2: {"-18446744073709551614", "18446744073709551614", "-36893488147419103228"},
	} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			t.Parallel()
			store, identity, _ := xnodeTrafficFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			// Retained rows use the full signed database amount range. Their
			// aggregate and net margin need not fit one individual wallet row.
			for version := 1; version <= count; version++ {
				id := fmt.Sprintf("summary-refund-%d", version)
				if _, err := store.db.ExecContext(ctx, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,note,created_at) VALUES($1,$2,$3,0,'member_refund_credit',$1,'retained refund',$4)`, id, identity.UserID, int64(math.MaxInt64), stamp(now.Add(-time.Hour))); err != nil {
					t.Fatal(err)
				}
				if _, err := store.db.ExecContext(ctx, `INSERT INTO pricing_cost_versions(node_id,version,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,recorded_at,actor_id,reason,actual_paid_minor) VALUES('node',$1,0,1,1,$2,$2,'retained','retained payment',$3)`, version, pricingStamp(now.Add(-time.Hour)), int64(math.MaxInt64)); err != nil {
					t.Fatal(err)
				}
			}
			summary, err := store.PricingSummary(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			for field, want := range map[string]string{"netRevenueMinor": amounts[0], "recordedCostMinor": amounts[1], "recordedMarginMinor": amounts[2]} {
				if summary[field] != want {
					t.Fatalf("%s = %v; want %s", field, summary[field], want)
				}
			}
		})
	}
}

func TestPricingSummaryKeepsFiniteForecastAmountsBeyondInt64(t *testing.T) {
	t.Parallel()
	store, _, _ := xnodeTrafficFixture(t)
	now := time.Now().UTC()
	seedPricingSummaryEstimate(t, store, now, float64(xnode.PAYGGB)*(1<<24), float64(1<<65))
	summary, err := store.PricingSummary(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"forecastRevenueMinor": "36893488147419103232", "forecastMarginMinor": "20116272147419103232", "unallocatedDailyCostMinor": "16777216000000000000"}
	for field, want := range expected {
		if summary[field] != want {
			t.Fatalf("%s = %v; want %s", field, summary[field], want)
		}
	}
	if summary["recordedCostMinor"] != nil || summary["recordedMarginMinor"] != nil {
		t.Fatalf("forecast invented a provider payment: %#v", summary)
	}
}

func TestPricingSummaryRejectsOverflowingCostEstimate(t *testing.T) {
	t.Parallel()
	store, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	seedPricingSummaryEstimate(t, store, now, math.MaxFloat64, 100)
	if summary, err := store.PricingSummary(ctx, now); !errors.Is(err, ErrPricingSummaryNonFinite) || summary != nil {
		t.Fatalf("nonfinite cost produced a summary: %#v, %v", summary, err)
	}
	if overhead, err := store.pricingUnallocatedOverhead(ctx, now); !errors.Is(err, ErrPricingSummaryNonFinite) || overhead != "" {
		t.Fatalf("nonfinite overhead produced money: %q, %v", overhead, err)
	}
}

func TestPricingSummaryEstimateRejectsNonFiniteValues(t *testing.T) {
	t.Parallel()
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if amount, err := pricingSummaryEstimate(value); !errors.Is(err, ErrPricingSummaryNonFinite) || amount != "" {
			t.Fatalf("nonfinite estimate produced money: %q, %v", amount, err)
		}
	}
	for value, want := range map[float64]string{-0.25: "0", -0.5: "-1", 0.5: "1"} {
		if amount, err := pricingSummaryEstimate(value); err != nil || amount != want {
			t.Fatalf("rounded estimate = %q, %v; want %s", amount, err, want)
		}
	}
}

func seedPricingSummaryEstimate(t *testing.T, store *Store, now time.Time, demand, revenue float64) {
	t.Helper()
	ctx := context.Background()
	actor := createTestAdmin(t, store, 98914)
	if err := store.SavePricingCost(ctx, actor.ID, PricingCost{
		NodeID: "node", CycleDays: 1, Capacity: 1, EffectiveAt: now.Add(-time.Hour),
		Reason: "summary estimate fixture", ChargeMode: "hourly", TrafficRate: pricing.MaxPrice * 10000,
	}, now); err != nil {
		t.Fatal(err)
	}
	day := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	for _, source := range []pricing.Forecast{
		{EntityType: "node", EntityID: "node", Metric: "traffic_bytes", ModelVersion: "empirical-daily-v1", Mean: demand},
		{EntityType: "product", EntityID: "combo:summary", Metric: "gross_revenue_minor", ModelVersion: "empirical-receipts-v1", Mean: revenue},
	} {
		id := "summary-" + source.EntityType
		if _, err := store.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES($1,$2,$3,$4,$5,10000,'HEALTHY','{}',$1)`, id, source.EntityType, source.EntityID, now.Format(time.DateOnly), pricingStamp(now)); err != nil {
			t.Fatal(err)
		}
		source.ID, source.GeneratedAt, source.TargetFrom, source.TargetUntil = id, now, day, day.Add(24*time.Hour)
		source.HorizonSeconds, source.P10, source.P50, source.P90 = 86400, source.Mean, source.Mean, source.Mean
		source.Watermark, source.SnapshotHash, source.CoverageBPS = now, id, 10000
		source.HealthState, source.Calibration = "HEALTHY", "UNVALIDATED"
		if err := store.SavePricingForecast(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
}
