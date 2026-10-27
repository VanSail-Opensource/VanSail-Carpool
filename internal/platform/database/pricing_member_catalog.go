package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PersonalizePricingCatalog shows owner decisions without wallet-based eligibility.
func (s *Store) PersonalizePricingCatalog(ctx context.Context, user string, c model.Catalog, now time.Time) (model.Catalog, error) {
	var err error
	ctx, err = s.pricingReadContext(ctx, user, now)
	if err != nil {
		return c, err
	}
	batch := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch)
	enabled := false
	for _, p := range batch.Policies {
		enabled = enabled || p.DynamicEnabled && (p.PurchaseMode == "DYNAMIC" || p.PersonalDiscounts)
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
		d, err := s.PreparePersonalPricing(ctx, user, "combo", p.ID, p.ID, squads, now)
		if err != nil {
			return c, err
		}
		p.PricingDecision = &d
		p.PriceTXBMinor = moneyMinor(d.PersonalPrice)
		p.Price = model.TXBMoney(p.PriceTXBMinor)
	}
	var current string
	err = s.db.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC LIMIT 1`, user, pricingStamp(now)).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return c, err
	}
	for i := range c.Addons {
		p := &c.Addons[i]
		d, e := s.PreparePersonalPricing(ctx, user, "addon", p.GroupID, current, []string{p.GroupID}, now)
		if e != nil {
			return c, e
		}
		p.PricingDecision = &d
		p.PriceTXBMinor = moneyMinor(d.PersonalPrice)
		p.Price = model.TXBMoney(p.PriceTXBMinor)
	}
	grants, err := s.pricingGrantScopes(ctx, now)
	if err != nil {
		return c, err
	}
	if len(grants) > 0 {
		tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return c, e
		}
		defer tx.Rollback()
		for i := range c.Combos {
			p := &c.Combos[i]
			if grants[user+":combo:"+p.ID] && p.PricingDecision.NetPrice.Minor == p.PricingDecision.PersonalPrice.Minor {
				p.PriceTXBMinor, e = personalPricingTx(ctx, tx, user, "combo", p.ID, p.PriceTXBMinor, 1, "", false, now)
				if e != nil {
					return c, e
				}
				p.Price = model.TXBMoney(p.PriceTXBMinor)
			}
		}
	}
	return c, nil
}

func (s *Store) retryAttempt_PricingPAYGPersonalPrice(ctx context.Context, user, group string, public int64, now time.Time) (int64, error) {
	grants, err := s.pricingGrantScopes(ctx, now)
	if err != nil {
		return 0, err
	}
	if !grants[user+":payg:"+group] {
		return public, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	return personalPricingTx(ctx, tx, user, "payg", group, public, 1, "", false, now)
}
