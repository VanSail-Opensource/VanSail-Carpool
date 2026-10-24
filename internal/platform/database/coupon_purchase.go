package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
)

func (s *Store) retryAttempt_QuotePurchaseCoupon(ctx context.Context, input coupons.PurchaseContext, now time.Time) (coupons.Discount, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return coupons.Discount{}, err
	}
	defer func() { _ = tx.Rollback() }()
	discount, err := quoteCouponGrantTx(ctx, tx, input, now)
	if err != nil {
		return coupons.Discount{}, err
	}
	return discount, nil
}

// applyPurchaseCouponTx consumes one selected grant inside the purchase transaction.
// Checkout must call this only after it has assigned the immutable purchase ID and
// before it debits the returned net price.

func applyPurchaseCouponTx(ctx context.Context, tx *sql.Tx, input coupons.PurchaseContext, purchaseID string, now time.Time) (coupons.Discount, error) {
	if strings.TrimSpace(purchaseID) == "" {
		return coupons.Discount{}, coupons.ErrInvalidInput
	}
	discount, err := quoteCouponGrantTx(ctx, tx, input, now)
	if err != nil {
		return coupons.Discount{}, err
	}
	grant, err := grantByIDTx(ctx, tx, input.GrantID)
	if err != nil {
		return coupons.Discount{}, err
	}
	discarded, err := grantDiscardedTx(ctx, tx, grant.ID)
	if err != nil {
		return coupons.Discount{}, err
	}
	if discarded {
		return coupons.Discount{}, ErrConflict
	}
	if err := ensureCouponUseAvailableTx(ctx, tx, grant.Coupon, input.UserID); err != nil {
		return coupons.Discount{}, err
	}
	useID, err := ids.New()
	if err != nil {
		return coupons.Discount{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO coupon_uses(id,coupon_id,grant_id,user_id,purchase_id,gross_price_minor,discount_minor,net_price_minor,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, useID, grant.Coupon.ID, grant.ID, input.UserID, purchaseID, discount.GrossMinor, discount.DiscountMinor, discount.NetMinor, stamp(now)); err != nil {
		if isUniqueConstraint(err) {
			return coupons.Discount{}, ErrConflict
		}
		return coupons.Discount{}, fmt.Errorf("record coupon use: %w", err)
	}
	status := "active"
	var consumed any
	if grant.Coupon.Kind == coupons.KindPurchaseOnce {
		status = "consumed"
		consumed = stamp(now)
	}
	result, err := tx.ExecContext(ctx, `UPDATE coupon_grants SET use_count=use_count+1,status=?,consumed_at=?
		WHERE id=? AND status='active' AND NOT EXISTS (SELECT 1 FROM coupon_grant_discards WHERE grant_id=?)`, status, consumed, grant.ID, grant.ID)
	if err != nil {
		return coupons.Discount{}, fmt.Errorf("consume coupon grant: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return coupons.Discount{}, rowsErr
	} else if affected != 1 {
		return coupons.Discount{}, ErrConflict
	}
	return discount, nil
}

func quoteCouponGrantTx(ctx context.Context, tx *sql.Tx, input coupons.PurchaseContext, now time.Time) (coupons.Discount, error) {
	if input.GrossPriceMinor < 0 || strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.GrantID) == "" || strings.TrimSpace(input.ComboID) == "" {
		return coupons.Discount{}, coupons.ErrInvalidInput
	}
	grant, err := grantByIDTx(ctx, tx, input.GrantID)
	if err != nil {
		return coupons.Discount{}, err
	}
	if grant.UserID != input.UserID || grant.Status != "active" {
		return coupons.Discount{}, ErrConflict
	}
	discarded, err := grantDiscardedTx(ctx, tx, grant.ID)
	if err != nil {
		return coupons.Discount{}, err
	}
	if discarded {
		return coupons.Discount{}, ErrConflict
	}
	if err := couponAvailable(grant.Coupon, now); err != nil {
		return coupons.Discount{}, err
	}
	if !grant.Coupon.EligibleFor(input.ComboID, input.AddonSquadIDs) {
		return coupons.Discount{}, ErrConflict
	}
	if err := ensureCouponUseAvailableTx(ctx, tx, grant.Coupon, input.UserID); err != nil {
		return coupons.Discount{}, err
	}
	value, err := coupons.CalculateDiscount(grant.Coupon, input.GrossPriceMinor)
	if err != nil {
		return coupons.Discount{}, err
	}
	return coupons.Discount{GrantID: grant.ID, CouponID: grant.Coupon.ID, CouponCode: grant.Coupon.Code,
		GrossMinor: input.GrossPriceMinor, DiscountMinor: value, NetMinor: input.GrossPriceMinor - value,
		Recurring: grant.Coupon.Kind == coupons.KindPurchaseRecurring}, nil
}

func grantCouponTx(ctx context.Context, tx *sql.Tx, userID string, coupon coupons.Coupon, sourceType, sourceID string, now time.Time) (coupons.Grant, error) {
	if coupon.Kind != coupons.KindPurchaseRecurring && coupon.Kind != coupons.KindPurchaseOnce {
		return coupons.Grant{}, fmt.Errorf("%w: only purchase coupons can be wallet grants", coupons.ErrInvalidInput)
	}
	if existing, err := grantBySourceTx(ctx, tx, userID, coupon.ID, sourceType, sourceID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return coupons.Grant{}, err
	}
	grantID, err := ids.New()
	if err != nil {
		return coupons.Grant{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO coupon_grants(id,coupon_id,user_id,source_type,source_id,status,use_count,created_at)
		VALUES(?,?,?,?,?,'active',0,?)`, grantID, coupon.ID, userID, sourceType, sourceID, stamp(now))
	if err != nil {
		return coupons.Grant{}, fmt.Errorf("grant coupon: %w", err)
	}
	return coupons.Grant{ID: grantID, Coupon: coupon, UserID: userID, SourceType: sourceType, SourceID: sourceID, Status: "active", CreatedAt: now}, nil
}

func ensureCouponUseAvailableTx(ctx context.Context, tx *sql.Tx, coupon coupons.Coupon, userID string) error {
	var globalCount, userCount int64
	query := `SELECT
		(SELECT COUNT(*) FROM coupon_uses WHERE coupon_id=?) +
		(SELECT COUNT(*) FROM coupon_redemptions r JOIN coupon_definitions d ON d.id=r.coupon_id WHERE r.coupon_id=? AND d.kind IN ('balance_add','balance_multiply')),
		(SELECT COUNT(*) FROM coupon_uses WHERE coupon_id=? AND user_id=?) +
		(SELECT COUNT(*) FROM coupon_redemptions r JOIN coupon_definitions d ON d.id=r.coupon_id WHERE r.coupon_id=? AND r.user_id=? AND d.kind IN ('balance_add','balance_multiply'))`
	if err := tx.QueryRowContext(ctx, query, coupon.ID, coupon.ID, coupon.ID, userID, coupon.ID, userID).Scan(&globalCount, &userCount); err != nil {
		return fmt.Errorf("count coupon uses: %w", err)
	}
	if coupon.GlobalUseLimit != nil && globalCount >= *coupon.GlobalUseLimit {
		return ErrConflict
	}
	if coupon.PerUserUseLimit != nil && userCount >= *coupon.PerUserUseLimit {
		return ErrConflict
	}
	return nil
}

func couponAvailable(coupon coupons.Coupon, now time.Time) error {
	if !coupon.Active || (coupon.ExpiresAt != nil && !coupon.ExpiresAt.After(now)) {
		return ErrConflict
	}
	return nil
}

func couponByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, couponID string) (coupons.Coupon, error) {
	return scanCoupon(queryer.QueryRowContext(ctx, couponSelect+` WHERE id=?`, couponID))
}

func couponByCodeTx(ctx context.Context, tx *sql.Tx, code string) (coupons.Coupon, error) {
	// Draw rewards are private wallet grants, never shareable redemption codes.
	return scanCoupon(tx.QueryRowContext(ctx, couponSelect+` WHERE code=? AND admin_visible=1`, code))
}
