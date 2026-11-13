package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RenewalCombo resolves the combo an owned renewal will provision so catalog
// hydration validates exactly the owned selection the store commits. A hidden
// or inactive combo stays renewable; a missing combo row fails closed with
// ErrNotFound.
func (s *Store) RenewalCombo(ctx context.Context, userID, purchaseID string) (model.Combo, error) {
	var owner, comboID string
	if err := s.db.QueryRowContext(ctx, `SELECT user_id,combo_id FROM purchases WHERE id=$1`, purchaseID).Scan(&owner, &comboID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Combo{}, ErrNotFound
		}
		return model.Combo{}, fmt.Errorf("load renewal combo owner: %w", err)
	}
	if owner != userID {
		return model.Combo{}, ErrNotFound
	}
	combo, err := s.ComboByID(ctx, comboID, false)
	if err != nil {
		return model.Combo{}, err
	}
	return renewalComboTx(ctx, s.db, purchaseID, combo)
}

func renewalQuoteTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, termCount int, excludedAddonIDs []string, now time.Time) (model.RenewalQuote, model.Combo, []model.SquadProduct, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2`, purchaseID, userID))
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	if purchase.Status != "active" && purchase.Status != "activating" && purchase.Status != "queued" {
		return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
	}
	// Owned renewal: a hidden or inactive combo stays renewable, while a missing
	// row fails closed. The purchase's owned squad selection is then applied.
	combo, err := ownedRenewalComboTx(ctx, tx, purchaseID)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	addons, err := renewalAddonsTx(ctx, tx, purchaseID, excludedAddonIDs)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	var customPrice sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT reward_renewal_price_minor FROM purchases WHERE id=$1`, purchaseID).Scan(&customPrice); err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	if customPrice.Valid {
		addons = nil
	}
	if empty, e := emptyRenewalTx(ctx, tx, purchaseID); e != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, e
	} else if empty {
		return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
	}
	// Included squads reserve capacity only on the initial purchase. A renewal
	// rechecks its paid add-ons without displacing the current member.
	selected := make([]string, 0, len(addons))
	for _, squad := range addons {
		selected = append(selected, squad.GroupID)
	}
	if err := checkSquadStockTx(ctx, tx, selected, userID); err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	start, err := nextPurchaseStartTx(ctx, tx, userID, now)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	grossPerTerm := combo.PriceTXBMinor
	if customPrice.Valid {
		grossPerTerm = customPrice.Int64
	}
	addonIDs := make([]string, 0, len(addons))
	for _, addon := range addons {
		grossPerTerm, err = checkedPriceSum(grossPerTerm, addon.PriceTXBMinor)
		if err != nil {
			return model.RenewalQuote{}, model.Combo{}, nil, err
		}
		addonIDs = append(addonIDs, addon.GroupID)
	}
	if !customPrice.Valid {
		grossPerTerm, err = pricingRenewalGrossTx(ctx, tx, purchaseID, grossPerTerm, addons, now)
		if err != nil {
			return model.RenewalQuote{}, model.Combo{}, nil, err
		}
		combo.PriceTXBMinor = grossPerTerm
		for _, addon := range addons {
			if addon.PriceTXBMinor < 0 || combo.PriceTXBMinor < addon.PriceTXBMinor {
				return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
			}
			combo.PriceTXBMinor -= addon.PriceTXBMinor
		}
		if combo.PriceTXBMinor < 0 {
			return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
		}
	}
	discount := coupons.Discount{GrossMinor: grossPerTerm, NetMinor: grossPerTerm}
	if purchase.RecurringDiscountAttached {
		if purchase.CouponGrantID == nil {
			return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
		}
		discount, err = quoteRecurringRenewalCouponTx(ctx, tx, userID, *purchase.CouponGrantID, grossPerTerm)
		if err != nil {
			return model.RenewalQuote{}, model.Combo{}, nil, err
		}
	}
	var couponGrantID *string
	if discount.Recurring && discount.GrantID != "" {
		couponGrantID = &discount.GrantID
	}
	totalPrice, err := checkedPriceTotal(discount.NetMinor, termCount)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	return model.RenewalQuote{PurchaseID: purchaseID, ComboID: combo.ID, TermCount: termCount, GrossPrice: model.TXBMoney(grossPerTerm), Discount: model.TXBMoney(discount.DiscountMinor), PricePerTerm: model.TXBMoney(discount.NetMinor), TotalPrice: model.TXBMoney(totalPrice), CouponGrantID: couponGrantID, EffectiveAt: start,
		ExpiresAt: start.AddDate(0, 0, combo.ValidityDays*termCount), AddonSquadUUIDs: addonIDs}, combo, addons, nil
}
