package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PricingUserPricePage uses explicit pagination and the same versioned numerical
// decisions as member catalog and checkout. No wallet balance participates.
func (s *Store) PricingUserPricePage(ctx context.Context, user, cursor string, now time.Time) (model.UserPricePage, error) {
	out := model.UserPricePage{Items: []model.UserProductPrice{}}
	if _, err := s.UserByID(ctx, user); err != nil {
		return out, err
	}
	products, next, err := s.PricingProducts(ctx, cursor)
	if err != nil {
		return out, err
	}
	out.NextCursor = next
	var current string
	err = s.db.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE user_id=? AND status IN ('active','activating') AND valid_until>? ORDER BY valid_until DESC LIMIT 1`, user, pricingStamp(now)).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	renewal, err := s.preparePersonalRenewalDecision(ctx, user, now)
	if err != nil {
		return out, err
	}
	for _, product := range products {
		kind, id := product["productType"].(string), product["productId"].(string)
		combo, squads := current, []string{id}
		if kind == "combo" {
			c, e := s.ComboByID(ctx, id, false)
			if e != nil {
				return out, e
			}
			combo = id
			squads = []string{}
			for _, g := range c.IncludedSquads {
				squads = append(squads, g.GroupID)
			}
		}
		d, e := s.PreparePersonalPricing(ctx, user, kind, id, combo, squads, now)
		if e != nil {
			return out, e
		}
		tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if e != nil {
			return out, e
		}
		fixed, e := pricingReference(ctx, tx, kind, id)
		if e != nil {
			tx.Rollback()
			return out, e
		}
		in, e := personalPriceSnapshotTx(ctx, tx, user, kind, id, combo, squads, now)
		if e != nil {
			tx.Rollback()
			return out, e
		}
		if in.Version != d.Version {
			d.Status = "STALE"
			d.Reason = "VERSION_CHANGED"
		}
		personal := moneyMinor(d.PersonalPrice)
		campaign := personal
		if len(in.Coupons) == 0 {
			campaign, e = personalPricingTx(ctx, tx, user, kind, id, personal, 1, "", false, now)
		}
		tx.Rollback()
		if e != nil {
			return out, e
		}
		rate := model.UserProductPrice{ProductType: kind, ProductID: id, Name: product["name"].(string), BillingMode: "combo", FixedPrice: model.TXBMoney(fixed), PublicPrice: d.PublicPrice, PersonalPrice: d.PersonalPrice, CampaignPrice: model.TXBMoney(campaign), NetPrice: d.NetPrice, Coupons: in.Coupons, Decision: d}
		if len(in.Coupons) == 0 {
			rate.NetPrice = rate.CampaignPrice
		}
		if kind == "payg" {
			rate.BillingMode = "payg"
		}
		if kind == "combo" && id == current {
			rate.RenewalDecision = renewal
		}
		out.Items = append(out.Items, rate)
	}
	return out, nil
}
