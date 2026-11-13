package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPricingSummaryUsesProviderPaymentRecordingTime(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := store.db.ExecContext(ctx, `INSERT INTO pricing_cost_versions(node_id,version,cycle_cost_minor,cycle_days,capacity_bytes,effective_at,recorded_at,actor_id,reason,actual_paid_minor) VALUES('node',1,1000,30,100000,$1,$2,'admin','fixture',777)`, pricingStamp(now.AddDate(0, 0, -90)), pricingStamp(now.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := store.PricingSummary(ctx, now)
	if err != nil || summary["recordedCostMinor"] != "777" || summary["providerPaymentRecords"] != float64(1) {
		t.Fatalf("recorded payment missing from summary: %#v, %v", summary, err)
	}
}

func TestPricingSummaryExcludesInterventionsAndDiscountsBeyondAsOf(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, at := range []time.Time{now.Add(-time.Nanosecond), now, now.Add(time.Nanosecond)} {
		id := fmt.Sprintf("as-of-%d", i)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_admin_overrides(id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at) VALUES($1,'global','','DISCOUNT','fixture','fixture',$2,$3,1,'{}','{}','ACTIVE',$2)`, id, pricingStamp(at), pricingStamp(at.Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_discount_spend(override_id,reference_id,user_id,discount_minor,created_at,status) VALUES($1,$1,$2,100,$3,'CONSUMED')`, id, identity.UserID, pricingStamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := s.pricingSummary(ctx, now)
	if err != nil || summary["interventions30"] != 1 || summary["manualDiscountMinor30"] != "100" {
		t.Fatal("summary leaked actions beyond its as-of boundary", summary, err)
	}
}
