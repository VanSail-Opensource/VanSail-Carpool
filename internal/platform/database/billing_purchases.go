package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strconv"
	"time"
)

// PurchaseByID returns transaction facts combined with the combo's current
// live configuration, as required by the mutable-catalog contract.
func (s *Store) PurchaseByID(ctx context.Context, id string) (model.Purchase, error) {
	purchase, err := scanPurchase(s.db.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=?`, id))
	if err != nil {
		return model.Purchase{}, err
	}
	purchase.SquadUUIDs, err = s.purchaseSquads(ctx, purchase.ID)
	return purchase, err
}

// ListPurchases returns a user's purchase history.
func (s *Store) ListPurchases(ctx context.Context, userID string) ([]model.Purchase, error) {
	rows, err := s.db.QueryContext(ctx, purchaseSelect+` WHERE purchases.user_id=? ORDER BY purchases.valid_from DESC,purchases.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list purchases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	purchases := make([]model.Purchase, 0)
	for rows.Next() {
		purchase, err := scanPurchase(rows)
		if err != nil {
			return nil, err
		}
		purchases = append(purchases, purchase)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range purchases {
		purchases[index].SquadUUIDs, err = s.purchaseSquads(ctx, purchases[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return purchases, nil
}

// ListAllPurchases returns recent entitlement records for the administrative view.
func (s *Store) ListAllPurchases(ctx context.Context, limit int) ([]model.Purchase, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, purchaseSelect+` ORDER BY purchases.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	purchases := make([]model.Purchase, 0)
	for rows.Next() {
		purchase, err := scanPurchase(rows)
		if err != nil {
			return nil, err
		}
		purchases = append(purchases, purchase)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range purchases {
		purchases[index].SquadUUIDs, err = s.purchaseSquads(ctx, purchases[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return purchases, nil
}

// CancelPurchase credits its snapshotted price and schedules entitlement replacement when needed.
func (s *Store) CancelPurchase(ctx context.Context, purchaseID, reason string, now time.Time) (model.Purchase, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=?`, purchaseID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.Status == "cancelled" {
		return purchase, nil
	}
	if purchase.Status == "expired" || purchase.Status == "failed" {
		return model.Purchase{}, ErrConflict
	}
	previousStatus := purchase.Status
	if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=?`, stamp(now), purchase.ID); err != nil {
		return model.Purchase{}, err
	}
	balance, err := adjustBalanceTx(ctx, tx, purchase.UserID, purchase.PriceTXBMinor, now)
	if err != nil {
		return model.Purchase{}, fmt.Errorf("refund cancelled purchase: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, purchase.UserID, purchase.PriceTXBMinor, balance, "admin_entitlement_cancellation", purchase.ID, reason, now); err != nil {
		return model.Purchase{}, err
	}
	if previousStatus != "queued" {
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+purchase.UserID+`"}`, now, now); err != nil {
			return model.Purchase{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return s.PurchaseByID(ctx, purchase.ID)
}

const purchaseSelect = `SELECT purchases.id,purchases.user_id,purchases.combo_id,combos.name,purchases.charged_txb_minor,purchases.valid_from,purchases.valid_until,
	purchases.status,COALESCE(purchases.entitlement_traffic_limit_bytes,combos.traffic_limit_bytes),COALESCE(purchases.entitlement_reset_strategy,combos.reset_strategy),purchases.coupon_grant_id,COALESCE(purchases.gross_price_txb_minor,purchases.charged_txb_minor),purchases.core_gross_txb_minor,purchases.coupon_discount_txb_minor,
	COALESCE(purchases.reward_rollover_min_remaining_bps,combos.rollover_min_remaining_bps),purchases.auto_renew_enabled,purchases.recurring_discount_attached,purchases.created_at,purchases.updated_at FROM purchases JOIN combos ON combos.id=purchases.combo_id`

func scanPurchase(row rowScanner) (model.Purchase, error) {
	var purchase model.Purchase
	var validFrom, validUntil, created, updated string
	var couponGrantID sql.NullString
	var autoRenewEnabled, recurringDiscountAttached int
	if err := row.Scan(&purchase.ID, &purchase.UserID, &purchase.ComboID, &purchase.ComboName, &purchase.PriceTXBMinor,
		&validFrom, &validUntil, &purchase.Status, &purchase.TrafficLimitBytes, &purchase.ResetStrategy, &couponGrantID, &purchase.GrossPriceTXBMinor,
		&purchase.CoreGrossTXBMinor, &purchase.CouponDiscountTXBMinor, &purchase.RolloverMinRemainingBPS, &autoRenewEnabled, &recurringDiscountAttached, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Purchase{}, ErrNotFound
		}
		return model.Purchase{}, fmt.Errorf("scan purchase: %w", err)
	}
	purchase.Price = model.TXBMoney(purchase.PriceTXBMinor)
	purchase.GrossPrice = model.TXBMoney(purchase.GrossPriceTXBMinor)
	purchase.CouponDiscount = model.TXBMoney(purchase.CouponDiscountTXBMinor)
	purchase.CouponGrantID = nullableString(couponGrantID)
	purchase.AutoRenewEnabled = autoRenewEnabled == 1
	purchase.RecurringDiscountAttached = recurringDiscountAttached == 1
	purchase.TrafficLimit = strconv.FormatInt(purchase.TrafficLimitBytes, 10)
	var err error
	if purchase.ValidFrom, err = parseStamp(validFrom); err != nil {
		return model.Purchase{}, err
	}
	if purchase.ValidUntil, err = parseStamp(validUntil); err != nil {
		return model.Purchase{}, err
	}
	if purchase.CreatedAt, err = parseStamp(created); err != nil {
		return model.Purchase{}, err
	}
	purchase.UpdatedAt, err = parseStamp(updated)
	return purchase, err
}

func (s *Store) purchaseSquads(ctx context.Context, purchaseID string) ([]string, error) {
	return purchaseSquadsFrom(ctx, s.db, purchaseID)
}

type purchaseSquadQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func purchaseSquadsFrom(ctx context.Context, queryer purchaseSquadQueryer, purchaseID string) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT value AS group_id FROM purchases,
		json_each(purchases.entitlement_squad_uuids) WHERE purchases.id=? AND purchases.entitlement_squad_uuids IS NOT NULL
		UNION SELECT value FROM purchases JOIN combos ON combos.id=purchases.combo_id,json_each(combos.included_squad_uuids)
		WHERE purchases.id=? AND purchases.entitlement_squad_uuids IS NULL
		UNION SELECT purchase_addons.group_id FROM purchase_addons JOIN purchases ON purchases.id=purchase_addons.purchase_id
		WHERE purchase_addons.purchase_id=? AND purchases.entitlement_squad_uuids IS NULL AND purchases.entitlement_addon_squad_uuids IS NULL
		UNION SELECT value FROM purchases,json_each(purchases.entitlement_addon_squad_uuids)
		WHERE purchases.id=? AND purchases.entitlement_squad_uuids IS NULL AND purchases.entitlement_addon_squad_uuids IS NOT NULL
		UNION SELECT squad_uuid FROM activity_reward_squad_access WHERE purchase_id=?
		ORDER BY group_id`, purchaseID, purchaseID, purchaseID, purchaseID, purchaseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
