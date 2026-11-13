package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ApplyCatalogPricing decorates public prices with ordinary coupon eligibility.
func (s *Store) ApplyCatalogPricing(ctx context.Context, user string, c model.Catalog, now time.Time) (model.Catalog, error) {
	var err error
	ctx, err = s.pricingReadContext(ctx, user, now)
	if err != nil {
		return c, err
	}
	batch := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch)
	enabled := false
	for _, p := range batch.Policies {
		enabled = enabled || p.DynamicEnabled && p.PurchaseMode == "DYNAMIC"
	}
	if !enabled {
		return c, nil
	}
	for i := range c.Combos {
		p := &c.Combos[i]
		squads := []string{}
		for _, g := range p.IncludedSquads {
			squads = append(squads, g.GroupID)
		}
		d, err := s.PrepareCheckoutPricing(ctx, user, "combo", p.ID, p.ID, squads, now)
		if err != nil {
			return c, err
		}
		p.PricingDecision = &d
		p.PriceTXBMinor = moneyMinor(d.PublicPrice)
		p.Price = model.TXBMoney(p.PriceTXBMinor)
	}
	var current string
	err = s.db.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE user_id=$1 AND status IN ('active','activating') AND `+purchaseTimestampOrderSQL("valid_until")+`>$2 ORDER BY `+purchaseTimestampOrderSQL("valid_until")+` DESC,id LIMIT 1`, user, purchaseTimestampOrder(now)).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return c, err
	}
	for i := range c.Addons {
		p := &c.Addons[i]
		d, e := s.PrepareCheckoutPricing(ctx, user, "addon", p.GroupID, current, []string{p.GroupID}, now)
		if e != nil {
			return c, e
		}
		p.PricingDecision = &d
		p.PriceTXBMinor = moneyMinor(d.PublicPrice)
		p.Price = model.TXBMoney(p.PriceTXBMinor)
	}
	return c, nil
}
