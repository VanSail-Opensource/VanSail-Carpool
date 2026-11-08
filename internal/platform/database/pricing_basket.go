package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"time"
)

type pricingBasketKey struct{}

func purchasePricingContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, pricingBasketKey{}, true)
}

// Full purchases include the Combo; live additions include only new add-ons.
// The coupon is evaluated once against that entire prospective basket.
func pricingBasketTx(ctx context.Context, tx *sql.Tx, in *checkoutPriceInput, combo string, squads []string, now time.Time) error {
	if in.Kind == "payg" {
		return nil
	}
	full, _ := ctx.Value(pricingBasketKey{}).(bool)
	components := map[string]int64{in.Kind + ":" + in.Product: in.Public}
	included := map[string]bool{}
	if in.Kind == "combo" || full {
		c, err := comboByIDTx(ctx, tx, combo, false)
		if err != nil {
			return err
		}
		for _, g := range c.IncludedSquads {
			included[g.GroupID] = true
		}
		if in.Kind != "combo" {
			price, err := resolvePricingTx(ctx, tx, "combo", combo, c.PriceTXBMinor, now)
			if err != nil {
				return err
			}
			components["combo:"+combo] = price
		}
	}
	for _, id := range uniqueSorted(squads) {
		if included[id] || in.Kind == "addon" && id == in.Product {
			continue
		}
		fixed, err := pricingReference(ctx, tx, "addon", id)
		if err != nil {
			return err
		}
		price, err := resolvePricingTx(ctx, tx, "addon", id, fixed, now)
		if err != nil {
			return err
		}
		components["addon:"+id] = price
	}
	for key, price := range components {
		if key != in.Kind+":"+in.Product {
			if price > pricing.MaxTotalPrice-in.Public-in.BasketOther {
				return pricing.VersionConflict
			}
			in.BasketOther += price
		}
	}
	return nil
}
func pricingPurchaseSquads(combo model.Combo, addons []string) []string {
	ids := append([]string{}, addons...)
	for _, g := range combo.IncludedSquads {
		ids = append(ids, g.GroupID)
	}
	return uniqueSorted(ids)
}
