package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestProviderCostModelsPreserveHistoryAndLegacy(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	actor := pricingTestAdmin(t, s)
	var legacy PricingCost
	if err := json.Unmarshal([]byte(`{"nodeId":"node","cycleCostMinor":"1200","cycleDays":30,"capacityBytes":"1000","effectiveAt":"2026-01-01T00:00:00Z","version":0,"reason":"legacy"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.EffectiveAt = now
	if err := s.SavePricingCost(ctx, actor.ID, legacy, now); err != nil {
		t.Fatal(err)
	}
	paid := int64(777)
	next := PricingCost{NodeID: "node", CycleDays: 30, Capacity: 0, EffectiveAt: now, Version: 1, Reason: "hourly", ChargeMode: "hourly", HourlyRate: 1234, TrafficRate: 250000, Unlimited: true, ActualPaid: &paid}
	if err := s.SavePricingCost(ctx, actor.ID, next, now); err != nil {
		t.Fatal(err)
	}
	var mode string
	var hourly, traffic, unlimited, actual int64
	if err := s.db.QueryRowContext(ctx, `SELECT charge_mode,hourly_rate_micros,traffic_rate_micros,capacity_unlimited,actual_paid_minor FROM pricing_cost_versions WHERE node_id='node' AND version=2`).Scan(&mode, &hourly, &traffic, &unlimited, &actual); err != nil {
		t.Fatal(err)
	}
	if mode != "hourly" || hourly != 1234 || traffic != 250000 || unlimited != 1 || actual != paid {
		t.Fatal(mode, hourly, traffic, unlimited, actual)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT charge_mode,hourly_rate_micros,traffic_rate_micros,capacity_unlimited FROM pricing_cost_versions WHERE node_id='node' AND version=1`).Scan(&mode, &hourly, &traffic, &unlimited); err != nil {
		t.Fatal(err)
	}
	if mode != "cycle" || hourly != 0 || traffic != 0 || unlimited != 0 {
		t.Fatal(mode, hourly, traffic, unlimited)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE pricing_cost_versions SET hourly_rate_micros=5 WHERE node_id='node'`); err == nil {
		t.Fatal("historical versions must be immutable")
	}
	next.Version = 2
	next.ActualPaid = nil
	next.HourlyRate = 0
	next.TrafficRate = 0
	if err := s.SavePricingCost(ctx, actor.ID, next, now); err != nil {
		t.Fatal("zero rates:", err)
	}
}
