package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

func renewalQuoteTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, termCount int, excludedAddonIDs []string, now time.Time) (model.RenewalQuote, model.Combo, []model.SquadProduct, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, purchaseID, userID))
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	if purchase.Status != "active" && purchase.Status != "activating" && purchase.Status != "queued" {
		return model.RenewalQuote{}, model.Combo{}, nil, ErrConflict
	}
	combo, err := comboByIDTx(ctx, tx, purchase.ComboID, true)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	combo, err = renewalComboTx(ctx, tx, purchaseID, combo)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	addons, err := renewalAddonsTx(ctx, tx, purchaseID, excludedAddonIDs)
	if err != nil {
		return model.RenewalQuote{}, model.Combo{}, nil, err
	}
	var customPrice sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT reward_renewal_price_minor FROM purchases WHERE id=?`, purchaseID).Scan(&customPrice); err != nil {
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
		grossPerTerm += addon.PriceTXBMinor
		addonIDs = append(addonIDs, addon.GroupID)
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
	return model.RenewalQuote{PurchaseID: purchaseID, ComboID: combo.ID, TermCount: termCount, GrossPrice: model.TXBMoney(grossPerTerm), Discount: model.TXBMoney(discount.DiscountMinor), PricePerTerm: model.TXBMoney(discount.NetMinor), TotalPrice: model.TXBMoney(discount.NetMinor * int64(termCount)), CouponGrantID: couponGrantID, EffectiveAt: start,
		ExpiresAt: start.AddDate(0, 0, combo.ValidityDays*termCount), AddonSquadUUIDs: addonIDs}, combo, addons, nil
}
