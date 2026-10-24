package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
)

func (s *Store) retryAttempt_RedeemCoupon(ctx context.Context, userID, code, idempotencyKey string, now time.Time) (coupons.RedemptionResult, error) {
	canonical, err := coupons.CanonicalCode(code)
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 128 {
		return coupons.RedemptionResult{}, coupons.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coupons.RedemptionResult{}, fmt.Errorf("begin coupon redemption: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := redemptionByKeyTx(ctx, tx, userID, idempotencyKey); loadErr == nil {
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return coupons.RedemptionResult{}, loadErr
	}
	coupon, err := couponByCodeTx(ctx, tx, canonical)
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	if err := couponAvailable(coupon, now); err != nil {
		return coupons.RedemptionResult{}, err
	}
	redemptionID, err := ids.New()
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	balance, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	var grantID *string
	delta := int64(0)
	switch coupon.Kind {
	case coupons.KindPurchaseRecurring, coupons.KindPurchaseOnce:
		grant, grantErr := grantCouponTx(ctx, tx, userID, coupon, "code", coupon.Code, now)
		if grantErr != nil {
			return coupons.RedemptionResult{}, grantErr
		}
		grantID = &grant.ID
	case coupons.KindBalanceAdd, coupons.KindBalanceMultiply:
		if err := ensureCouponUseAvailableTx(ctx, tx, coupon, userID); err != nil {
			return coupons.RedemptionResult{}, err
		}
		if coupon.Kind == coupons.KindBalanceAdd {
			delta = coupon.ValueMinorOrBPS
		} else {
			delta, err = coupons.CalculateBalanceMultiplyCredit(balance, coupon.ValueMinorOrBPS)
			if err != nil {
				return coupons.RedemptionResult{}, err
			}
		}
		balance, err = changeBalanceTx(ctx, tx, userID, delta, now)
		if err != nil {
			return coupons.RedemptionResult{}, err
		}
		ledgerKind := "coupon_balance_add"
		if coupon.Kind == coupons.KindBalanceMultiply {
			ledgerKind = "coupon_balance_multiply"
		}
		if _, err := insertLedgerTx(ctx, tx, userID, delta, balance, ledgerKind, redemptionID, coupon.Code, now); err != nil {
			return coupons.RedemptionResult{}, err
		}
	default:
		return coupons.RedemptionResult{}, coupons.ErrInvalidInput
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO coupon_redemptions(id,coupon_id,grant_id,user_id,balance_delta_minor,balance_after_minor,idempotency_key,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, redemptionID, coupon.ID, nullableStringValue(grantID), userID, delta, balance, idempotencyKey, stamp(now))
	if err != nil {
		return coupons.RedemptionResult{}, fmt.Errorf("record coupon redemption: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return coupons.RedemptionResult{}, fmt.Errorf("commit coupon redemption: %w", err)
	}
	return s.redemptionByID(ctx, redemptionID)
}

// GrantCoupon idempotently puts a purchase-discount coupon in a member wallet.

func (s *Store) retryAttempt_GrantCoupon(ctx context.Context, userID, couponID, sourceType, sourceID string, now time.Time) (coupons.Grant, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(couponID) == "" || strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceID) == "" {
		return coupons.Grant{}, coupons.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coupons.Grant{}, err
	}
	defer func() { _ = tx.Rollback() }()
	coupon, err := couponByID(ctx, tx, couponID)
	if err != nil {
		return coupons.Grant{}, err
	}
	if err := couponAvailable(coupon, now); err != nil {
		return coupons.Grant{}, err
	}
	grant, err := grantCouponTx(ctx, tx, userID, coupon, sourceType, sourceID, now.UTC())
	if err != nil {
		return coupons.Grant{}, err
	}
	if err := tx.Commit(); err != nil {
		return coupons.Grant{}, err
	}
	return grant, nil
}

// ListCouponGrants returns currently usable purchase-discount grants.

func (s *Store) ListCouponGrants(ctx context.Context, userID string, now time.Time) ([]coupons.Grant, error) {
	if _, err := s.db.ExecContext(ctx, `UPDATE coupon_grants SET status='expired' WHERE user_id=? AND status='active'
		AND coupon_id IN (SELECT id FROM coupon_definitions WHERE expires_at IS NOT NULL AND expires_at<=?)
		AND NOT EXISTS (SELECT 1 FROM coupon_grant_discards WHERE coupon_grant_discards.grant_id=coupon_grants.id)`, userID, stamp(now)); err != nil {
		return nil, fmt.Errorf("expire coupon grants: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, grantSelect+` WHERE coupon_grants.user_id=? AND coupon_grants.status='active'
		AND coupon_definitions.active=1 AND (coupon_definitions.expires_at IS NULL OR coupon_definitions.expires_at>?)
		AND NOT EXISTS (SELECT 1 FROM coupon_grant_discards WHERE coupon_grant_discards.grant_id=coupon_grants.id)
		AND `+authorizedCouponGrant+` ORDER BY coupon_grants.created_at DESC`, userID, stamp(now))
	if err != nil {
		return nil, fmt.Errorf("list coupon grants: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]coupons.Grant, 0)
	for rows.Next() {
		grant, scanErr := scanGrant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, grant)
	}
	return result, rows.Err()
}

// DiscardCouponGrant hides one active grant while preserving its immutable
// redemption and purchase links. Replaying a member's own discard is a no-op.
