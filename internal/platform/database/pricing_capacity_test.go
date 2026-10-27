package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPricingCapacityUsesGiBAndExactForecastHorizon(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	group := saveTestSquad(t, s, "capacity", 100, true)
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: group.GroupID, Name: "Capacity", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "in"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_payg_accounts(proxy_user_id,user_id,group_id,node_id,period_id,unit_price_minor) VALUES(?,?,?,'node',?,100)`, identity.ProxyUserID, identity.UserID, group.GroupID, period.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes) VALUES(?,'node',?,?,?,0)`, identity.ProxyUserID, period.ID, now.AddDate(0, 0, -1).Format(time.DateOnly), xnode.PAYGGB); err != nil {
		t.Fatal(err)
	}
	stamp := pricingStamp(now)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_cost_metadata(node_id,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,updated_by,updated_at) VALUES('node',3000,30,?,?, 'fixture',?)`, 300*xnode.PAYGGB, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	base := pricingForecastFixture(t, s, "capacity-source", now)
	day := now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	for i, model := range []string{"empirical-daily-v1", "wrong-model", "empirical-daily-v1", "empirical-daily-v1"} {
		f := base
		f.ID = []string{"correct", "wrong-model", "wrong-lead", "stale"}[i]
		f.ModelVersion = model
		f.TargetFrom = day
		f.TargetUntil = day.Add(24 * time.Hour)
		f.P90 = 8 * float64(xnode.PAYGGB)
		f.P10 = f.P90 / 2
		f.P50 = f.P90 * .75
		f.Mean = f.P50
		if i == 1 {
			f.P90 = 100 * float64(xnode.PAYGGB)
		}
		if i == 2 {
			f.HorizonSeconds = 172800
			f.GeneratedAt = now.Add(-24 * time.Hour)
			f.Watermark = base.Watermark
		}
		if i == 3 {
			f.GeneratedAt = now.Add(-72 * time.Hour)
		}
		if i >= 2 {
			f.Watermark = f.GeneratedAt
			f.SnapshotHash = f.ID
			if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_features(id,entity_type,entity_id,day,watermark,coverage_bps,health_state,features_json,snapshot_hash) VALUES(?,'node','node',?,?,10000,'HEALTHY','{}',?)`, f.ID, f.GeneratedAt.Format(time.DateOnly), pricingStamp(f.Watermark), f.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.SavePricingForecast(ctx, f); err != nil {
			t.Fatal(f.ID, err)
		}
	}
	read := func() (pricing.OptimizationInputs, error) {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return pricing.OptimizationInputs{}, err
		}
		defer tx.Rollback()
		return pricingOptimizationInputsTx(ctx, tx, pricing.DefaultPolicy("payg", group.GroupID, 100), pricing.ResponseModel{}, now)
	}
	got, err := read()
	if err != nil || got.SalesUnits != 8 || got.CapacityGB != 10 || got.DemandGB != 8 || got.CostMinor != 100 {
		t.Fatal(got, err)
	}
	// Prepaid traffic is already in forecast consumption; reserve the larger amount.
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=?,credit_started_at=?`, 6*xnode.PAYGGB, stamp); err != nil {
		t.Fatal(err)
	}
	got, err = read()
	if err != nil || got.CommittedGB != 2 || got.DemandGB != 6 || got.SalesUnits != 6 {
		t.Fatal(got, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE pricing_cost_metadata SET capacity_bytes=?`, 30*xnode.PAYGGB); err != nil {
		t.Fatal(err)
	}
	if _, err = read(); !errors.Is(err, pricing.CapacityUnsafe) {
		t.Fatal("unsafe fulfilling node accepted", err)
	}
}
