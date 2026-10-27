package database

import (
	"context"
	"testing"
	"time"
)

func TestPricingSummaryUsesProviderPaymentRecordingTime(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := store.db.ExecContext(ctx, `INSERT INTO pricing_cost_versions(node_id,version,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,recorded_at,actor_id,reason,actual_paid_minor) VALUES('node',1,1000,30,100000,?,?,'admin','fixture',777)`, pricingStamp(now.AddDate(0, 0, -90)), pricingStamp(now.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := store.PricingSummary(ctx, now)
	if err != nil || summary["recordedCostMinor"] != "777" || summary["providerPaymentRecords"] != float64(1) {
		t.Fatalf("recorded payment missing from summary: %#v, %v", summary, err)
	}
}
