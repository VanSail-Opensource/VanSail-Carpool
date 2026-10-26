package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

// ensurePricingContractTx automatically snapshots the enabled product's next
// renewal basis. A generated quote keeps its original snapshot despite edits.
// Completed charges and the current entitlement are never rewritten.
func ensurePricingContractTx(ctx context.Context, tx *sql.Tx, purchase string, now time.Time) (renewal.Contract, error) {
	var user, product string
	var gross int64
	var reward sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT user_id,combo_id,gross_price_txb_minor,reward_renewal_price_minor FROM purchases WHERE id=? /*pg:FOR UPDATE*/`, purchase).Scan(&user, &product, &gross, &reward); err != nil {
		return renewal.Contract{}, err
	}
	p, err := loadPricingPolicy(ctx, tx, "combo", product, gross)
	if err != nil {
		return renewal.Contract{}, err
	}
	c, err := pricingContractTx(ctx, tx, purchase)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return c, err
	}
	if c.ID != "" {
		var quoted int
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_renewal_quotes WHERE contract_id=? AND state NOT IN ('CANCELLED','EXPIRED'))`, c.ID).Scan(&quoted); err != nil {
			return c, err
		}
		if quoted != 0 && c.PolicyVersion == p.Version {
			return c, nil
		}
	}
	if !p.DynamicEnabled || p.RenewalMode != "DYNAMIC" {
		return c, nil
	}
	addons, err := renewalAddonsTx(ctx, tx, purchase, nil)
	if err != nil {
		return c, err
	}
	if c.ID != "" {
		gross = currentPricingContractGross(c, addons)
	} else {
		paid, e := paidPricingAddonTermsTx(ctx, tx, purchase)
		if e != nil {
			return c, e
		}
		basis := map[string]int64{}
		for _, a := range paid {
			basis[a.ID] = a.Price
		}
		for i := range addons {
			if price, ok := basis[addons[i].GroupID]; ok {
				addons[i].PriceTXBMinor = price
			}
		}
	}
	if reward.Valid {
		gross, addons = reward.Int64, nil
	}
	basis := pricingAddonTerms(addons)
	if c.ID != "" && c.Mode == "DYNAMIC" && c.PolicyVersion == p.Version && pricingAddonTermsMatch(c, addons) && c.MaxChangeBPS == p.MaxIncreaseBPS {
		return c, nil
	}
	id, err := ids.New()
	if err != nil {
		return c, err
	}
	root := c.SourcePurchaseID
	if root == "" {
		root = purchase
	}
	limit := min(pricing.MaxTotalPrice, gross+gross*p.MaxIncreaseBPS/pricing.Scale)
	next := renewal.Contract{ID: id, PurchaseID: purchase, UserID: user, ProductID: product, Mode: "DYNAMIC", Price: gross, Ceiling: limit, MaxChangeBPS: p.MaxIncreaseBPS, PolicyVersion: p.Version, RecordedAt: now, Version: c.Version + 1, SourcePurchaseID: root, Addons: basis}
	return next, insertPricingContractTx(ctx, tx, next)
}

// prepareOwnedPricingRenewal performs bounded preparation before an existing
// commerce transaction. Legacy fixed products take the inexpensive query path.
func (s *Store) prepareOwnedPricingRenewal(ctx context.Context, user, purchase string, now time.Time) error {
	var enabled int
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases p JOIN pricing_product_settings s ON s.product_type='combo' AND s.product_id=p.combo_id WHERE p.id=? AND (p.user_id=? OR ?='') AND s.dynamic_enabled=1 AND s.renewal_pricing_mode='DYNAMIC')`, purchase, user, user).Scan(&enabled); err != nil {
		return err
	}
	if enabled == 0 {
		return nil
	}
	_, err := s.PreparePricingRenewal(ctx, purchase, now)
	return err
}
