package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPricingExposuresIncludeShownProductsBeyondInventoryPrefilter(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	user := createTestUser(t, store, 32_209)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	for _, query := range []string{
		`INSERT INTO combos(id,name,price_txb_minor,validity_days,traffic_limit_bytes,created_at,updated_at)
		 SELECT 'exposure-'||i,'Shown product '||i,100,30,1000000,$1,$1 FROM generate_series(1,513) i`,
		`INSERT INTO pricing_product_settings(product_type,product_id,dynamic_enabled,purchase_pricing_mode,fixed_price_minor,updated_by,updated_at)
		 SELECT 'combo',id,1,'DYNAMIC',100,'fixture',$1 FROM combos WHERE id LIKE 'exposure-%'`,
	} {
		if _, err := store.DB().ExecContext(ctx, query, pricingStamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	// Every response stays within the 64-item exposure budget. The entire opted-in
	// inventory must not be silently capped before selecting those shown items.
	for offset := 1; offset <= 513; offset += 64 {
		catalog := model.Catalog{}
		for i := offset; i <= min(offset+63, 513); i++ {
			catalog.Combos = append(catalog.Combos, model.Combo{ID: fmt.Sprintf("exposure-%d", i), PriceTXBMinor: 100})
		}
		if err := store.RecordPricingExposures(ctx, user.ID, catalog, now); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordPricingExposures(ctx, user.ID, catalog, now); err != nil {
			t.Fatal("same shown response lost replay", err)
		}
	}
	var count, owners int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COUNT(DISTINCT user_id) FROM pricing_offers`).Scan(&count, &owners); err != nil || count != 513 || owners != 1 {
		t.Fatal("shown inventory was omitted or duplicated", count, owners, err)
	}
}
