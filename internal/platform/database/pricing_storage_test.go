package database

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Hosted PostgreSQL measures real heaps, indexes and TOAST. No local benchmark
// is run; SQLite skips this backend-specific footprint acceptance gate.
func TestPricingPostgresRepresentativeStorageBudget(t *testing.T) {
	s := newTestStore(t)
	if !IsPostgres(s.db) {
		t.Skip("PostgreSQL physical footprint gate")
	}
	ctx := context.Background()
	stats := map[string]any{}
	for i := 0; i < 6; i++ {
		stats[string(rune('a'+i))] = map[string]any{"p10": 12345678, "p50": 456789012, "p90": 987654321, "mean": 456789012, "samples": 30}
	}
	feature, _ := json.Marshal(map[string]any{"nodeDistributions": stats, "perNodeSevenDayBytes": map[string]int64{"a": 123456789, "b": 987654321}, "activeDays30": 29, "totalDistribution": map[string]any{"p10": 12345678, "p50": 456789012, "p90": 987654321, "mean": 456789012, "samples": 30}, "dailyObservedDays": 30, "observedNodeDays": 180, "expectedNodeDays": 180, "renewals30": 3, "sourceSeriesHash": "0123456789012345678901234567890123456789012345678901234567890123"})
	queries := []string{
		`INSERT INTO pricing_features(feature_key,id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) SELECT v,'user:'||v,'user',((v-1)%1500)::text,('2026-01-01'::date+((v-1)/1500)::integer)::text,'2026-01-01T00:00:00.000000000Z',10000,'UNKNOWN',$1,md5(v::text)||md5(v::text) FROM generate_series(1,5000) v`,
		`INSERT INTO pricing_forecast_entities(entity_key,entity_type,entity_id) SELECT v,'user_node','member-'||((v-1)/6)||':node-00000000-0000-0000-0000-'||((v-1)%6) FROM generate_series(1,9000) v`,
		`INSERT INTO pricing_forecast_batches(batch_key,metric,generated_at,target_from,target_until,horizon_seconds,model_version,health_state,calibration_status) SELECT v,'traffic_bytes',to_char('2026-01-01'::timestamp+v*interval '1 second','YYYY-MM-DD"T"HH24:MI:SS".000000000Z"'),'2026-01-02T00:00:00.000000000Z','2026-01-03T00:00:00.000000000Z',86400-v,'empirical-daily-v1','UNKNOWN','UNVALIDATED' FROM generate_series(1,313) v`,
		`INSERT INTO pricing_forecast_values(forecast_id,entity_key,batch_key,feature_key,predicted_p10,predicted_p50,predicted_p90,predicted_mean,data_coverage_bps) SELECT v,1+(v-1)%9000,1+(v-1)/96,1+(v-1)/6,12345678,456789012,987654321,456789012,9666 FROM generate_series(1,30000) v`,
		`INSERT INTO pricing_forecast_actuals(forecast_id,actual,status_code,observed_until_ns,evaluated_at_ns) SELECT v,567890123,0,1767398400000000000,1767484800000000000 FROM generate_series(1,30000) v`,
	}
	for i, q := range queries {
		var err error
		if i == 0 {
			_, err = s.db.ExecContext(ctx, q, string(feature))
		} else {
			_, err = s.db.ExecContext(ctx, q)
		}
		if err != nil {
			t.Fatal(i, err)
		}
	}
	size := func(table string) float64 {
		var value float64
		if err := s.db.QueryRowContext(ctx, `SELECT pg_total_relation_size($1::regclass)`, table).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	// 1,500 users: six per-node leads for 30 days + five total leads for 60 days.
	forecast := (size("pricing_forecast_values")+size("pricing_forecast_actuals"))*720000/30000 + size("pricing_forecast_batches")*720000/96/313 + size("pricing_forecast_entities")
	features := size("pricing_features") * 135000 / 5000
	projected := forecast + features + float64(210<<20) // Other published category budgets plus reserve.
	t.Logf("BAPO PostgreSQL measured projection: forecasts=%.2f MiB; features=%.2f MiB; total including other budgets=%.2f MiB", forecast/(1<<20), features/(1<<20), projected/(1<<20))
	if forecast > 220<<20 || projected > 700<<20 {
		t.Fatalf("pricing storage budget exceeded: forecast %.2f MiB; total %.2f MiB", forecast/(1<<20), projected/(1<<20))
	}
	if len(pricing.Datasets()) < 20 {
		t.Fatal("analytical registry unexpectedly incomplete")
	}
}
