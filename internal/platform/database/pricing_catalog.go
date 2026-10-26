package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PriceCatalog overlays future offers only; administration keeps fixed catalog fields.
func (s *Store) PriceCatalog(ctx context.Context, combos []model.Combo, products []model.SquadProduct, now time.Time) ([]model.Combo, []model.SquadProduct, error) {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	var active int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_product_settings WHERE dynamic_enabled=1 AND purchase_pricing_mode='DYNAMIC') OR EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='PAUSE_ADMISSIONS' AND status='ACTIVE' AND expires_at>?)`, pricingStamp(now)).Scan(&active); err != nil {
		return nil, nil, err
	}
	if active == 0 {
		return combos, products, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	var paused int
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='PAUSE_ADMISSIONS' AND status='ACTIVE' AND starts_at<=? AND expires_at>?)`, pricingStamp(now), pricingStamp(now)).Scan(&paused); err != nil {
		return nil, nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_type||':'||product_id FROM pricing_product_settings WHERE dynamic_enabled=1 AND purchase_pricing_mode='DYNAMIC'`)
	if err != nil {
		return nil, nil, err
	}
	dynamic := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		dynamic[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	out := make([]model.Combo, 0, len(combos))
	for _, c := range combos {
		if paused == 0 && !dynamic["combo:"+c.ID] {
			out = append(out, c)
			continue
		}
		c.PriceTXBMinor, err = resolvePricingTx(ctx, tx, "combo", c.ID, c.PriceTXBMinor, now)
		var code pricing.Code
		if errors.As(err, &code) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		c.Price = model.TXBMoney(c.PriceTXBMinor)
		out = append(out, c)
	}
	squads := make([]model.SquadProduct, 0, len(products))
	for _, p := range products {
		kind, reference := "addon", p.PriceTXBMinor
		if p.BillingMode == "payg" {
			kind, reference = "payg", p.PAYGPriceTXBMinor
		}
		if paused == 0 && !dynamic[kind+":"+p.GroupID] {
			squads = append(squads, p)
			continue
		}
		price, e := resolvePricingTx(ctx, tx, kind, p.GroupID, reference, now)
		var code pricing.Code
		if errors.As(e, &code) {
			if kind == "payg" {
				p.PAYGState = "unavailable"
				squads = append(squads, p)
			}
			continue
		}
		if e != nil {
			return nil, nil, e
		}
		if kind == "payg" {
			p.PAYGPriceTXBMinor = price
			m := model.TXBMoney(price)
			p.PAYGPrice = &m
		} else {
			p.PriceTXBMinor = price
			p.Price = model.TXBMoney(price)
		}
		squads = append(squads, p)
	}
	return out, squads, nil
}

// PricePAYGSquads reuses the same public offer resolver for member PAYG views.
func (s *Store) PricePAYGSquads(ctx context.Context, p []model.SquadProduct, now time.Time) ([]model.SquadProduct, error) {
	_, out, err := s.PriceCatalog(ctx, nil, p, now)
	return out, err
}
