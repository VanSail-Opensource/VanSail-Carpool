package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"testing"
	"time"
)

func TestPricingDatasetCursorHasStableTiesAndScopeBinding(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	actor := pricingTestAdmin(t, s)
	for version := int64(0); version < 3; version++ {
		if err := s.SavePricingCost(ctx, actor.ID, PricingCost{NodeID: "node", Cost: 1000, Capacity: 1000000, CycleDays: 30, EffectiveAt: now, Version: version, Reason: "cursor fixture"}, now); err != nil {
			t.Fatal(err)
		}
	}
	scope := PricingExportScope{Format: "jsonl", IncludeFinancial: true, IncludeUsers: true}
	first, err := s.ReadPricingDatasetPage(ctx, "pricing/provider_cost_versions", scope, 2, "")
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := s.ReadPricingDatasetPage(ctx, "pricing/provider_cost_versions", scope, 2, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0]["version"] != "1" || second.NextCursor != "" {
		t.Fatal(second, err)
	}
	scope.Nodes = []string{"another-node"}
	if _, err = s.ReadPricingDatasetPage(ctx, "pricing/provider_cost_versions", scope, 2, first.NextCursor); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestEveryPricingDatasetHasUsablePageKeys(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, dataset := range pricing.Datasets() {
		if _, err := s.ReadPricingDatasetPage(ctx, dataset.Name, PricingExportScope{Format: "jsonl", IncludeUsers: true, IncludeFinancial: true}, 2, ""); err != nil {
			t.Fatal(dataset.Name, err)
		}
	}
}

func TestPersonalDecisionCursorKeepsOpaqueVersions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 49553)
	now := time.Now().UTC()
	for _, version := range []string{"hash-a", "hash-b"} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_personal_decisions(version,user_id,product_type,product_id,context_key,policy_version,public_minor,personal_minor,net_minor,status,reason,model_version,calibration,generated_at,valid_until,coupon_json) VALUES(?,?,'combo','c','scope',1,100,100,100,'UNCHANGED','FIXED_PRICE','fixed-v1','NOT_APPLICABLE',?,?,'[]')`, version, user.ID, pricingStamp(now), pricingStamp(now.Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	scope := PricingExportScope{Format: "jsonl", IncludeUsers: true, IncludeFinancial: true}
	first, err := s.ReadPricingDatasetPage(ctx, "pricing/personal_decisions", scope, 1, "")
	if err != nil || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := s.ReadPricingDatasetPage(ctx, "pricing/personal_decisions", scope, 1, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0]["version"] != "hash-a" {
		t.Fatal(second, err)
	}
}
func TestPricingTypedProductScopeDoesNotMixEqualIdentifiers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := pricingStamp(time.Now().UTC())
	for _, kind := range []string{"combo", "addon"} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_decisions(id,product_type,product_id,generated_at,policy_version,model_version,reference_minor,proposed_minor,selected_minor,state,trace_json) VALUES(?,?,?, ?,1,'fixture',100,100,100,'SHADOW','{}')`, kind, kind, "shared", now); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ReadPricingDatasetPage(ctx, "pricing/pricing_decisions", PricingExportScope{Format: "jsonl", IncludeUsers: true, Products: []string{"combo:shared"}}, 100, "")
	if err != nil || len(page.Items) != 1 || page.Items[0]["product_type"] != "combo" {
		t.Fatal(page, err)
	}
}
