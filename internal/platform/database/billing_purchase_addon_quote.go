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

var (
	// ErrPurchaseNotActive means an add-on change cannot alter the requested term.
	ErrPurchaseNotActive = errors.New("purchase is not active")
	// ErrQueuedPurchase means a separately scheduled term owns the next boundary.
	ErrQueuedPurchase = errors.New("queued purchase prevents squad addition")
	// ErrSquadAlreadyAdded means the active term already contains a selected squad.
	ErrSquadAlreadyAdded = errors.New("squad is already added to purchase")
	// ErrAddonEntitlementOverride prevents charging for masked add-on facts.
	ErrAddonEntitlementOverride = errors.New("purchase entitlement override prevents squad addition")
)

type pricedPurchaseAddon struct {
	product     model.SquadProduct
	price       int64
	fingerprint string
}

func quotePurchaseAddonsTx(ctx context.Context, tx *sql.Tx, input PurchaseAddonInput, now time.Time) (model.PurchaseAddonQuote, []pricedPurchaseAddon, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, input.PurchaseID, input.UserID))
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	var overridden bool
	if err := tx.QueryRowContext(ctx, `SELECT entitlement_squad_uuids IS NOT NULL OR entitlement_addon_squad_uuids IS NOT NULL FROM purchases WHERE id=?`, purchase.ID).Scan(&overridden); err != nil {
		return model.PurchaseAddonQuote{}, nil, fmt.Errorf("check add-on entitlement override: %w", err)
	}
	if overridden {
		return model.PurchaseAddonQuote{}, nil, ErrAddonEntitlementOverride
	}
	if (purchase.Status != "active" && purchase.Status != "activating") || !purchase.ValidUntil.After(now) {
		return model.PurchaseAddonQuote{}, nil, ErrPurchaseNotActive
	}
	if conflict, err := comboControlConflictFrom(ctx, tx, input.UserID); err != nil || conflict {
		if err == nil {
			err = ErrConflict
		}
		return model.PurchaseAddonQuote{}, nil, err
	}
	combo, err := comboByIDTx(ctx, tx, purchase.ComboID, false)
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	existingSquads, err := purchaseSquadsFrom(ctx, tx, purchase.ID)
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	existing := make(map[string]struct{}, len(existingSquads))
	for _, squadUUID := range existingSquads {
		existing[squadUUID] = struct{}{}
	}
	if err := purchaseAddonFactsTx(ctx, tx, purchase.ID, existing); err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	selected := make([]string, 0, len(input.AddonSquadIDs))
	products := make([]model.SquadProduct, 0, len(input.AddonSquadIDs))
	for _, squadID := range input.AddonSquadIDs {
		product, loadErr := squadByIDTx(ctx, tx, squadID, true)
		if loadErr != nil {
			return model.PurchaseAddonQuote{}, nil, loadErr
		}
		if _, found := existing[product.GroupID]; found {
			return model.PurchaseAddonQuote{}, nil, ErrSquadAlreadyAdded
		}
		existing[product.GroupID] = struct{}{}
		selected = append(selected, product.GroupID)
		products = append(products, product)
	}
	if err := checkSquadStockTx(ctx, tx, selected, input.UserID); err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	original := time.Duration(combo.ValidityDays) * 24 * time.Hour
	remaining := purchase.ValidUntil.Sub(now)
	priced := make([]pricedPurchaseAddon, 0, len(products))
	total := int64(0)
	for _, product := range products {
		var resolveErr error
		product.PriceTXBMinor, resolveErr = resolvePricingTx(ctx, tx, "addon", product.GroupID, product.PriceTXBMinor, now)
		if resolveErr != nil {
			return model.PurchaseAddonQuote{}, nil, resolveErr
		}
		personal, decision, resolveErr := personalProductPriceTx(ctx, tx, input.UserID, "addon", product.GroupID, combo.ID, input.AddonSquadIDs, now, false)
		if resolveErr != nil {
			return model.PurchaseAddonQuote{}, nil, resolveErr
		}
		product.PricingDecision = decision
		product.PriceTXBMinor = personal
		price, priceErr := proratedAddonPrice(product.PriceTXBMinor, remaining, original)
		if priceErr != nil {
			return model.PurchaseAddonQuote{}, nil, priceErr
		}
		if total > int64(^uint64(0)>>1)-price {
			return model.PurchaseAddonQuote{}, nil, errors.New("purchase add-on price exceeds integer range")
		}
		total += price
		priced = append(priced, pricedPurchaseAddon{product: product, price: price, fingerprint: decision.CouponFingerprint})
	}
	uuids := make([]string, 0, len(priced))
	for _, addon := range priced {
		uuids = append(uuids, addon.product.GroupID)
	}
	discount := coupons.Discount{GrossMinor: total, NetMinor: total}
	if input.CouponGrantID != "" {
		discount, err = quoteCouponGrantTx(ctx, tx, coupons.PurchaseContext{UserID: input.UserID, GrantID: input.CouponGrantID, ComboID: combo.ID, AddonSquadIDs: uuids, GrossPriceMinor: total}, now)
		if err != nil {
			return model.PurchaseAddonQuote{}, nil, err
		}
	}
	quote := model.PurchaseAddonQuote{PurchaseID: purchase.ID, GrossPriceTXBMinor: total, DiscountTXBMinor: discount.DiscountMinor, GrossPrice: model.TXBMoney(total), Discount: model.TXBMoney(discount.DiscountMinor), PriceTXBMinor: discount.NetMinor, Price: model.TXBMoney(discount.NetMinor), ExpiresAt: purchase.ValidUntil, AddonSquadUUIDs: uuids}
	if _, err = bindAddonQuoteTx(ctx, tx, input, &quote, priced, now); err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	return quote, priced, nil
}

func purchaseAddonFactsTx(ctx context.Context, tx *sql.Tx, purchaseID string, existing map[string]struct{}) error {
	rows, err := tx.QueryContext(ctx, `SELECT group_id FROM purchase_addons WHERE purchase_id=?`, purchaseID)
	if err != nil {
		return fmt.Errorf("load purchase add-on facts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return fmt.Errorf("scan purchase add-on fact: %w", err)
		}
		existing[uuid] = struct{}{}
	}
	return rows.Err()
}
