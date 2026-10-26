package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// QuotePurchase performs the same catalog, coupon, and effective-date checks as
// checkout without changing balances, coupon grants, or entitlement state.
func (s *Store) retryAttempt_QuotePurchase(ctx context.Context, input PurchaseInput, now time.Time) (model.PurchaseQuote, error) {
	if err := s.PreparePurchasePricing(ctx, input, now); err != nil {
		return model.PurchaseQuote{}, err
	}
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	input.IdempotencyKey = "quote"
	if _, err := normalizeAndFingerprintPurchase(&input); err != nil {
		return model.PurchaseQuote{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.PurchaseQuote{}, fmt.Errorf("begin purchase quote: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, _, err := s.quotePurchaseTx(ctx, tx, input, now.UTC())
	return quote, err
}

func (s *Store) quotePurchaseTx(ctx context.Context, tx *sql.Tx, input PurchaseInput, now time.Time) (model.PurchaseQuote, model.Combo, []model.SquadProduct, error) {
	combo, err := comboByIDTx(ctx, tx, input.ComboID, true)
	if err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	combo.PriceTXBMinor, err = resolvePricingTx(ctx, tx, "combo", combo.ID, combo.PriceTXBMinor, now)
	if err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	originalSquads := len(combo.IncludedSquads)
	combo.IncludedSquads, err = comboSquadsTx(ctx, tx, combo.IncludedSquads)
	if err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	includedUUIDs := make(map[string]struct{}, len(combo.IncludedSquads))
	for _, product := range combo.IncludedSquads {
		includedUUIDs[product.GroupID] = struct{}{}
	}
	addonRows := make([]model.SquadProduct, 0, len(input.AddonSquadIDs))
	addonPrice := int64(0)
	for _, squadUUID := range uniqueSorted(input.AddonSquadIDs) {
		product, loadErr := squadByIDTx(ctx, tx, squadUUID, true)
		if loadErr != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, loadErr
		}
		if product.BillingMode == "payg" {
			return model.PurchaseQuote{}, model.Combo{}, nil, ErrNotFound
		}
		if _, included := includedUUIDs[product.GroupID]; included {
			continue
		}
		product.PriceTXBMinor, loadErr = resolvePricingTx(ctx, tx, "addon", product.GroupID, product.PriceTXBMinor, now)
		if loadErr != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, loadErr
		}
		product.PriceTXBMinor, _, loadErr = personalProductPriceTx(ctx, tx, input.UserID, "addon", product.GroupID, combo.ID, []string{product.GroupID}, now, false)
		if loadErr != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, loadErr
		}
		includedUUIDs[product.GroupID] = struct{}{}
		addonRows = append(addonRows, product)
		addonPrice += product.PriceTXBMinor
	}
	if originalSquads > 0 && len(includedUUIDs) == 0 {
		return model.PurchaseQuote{}, model.Combo{}, nil, ErrConflict
	}
	selectedSquads := make([]string, 0, len(includedUUIDs))
	for squadUUID := range includedUUIDs {
		selectedSquads = append(selectedSquads, squadUUID)
	}
	if err := checkSquadStockTx(ctx, tx, selectedSquads, input.UserID); err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	validFrom, err := nextPurchaseStartTx(ctx, tx, input.UserID, now)
	if err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	var renewalDecision *model.PricingDecision
	if validFrom.After(now) {
		renewalDecision, err = s.queuedRenewalDecisionTx(ctx, tx, input.UserID, now)
		if err != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, err
		}
	}
	personalPrice, decision, err := personalProductPriceTx(ctx, tx, input.UserID, "combo", combo.ID, combo.ID, selectedSquads, now, validFrom.After(now))
	if err != nil {
		return model.PurchaseQuote{}, model.Combo{}, nil, err
	}
	combo.PriceTXBMinor = personalPrice
	if input.CouponGrantID == "" {
		combo.PriceTXBMinor, err = personalPricingTx(ctx, tx, input.UserID, "combo", combo.ID, combo.PriceTXBMinor, 1, "", false, now)
		if err != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, err
		}
	}
	grossMinor := combo.PriceTXBMinor + addonPrice
	discount := coupons.Discount{GrossMinor: grossMinor, NetMinor: grossMinor}
	addonUUIDs := make([]string, 0, len(addonRows))
	for _, addon := range addonRows {
		addonUUIDs = append(addonUUIDs, addon.GroupID)
	}
	if input.CouponGrantID != "" {
		discount, err = quoteCouponGrantTx(ctx, tx, coupons.PurchaseContext{
			UserID: input.UserID, GrantID: input.CouponGrantID, ComboID: combo.ID,
			AddonSquadIDs: addonUUIDs, GrossPriceMinor: grossMinor,
		}, now)
		if err != nil {
			return model.PurchaseQuote{}, model.Combo{}, nil, err
		}
	}
	return model.PurchaseQuote{
		PricingDecision: decision, RenewalDecision: renewalDecision,
		ComboID: combo.ID, ComboName: combo.Name,
		GrossPriceTXBMinor: grossMinor, DiscountTXBMinor: discount.DiscountMinor, NetPriceTXBMinor: discount.NetMinor,
		GrossPrice: model.TXBMoney(grossMinor), Discount: model.TXBMoney(discount.DiscountMinor), NetPrice: model.TXBMoney(discount.NetMinor),
		EffectiveAt: validFrom, ExpiresAt: validFrom.AddDate(0, 0, combo.ValidityDays), Queued: validFrom.After(now), AddonSquadUUIDs: addonUUIDs,
	}, combo, addonRows, nil
}
