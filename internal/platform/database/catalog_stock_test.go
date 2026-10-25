package database

import (
	"context"
	"testing"
	"time"
)

func TestCatalogStockUsesEntitlementSnapshots(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	combo := saveTestCombo(t, s, "stock-snapshots", 100, 30)
	user := createTestUser(t, s, 519031)
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "stock", "seed", now); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "stock"}, now)
	if err != nil {
		t.Fatal(err)
	}
	limit := 4
	for _, group := range []string{"snapshot", "live", "addon-snapshot", "old-addon"} {
		if _, err = s.SaveSquadProduct(ctx, SquadProductInput{GroupID: group, UpstreamPresent: true, Visible: true, StockLimit: &limit}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE combos SET included_squad_uuids='["live"]' WHERE id=?`, combo.ID); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		full, combo, addons any
		group               string
	}{
		{`["snapshot"]`, `["live"]`, `["live"]`, "snapshot"},
		{nil, `["snapshot"]`, `["addon-snapshot"]`, "snapshot"},
		{nil, nil, nil, "live"},
	} {
		if _, err = s.db.ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids=?,combo_squad_uuids=?,entitlement_addon_squad_uuids=? WHERE id=?`, item.full, item.combo, item.addons, p.ID); err != nil {
			t.Fatal(err)
		}
		counts, err := s.squadStockCounts(ctx)
		if err != nil || counts[item.group] != 1 {
			t.Fatal("snapshot reservation differs", counts, err)
		}
		owned, err := s.purchaseSquads(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range owned {
			if counts[group] != 1 {
				t.Fatal("stock differs from entitlement", owned, counts)
			}
		}
	}
	raw, err := s.ListSquadProductOverrides(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range raw {
		if product.StockRemaining != nil {
			t.Fatal("raw list calculated stock")
		}
	}
	products, err := s.ListSquadProducts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range products {
		if product.GroupID == "live" && (product.StockRemaining == nil || *product.StockRemaining != 3) {
			t.Fatal("remaining stock missing")
		}
	}
	if IsPostgres(s.db) {
		explainStockPlan(t, s, p.ID)
	}
}
