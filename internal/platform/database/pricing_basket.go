package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
func pricingBasketTx(ctx context.Context, tx *sql.Tx, in *personalPriceInput, combo string, squads []string, now time.Time) error {
	if in.Kind == "payg" {
		return nil
	}
	full, _ := ctx.Value(pricingBasketKey{}).(bool)
	components := map[string]int64{in.Kind + ":" + in.Product: in.Public}
	in.CostsComplete = true
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
			in.BasketOther += price
		}
		var raw string
		var err error
		if batch, ok := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch); ok {
			var found bool
			raw, found = batch.Economics[key]
			if !found {
				err = sql.ErrNoRows
			}
		} else {
			err = tx.QueryRowContext(ctx, `SELECT d.trace_json FROM pricing_decisions d JOIN pricing_product_settings p ON p.product_type=d.product_type AND p.product_id=d.product_id AND p.version=d.policy_version WHERE d.product_type||':'||d.product_id=? AND d.state='PUBLISHED' AND d.generated_at>=? ORDER BY d.generated_at DESC,d.id DESC LIMIT 1`, key, pricingStamp(now.Add(-24*time.Hour))).Scan(&raw)
		}
		if errors.Is(err, sql.ErrNoRows) {
			in.CostsComplete = false
			continue
		}
		if err != nil {
			return err
		}
		var economics struct {
			Cost  float64 `json:"dailyCostMinor"`
			Units float64 `json:"salesUnits"`
		}
		if err = json.Unmarshal([]byte(raw), &economics); err != nil {
			return err
		}
		if economics.Units <= 0 || economics.Cost < 0 {
			in.CostsComplete = false
		} else {
			in.BasketCost += economics.Cost / economics.Units
		}
	}
	if in.BasketOther > pricing.MaxPrice-in.Public {
		in.CostsComplete = false
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

func personalBasketDiscount(in personalPriceInput, price int64) (int64, error) {
	return pricingCouponDiscount(in.Coupons, price+in.BasketOther)
}
