package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PurchaseByID returns transaction facts combined with the combo's current
// live configuration, as required by the mutable-catalog contract.
func (s *Store) PurchaseByID(ctx context.Context, id string) (model.Purchase, error) {
	purchase, err := scanPurchase(s.db.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1`, id))
	if err != nil {
		return model.Purchase{}, err
	}
	purchase.SquadUUIDs, err = s.purchaseSquads(ctx, purchase.ID)
	return purchase, err
}

// ListPurchases returns a user's purchase history.
func (s *Store) ListPurchases(ctx context.Context, userID string) ([]model.Purchase, error) {
	rows, err := s.db.QueryContext(ctx, purchaseSelect+` WHERE purchases.user_id=$1 ORDER BY purchases.valid_from DESC,purchases.created_at DESC`, userID)
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
	sort.Slice(purchases, func(i, j int) bool { return moreRecentPurchase(purchases[i], purchases[j]) })
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
	rows, err := s.db.QueryContext(ctx, purchaseSelect+` ORDER BY purchases.created_at DESC LIMIT $1`, limit)
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

const purchaseSelect = `SELECT purchases.id,purchases.user_id,purchases.combo_id,combos.name,purchases.charged_txb_minor,purchases.valid_from,purchases.valid_until,
	purchases.status,COALESCE(purchases.entitlement_traffic_limit_bytes,combos.traffic_limit_bytes),COALESCE(purchases.entitlement_reset_strategy,combos.reset_strategy),purchases.coupon_grant_id,COALESCE(purchases.gross_price_txb_minor,purchases.charged_txb_minor),purchases.core_gross_txb_minor,purchases.coupon_discount_txb_minor,
	purchases.auto_renew_enabled,purchases.recurring_discount_attached,purchases.created_at,purchases.updated_at,purchases.purchased_term_seconds FROM purchases JOIN combos ON combos.id=purchases.combo_id`

func scanPurchase(row rowScanner) (model.Purchase, error) {
	var purchase model.Purchase
	var validFrom, validUntil, created, updated string
	var couponGrantID sql.NullString
	var purchasedTermSeconds sql.NullInt64
	var autoRenewEnabled, recurringDiscountAttached int
	if err := row.Scan(&purchase.ID, &purchase.UserID, &purchase.ComboID, &purchase.ComboName, &purchase.PriceTXBMinor,
		&validFrom, &validUntil, &purchase.Status, &purchase.TrafficLimitBytes, &purchase.ResetStrategy, &couponGrantID, &purchase.GrossPriceTXBMinor,
		&purchase.CoreGrossTXBMinor, &purchase.CouponDiscountTXBMinor, &autoRenewEnabled, &recurringDiscountAttached, &created, &updated, &purchasedTermSeconds); err != nil {
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
	purchase.PurchasedTermSeconds = int64Pointer(purchasedTermSeconds)
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
	rows, err := queryer.QueryContext(ctx, `WITH selected_purchases AS (SELECT `+purchaseSquadColumns+` FROM purchases WHERE id=$1), `+purchaseSquadMembershipCTE+`
		SELECT DISTINCT group_id FROM owned_squads ORDER BY group_id`, purchaseID)
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
