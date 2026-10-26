package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// PurchaseAddonInput contains a requested active-term squad addition.
type PurchaseAddonInput struct {
	CouponGrantID        string
	UserID               string
	PurchaseID           string
	AddonSquadIDs        []string
	IdempotencyKey       string
	SquadActivationCodes map[string]string
}

// QuotePurchaseAddons returns the authoritative active-term price without a mutation.
func (s *Store) retryAttempt_quotePurchaseAddons(ctx context.Context, input PurchaseAddonInput, now time.Time) (model.PurchaseAddonQuote, error) {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	input.IdempotencyKey = "quote"
	if _, err := normalizeAndFingerprintPurchaseAddons(&input); err != nil {
		return model.PurchaseAddonQuote{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.PurchaseAddonQuote{}, fmt.Errorf("begin purchase add-on quote: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, err := quotePurchaseAddonsTx(ctx, tx, input, now.UTC())
	return quote, err
}

// AddPurchaseAddons atomically debits the price, records squads, and queues reconciliation.
func (s *Store) retryAttempt_addPurchaseAddons(ctx context.Context, input PurchaseAddonInput, now time.Time) (model.Purchase, error) {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	fingerprint, err := normalizeAndFingerprintPurchaseAddons(&input)
	if err != nil {
		return model.Purchase{}, err
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, fmt.Errorf("begin purchase add-on: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var owned int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE id=? AND user_id=?)`, input.PurchaseID, input.UserID).Scan(&owned); err != nil {
		return model.Purchase{}, fmt.Errorf("verify purchase ownership: %w", err)
	}
	if owned == 0 {
		return model.Purchase{}, ErrNotFound
	}
	var existingFingerprint string
	err = tx.QueryRowContext(ctx, `SELECT request_fingerprint FROM purchase_addon_adjustments WHERE purchase_id=? AND idempotency_key=?`, input.PurchaseID, input.IdempotencyKey).Scan(&existingFingerprint)
	if err == nil {
		if existingFingerprint != fingerprint {
			return model.Purchase{}, ErrConflict
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return model.Purchase{}, fmt.Errorf("close purchase add-on replay transaction: %w", rollbackErr)
		}
		return s.PurchaseByID(ctx, input.PurchaseID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.Purchase{}, fmt.Errorf("load purchase add-on idempotency key: %w", err)
	}
	quote, addons, err := quotePurchaseAddonsTx(ctx, tx, input, now)
	if err != nil {
		return model.Purchase{}, err
	}
	products := make([]model.SquadProduct, 0, len(addons))
	for _, addon := range addons {
		products = append(products, addon.product)
	}
	if err := validateSquadActivationCodesTx(ctx, tx, model.Combo{}, products, input.SquadActivationCodes); err != nil {
		return model.Purchase{}, err
	}
	adjustmentID, err := ids.New()
	if err != nil {
		return model.Purchase{}, err
	}
	if input.CouponGrantID != "" {
		var combo string
		if err = tx.QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE id=?`, input.PurchaseID).Scan(&combo); err != nil {
			return model.Purchase{}, err
		}
		discount, e := applyPurchaseCouponTx(ctx, tx, coupons.PurchaseContext{UserID: input.UserID, GrantID: input.CouponGrantID, ComboID: combo, AddonSquadIDs: quote.AddonSquadUUIDs, GrossPriceMinor: quote.GrossPriceTXBMinor}, adjustmentID, now)
		if e != nil {
			return model.Purchase{}, e
		}
		if discount.NetMinor != quote.PriceTXBMinor {
			return model.Purchase{}, pricing.VersionConflict
		}
	}
	newBalance, err := debitBalanceTx(ctx, tx, input.UserID, quote.PriceTXBMinor, now)
	if err != nil {
		return model.Purchase{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_addon_adjustments(id,purchase_id,idempotency_key,request_fingerprint,charged_txb_minor,created_at) VALUES(?,?,?,?,?,?)`, adjustmentID, input.PurchaseID, input.IdempotencyKey, fingerprint, quote.PriceTXBMinor, stamp(now)); err != nil {
		return model.Purchase{}, fmt.Errorf("record purchase add-on adjustment: %w", err)
	}
	for _, a := range addons {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_offers SET outcome=1,outcome_reason='',observed_at=?,purchase_id=? WHERE user_id=? AND product_type='addon' AND product_id=? AND price_minor=? AND outcome IS NULL AND json_extract(features_json,'$.couponFingerprint')=? AND expires_at>?`, pricingStamp(now), adjustmentID, input.UserID, a.product.GroupID, a.product.PriceTXBMinor, a.fingerprint, pricingStamp(now)); err != nil {
			return model.Purchase{}, err
		}
	}
	for _, addon := range addons {
		if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES(?,?,?)`, input.PurchaseID, addon.product.GroupID, addon.price); err != nil {
			if isUniqueConstraint(err) {
				return model.Purchase{}, ErrSquadAlreadyAdded
			}
			return model.Purchase{}, fmt.Errorf("record purchase add-on squad: %w", err)
		}
	}
	if err := invalidatePricingRenewalBasketTx(ctx, tx, input.PurchaseID); err != nil {
		return model.Purchase{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET charged_txb_minor=charged_txb_minor+?,gross_price_txb_minor=gross_price_txb_minor+?,coupon_discount_txb_minor=coupon_discount_txb_minor+?,updated_at=? WHERE id=? AND user_id=?`, quote.PriceTXBMinor, quote.GrossPriceTXBMinor, quote.DiscountTXBMinor, stamp(now), input.PurchaseID, input.UserID); err != nil {
		return model.Purchase{}, fmt.Errorf("update purchase add-on totals: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, input.UserID, -quote.PriceTXBMinor, newBalance, "purchase_addon_debit", adjustmentID, "purchase squad addition", now); err != nil {
		return model.Purchase{}, err
	}
	if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+input.UserID+`"}`, now, now); err != nil {
		return model.Purchase{}, err
	}
	if err := s.insertAddonNoticeTx(ctx, tx, adjustmentID, input.UserID, input.PurchaseID,
		products, quote.PriceTXBMinor, newBalance, now); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, fmt.Errorf("commit purchase add-on: %w", err)
	}
	return s.PurchaseByID(ctx, input.PurchaseID)
}
