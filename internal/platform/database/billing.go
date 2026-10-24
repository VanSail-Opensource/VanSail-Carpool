package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

// PurchaseInput contains the server-selected catalog IDs for a checkout.
type PurchaseInput struct {
	UserID               string
	ComboID              string
	AddonSquadIDs        []string
	CouponGrantID        string
	IdempotencyKey       string
	SquadActivationCodes map[string]string
}

// CreatePurchase atomically revalidates live pricing, debits TXB, stores only
// transaction facts and selected paid add-ons, and queues Proxy sync.
func (s *Store) retryAttempt_CreatePurchase(ctx context.Context, input PurchaseInput, now time.Time) (model.Purchase, error) {
	fingerprint, err := normalizeAndFingerprintPurchase(&input)
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
		return model.Purchase{}, fmt.Errorf("begin purchase: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var existingID, existingFingerprint string
	loadErr := tx.QueryRowContext(ctx, `SELECT id,request_fingerprint FROM purchases WHERE user_id=? AND idempotency_key=?`, input.UserID, input.IdempotencyKey).Scan(&existingID, &existingFingerprint)
	if loadErr == nil {
		if existingFingerprint != fingerprint {
			return model.Purchase{}, ErrConflict
		}
		// End the read transaction before loading the full aggregate (including
		// purchase_squads) through the normal repository query path.
		if err := tx.Rollback(); err != nil {
			return model.Purchase{}, fmt.Errorf("close purchase replay transaction: %w", err)
		}
		return s.PurchaseByID(ctx, existingID)
	}
	if !errors.Is(loadErr, sql.ErrNoRows) {
		return model.Purchase{}, fmt.Errorf("load purchase idempotency key: %w", loadErr)
	}

	quote, combo, addonRows, err := quotePurchaseTx(ctx, tx, input, now)
	if err != nil {
		return model.Purchase{}, err
	}
	if err := validateSquadActivationCodesTx(ctx, tx, combo, addonRows, input.SquadActivationCodes); err != nil {
		return model.Purchase{}, err
	}
	validFrom := quote.EffectiveAt
	validUntil := quote.ExpiresAt
	status := "queued"
	if !validFrom.After(now) {
		status = "activating"
	}

	purchaseID, err := ids.New()
	if err != nil {
		return model.Purchase{}, err
	}
	discount := coupons.Discount{GrossMinor: quote.GrossPriceTXBMinor, NetMinor: quote.GrossPriceTXBMinor}
	if input.CouponGrantID != "" {
		pricedAddonIDs := make([]string, 0, len(addonRows))
		for _, addon := range addonRows {
			pricedAddonIDs = append(pricedAddonIDs, addon.GroupID)
		}
		discount, err = applyPurchaseCouponTx(ctx, tx, coupons.PurchaseContext{
			UserID: input.UserID, GrantID: input.CouponGrantID, ComboID: combo.ID,
			AddonSquadIDs: pricedAddonIDs, GrossPriceMinor: quote.GrossPriceTXBMinor,
		}, purchaseID, now)
		if err != nil {
			return model.Purchase{}, err
		}
	}
	netPrice := discount.NetMinor
	newBalance, err := debitBalanceTx(ctx, tx, input.UserID, netPrice, now)
	if err != nil {
		return model.Purchase{}, err
	}
	var couponGrantID any
	if input.CouponGrantID != "" {
		couponGrantID = input.CouponGrantID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,coupon_grant_id,gross_price_txb_minor,core_gross_txb_minor,coupon_discount_txb_minor,auto_renew_enabled,recurring_discount_attached,idempotency_key,request_fingerprint,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, purchaseID, input.UserID, combo.ID, netPrice, stamp(validFrom), stamp(validUntil), status,
		couponGrantID, quote.GrossPriceTXBMinor, combo.PriceTXBMinor, discount.DiscountMinor, 0, boolInt(discount.Recurring), input.IdempotencyKey, fingerprint, stamp(now), stamp(now))
	if err != nil {
		return model.Purchase{}, fmt.Errorf("insert purchase: %w", err)
	}
	if err := snapshotComboSquadsTx(ctx, tx, purchaseID, combo.IncludedSquads); err != nil {
		return model.Purchase{}, err
	}
	if status == "activating" {
		if err := applyPendingExtensionsToActivationTx(ctx, tx, purchaseID, now); err != nil {
			return model.Purchase{}, fmt.Errorf("apply pending subscription extensions: %w", err)
		}
	}
	for _, product := range addonRows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES(?,?,?)`, purchaseID, product.GroupID, product.PriceTXBMinor); err != nil {
			return model.Purchase{}, fmt.Errorf("snapshot add-on squad: %w", err)
		}
	}
	if _, err := insertLedgerTx(ctx, tx, input.UserID, -netPrice, newBalance, "purchase_debit", purchaseID, combo.Name, now); err != nil {
		return model.Purchase{}, err
	}
	if err := enqueuePurchaseTransitionTx(ctx, tx, purchaseID, status, validFrom, now); err != nil {
		return model.Purchase{}, err
	}
	if status == "queued" {
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+input.UserID+`"}`, now, now); err != nil {
			return model.Purchase{}, err
		}
		if err := s.insertQueuedPurchaseNoticeTx(ctx, tx, purchaseID, input.UserID, combo, addonRows,
			netPrice, newBalance, validFrom, validUntil, now); err != nil {
			return model.Purchase{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, fmt.Errorf("commit purchase: %w", err)
	}
	return s.PurchaseByID(ctx, purchaseID)
}
