package database

import (
	"context"
	"testing"

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
