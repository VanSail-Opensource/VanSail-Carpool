package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyStockProjectionIsIndependentOfLegacyPAYGMode(t *testing.T) {
	s, _, _, principal, _ := usageHistoryFixture(t)
	ctx := context.Background()
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: principal.GroupID, Name: "Hourly", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	limit := 1
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: principal.GroupID, Name: "Shared", UpstreamPresent: true, Visible: true, BillingMode: "payg", PAYGPriceTXBMinor: 100, StockLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO hourly_offers(group_id,enabled,hourly_price_minor,revision,created_at,updated_at) VALUES($1,1,1,1,clock_timestamp(),clock_timestamp())`, principal.GroupID); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.HourlyStockRemaining(ctx)
	if err != nil || remaining[principal.GroupID] != 0 {
		t.Fatal("hourly offer ignored its stock on a PAYG-marked product", remaining, err)
	}
	actor := createTestUser(t, s, 61931)
	if _, err := s.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: principal.GroupID, Enabled: false, HourlyPriceTXBMinor: 1}, 1); err != nil {
		t.Fatal(err)
	}
	remaining, err = s.HourlyStockRemaining(ctx)
	if err != nil || remaining[principal.GroupID] != 0 {
		t.Fatal("disabled offer lost the stock projection the UI explains with", remaining, err)
	}
	products, err := s.ListSquadProducts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range products {
		if product.GroupID == principal.GroupID && product.StockRemaining != nil {
			t.Fatal("hourly projection changed legacy PAYG stock semantics")
		}
	}
}

// Hosted PostgreSQL only: the member catalog and the hourly view must read one
// shared seat count (fixed-term owners plus active hourly owners, distinct users)
// and the same limit arithmetic.
func TestCatalogAndHourlyStockShareOneCountAndLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	group := "shared-seat"
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: group, Name: "Shared seat", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	limit := 3
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: group, UpstreamPresent: true, Visible: true, PriceTXBMinor: 50, StockLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	addHourlyBillingOffer(t, s, createTestUser(t, s, 61932).ID, group, 1)
	combo := saveTestCombo(t, s, "shared-seat-combo", 100, 30, group)
	member := createTestUser(t, s, 519032)
	if _, err := s.AdjustBalance(ctx, member.ID, 1000, "shared-seat", "fixture", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePurchase(ctx, PurchaseInput{UserID: member.ID, ComboID: combo.ID, IdempotencyKey: "shared-seat"}, now); err != nil {
		t.Fatal(err)
	}
	hourlyOwner := createTestUser(t, s, 519033)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'active',$7,$7)`,
		"shared-seat-hourly", hourlyOwner.ID, hourlyOwner.AccountGeneration, group, "shared-seat-key", strings.Repeat("b", 64), now); err != nil {
		t.Fatal(err)
	}
	counts, err := s.squadStockCounts(ctx)
	if err != nil || counts[group] != 2 {
		t.Fatalf("shared seat count = %v, %v; want one entitlement plus one hourly member", counts, err)
	}
	hourly, err := s.HourlyStockRemaining(ctx)
	if err != nil || hourly[group] != 1 {
		t.Fatalf("hourly stock = %v, %v; want max(0, 3-2)", hourly, err)
	}
	products, err := s.ListSquadProducts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, product := range products {
		if product.GroupID != group {
			continue
		}
		found = true
		if product.StockRemaining == nil || *product.StockRemaining != 1 {
			t.Fatalf("member catalog stock = %v, want the shared max(0, 3-2)", product.StockRemaining)
		}
	}
	if !found {
		t.Fatal("member catalog omitted the shared squad")
	}
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: group, UpstreamPresent: true, Visible: true, BillingMode: "payg", PAYGPriceTXBMinor: 100, StockLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	hourly, err = s.HourlyStockRemaining(ctx)
	if err != nil || hourly[group] != 1 {
		t.Fatalf("hourly stock diverged after PAYG edit: %v, %v", hourly, err)
	}
}

// Hosted PostgreSQL only: a full shared seat does not stop an already-active
// hourly member from paying the next hour. Only new activations consult stock.
func TestHourlySettlementPaysNextHourWhenSharedStockIsFull(t *testing.T) {
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	limit := 1
	if _, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: input.GroupID, UpstreamPresent: true, Visible: true, StockLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	e := makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	remaining, err := s.HourlyStockRemaining(ctx)
	if err != nil || remaining[input.GroupID] != 0 {
		t.Fatalf("active enrollment did not fill the shared seat: %v, %v", remaining, err)
	}
	before := hourlyBillingBalance(t, s, input.OwnerID)
	result, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy, hourlyBillingDelivery(t, s, e.ID), time.Now())
	if err != nil || len(result.Settlements) != 1 || result.Enrollments[0].Status != model.HourlyActive {
		t.Fatalf("full stock blocked the next paid hour: %+v %v", result, err)
	}
	charged := result.Settlements[0].ChargedTXBMinor
	if charged <= 0 || charged > 100 {
		t.Fatalf("next-hour charge = %d, want a positive prorated amount within the hourly price", charged)
	}
	if after := hourlyBillingBalance(t, s, input.OwnerID); after != before-charged {
		t.Fatalf("next hour balance = %d, want %d after a %d charge", after, before-charged, charged)
	}
}
