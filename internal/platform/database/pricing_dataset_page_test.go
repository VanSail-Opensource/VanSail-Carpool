package database

import (
	"context"
	"errors"
	"fmt"
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

func TestPricingDatasetCursorAndScopeKeepExactTimestamps(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	user := createTestUser(t, s, 49554)
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	stamps := []string{stamp(now), stamp(now.Add(time.Nanosecond)), stamp(now.Add(time.Second / 10)), pricingStamp(now.Add(time.Second / 10)), stamp(now.Add(time.Second))}
	for i, at := range stamps {
		id := fmt.Sprintf("time-%d", i)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at) VALUES($1,$2,1,1,'fixture',$1,$3)`, id, user.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	scope := PricingExportScope{Format: "jsonl", IncludeUsers: true, IncludeFinancial: true, Users: []string{user.ID}, From: now, Until: now.Add(time.Second)}
	cursor := ""
	for _, id := range []string{"time-3", "time-2", "time-1", "time-0"} {
		page, err := s.ReadPricingDatasetPage(ctx, "raw/wallet_events", scope, 1, cursor)
		if err != nil || len(page.Items) != 1 || page.Items[0]["id"] != id {
			t.Fatal("chronological keyset changed equal-instant precision or window bounds", id, page, err)
		}
		cursor = page.NextCursor
	}
	if cursor != "" {
		t.Fatal("exclusive upper boundary was included")
	}
	scope.From, scope.Until = now.Add(time.Nanosecond), now.Add(time.Second/10)
	page, err := s.ReadPricingDatasetPage(ctx, "raw/wallet_events", scope, 100, "")
	if err != nil || len(page.Items) != 1 || page.Items[0]["id"] != "time-1" {
		t.Fatal("fractional bounds changed retained evidence", page, err)
	}
}

func TestPricingRelatedExportScopeKeepsProductKinds(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), pricingStamp(time.Now().UTC())
	for _, kind := range []string{"combo", "addon", "payg"} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_product_settings(product_type,product_id,fixed_price_minor,updated_by,updated_at) VALUES($1,'shared',100,'fixture',$2)`, kind, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,model_version,predicted_probability,features_json) VALUES('related','member','combo','shared',100,100,$1,$1,'fixture',0.5,'{}')`, now); err != nil {
		t.Fatal(err)
	}
	scope := PricingExportScope{Format: "jsonl", IncludeUsers: true, IncludeFinancial: true, Users: []string{"member"}}
	page, err := s.ReadPricingDatasetPage(ctx, "pricing/product_settings", scope, 100, "")
	if err != nil || len(page.Items) != 1 || page.Items[0]["product_type"] != "combo" {
		t.Fatal("user offer relation crossed product kinds", page, err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_groups(id,name,enabled,created_at,updated_at) VALUES('shared','Shared ID',1,$1,$1)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_group_bindings(group_id,node_id,inbound_tag) VALUES('shared','node','fixture')`); err != nil {
		t.Fatal(err)
	}
	scope.Users, scope.Nodes = nil, []string{"node"}
	page, err = s.ReadPricingDatasetPage(ctx, "pricing/product_settings", scope, 100, "")
	if err != nil || len(page.Items) != 2 || page.Items[0]["product_type"] == "combo" || page.Items[1]["product_type"] == "combo" {
		t.Fatal("node group relation included an unrelated combo", page, err)
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
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_personal_decisions(version,user_id,product_type,product_id,context_key,policy_version,public_minor,personal_minor,net_minor,status,reason,model_version,calibration,generated_at,valid_until,coupon_json) VALUES($1,$2,'combo','c','scope',1,100,100,100,'UNCHANGED','FIXED_PRICE','fixed-v1','NOT_APPLICABLE',$3,$4,'[]')`, version, user.ID, pricingStamp(now), pricingStamp(now.Add(time.Hour))); err != nil {
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
		if _, err := s.db.ExecContext(ctx, `INSERT INTO pricing_decisions(id,product_type,product_id,generated_at,policy_version,model_version,reference_minor,proposed_minor,selected_minor,state,trace_json) VALUES($1,$2,$3, $4,1,'fixture',100,100,100,'SHADOW','{}')`, kind, kind, "shared", now); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ReadPricingDatasetPage(ctx, "pricing/pricing_decisions", PricingExportScope{Format: "jsonl", IncludeUsers: true, Products: []string{"combo:shared"}}, 100, "")
	if err != nil || len(page.Items) != 1 || page.Items[0]["product_type"] != "combo" {
		t.Fatal(page, err)
	}
}
