package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

const grantSelect = `SELECT coupon_grants.id,coupon_grants.user_id,coupon_grants.source_type,coupon_grants.source_id,coupon_grants.status,
	coupon_grants.use_count,coupon_grants.created_at,coupon_grants.consumed_at,
	coupon_definitions.id,coupon_definitions.code,coupon_definitions.name,coupon_definitions.kind,coupon_definitions.discount_mode,
	coupon_definitions.value_minor_or_bps,coupon_definitions.percent_cap_minor,coupon_definitions.eligible_combo_ids,
	coupon_definitions.eligible_squad_ids,coupon_definitions.expires_at,coupon_definitions.global_use_limit,
	coupon_definitions.per_user_use_limit,coupon_definitions.active,coupon_definitions.created_at,coupon_definitions.updated_at
	FROM coupon_grants JOIN coupon_definitions ON coupon_definitions.id=coupon_grants.coupon_id`

// Reject copies redeemed before private draw codes were blocked. Preserve their
// historical records, but never use them for checkout or attached renewal pricing.
const authorizedCouponGrant = `(coupon_definitions.admin_visible=1 OR coupon_grants.source_type<>'code')`

func grantByIDTx(ctx context.Context, tx *sql.Tx, grantID string) (coupons.Grant, error) {
	return scanGrant(tx.QueryRowContext(ctx, grantSelect+` WHERE coupon_grants.id=? AND `+authorizedCouponGrant, grantID))
}

func grantBySourceTx(ctx context.Context, tx *sql.Tx, userID, couponID, sourceType, sourceID string) (coupons.Grant, error) {
	return scanGrant(tx.QueryRowContext(ctx, grantSelect+` WHERE coupon_grants.user_id=? AND coupon_grants.coupon_id=? AND coupon_grants.source_type=? AND coupon_grants.source_id=?`, userID, couponID, sourceType, sourceID))
}

func grantDiscardedTx(ctx context.Context, tx *sql.Tx, grantID string) (bool, error) {
	var matched int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM coupon_grant_discards WHERE grant_id=?`, grantID).Scan(&matched)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load coupon discard: %w", err)
	}
	return true, nil
}

func scanGrant(row rowScanner) (coupons.Grant, error) {
	var grant coupons.Grant
	var consumed sql.NullString
	var grantCreated string
	var percentCap, globalLimit, userLimit sql.NullInt64
	var comboJSON, squadJSON string
	var expires sql.NullString
	var active int
	var couponCreated, couponUpdated string
	if err := row.Scan(&grant.ID, &grant.UserID, &grant.SourceType, &grant.SourceID, &grant.Status, &grant.UseCount, &grantCreated, &consumed,
		&grant.Coupon.ID, &grant.Coupon.Code, &grant.Coupon.Name, &grant.Coupon.Kind, &grant.Coupon.DiscountMode,
		&grant.Coupon.ValueMinorOrBPS, &percentCap, &comboJSON, &squadJSON, &expires, &globalLimit, &userLimit, &active, &couponCreated, &couponUpdated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coupons.Grant{}, ErrNotFound
		}
		return coupons.Grant{}, fmt.Errorf("scan coupon grant: %w", err)
	}
	grant.Coupon.PercentCapMinor = int64Pointer(percentCap)
	grant.Coupon.GlobalUseLimit = int64Pointer(globalLimit)
	grant.Coupon.PerUserUseLimit = int64Pointer(userLimit)
	grant.Coupon.Active = active == 1
	if err := json.Unmarshal([]byte(comboJSON), &grant.Coupon.EligibleComboIDs); err != nil {
		return coupons.Grant{}, err
	}
	if err := json.Unmarshal([]byte(squadJSON), &grant.Coupon.EligibleSquadIDs); err != nil {
		return coupons.Grant{}, err
	}
	var err error
	if grant.CreatedAt, err = parseStamp(grantCreated); err != nil {
		return coupons.Grant{}, err
	}
	if consumed.Valid {
		value, parseErr := parseStamp(consumed.String)
		if parseErr != nil {
			return coupons.Grant{}, parseErr
		}
		grant.ConsumedAt = &value
	}
	if expires.Valid {
		value, parseErr := parseStamp(expires.String)
		if parseErr != nil {
			return coupons.Grant{}, parseErr
		}
		grant.Coupon.ExpiresAt = &value
	}
	if grant.Coupon.CreatedAt, err = parseStamp(couponCreated); err != nil {
		return coupons.Grant{}, err
	}
	grant.Coupon.UpdatedAt, err = parseStamp(couponUpdated)
	return grant, err
}

func (s *Store) redemptionByID(ctx context.Context, redemptionID string) (coupons.RedemptionResult, error) {
	result, err := scanRedemption(s.db.QueryRowContext(ctx, redemptionSelect+` WHERE coupon_redemptions.id=?`, redemptionID))
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	if result.Grant != nil {
		grant, grantErr := scanGrant(s.db.QueryRowContext(ctx, grantSelect+` WHERE coupon_grants.id=?`, result.Grant.ID))
		if grantErr != nil {
			return coupons.RedemptionResult{}, grantErr
		}
		result.Grant = &grant
	}
	return result, nil
}

const redemptionSelect = `SELECT coupon_redemptions.id,coupon_redemptions.balance_delta_minor,coupon_redemptions.balance_after_minor,
	coupon_redemptions.idempotency_key,coupon_redemptions.created_at,coupon_redemptions.grant_id,
	coupon_definitions.id,coupon_definitions.code,coupon_definitions.name,coupon_definitions.kind,coupon_definitions.discount_mode,
	coupon_definitions.value_minor_or_bps,coupon_definitions.percent_cap_minor,coupon_definitions.eligible_combo_ids,
	coupon_definitions.eligible_squad_ids,coupon_definitions.expires_at,coupon_definitions.global_use_limit,
	coupon_definitions.per_user_use_limit,coupon_definitions.active,coupon_definitions.created_at,coupon_definitions.updated_at
	FROM coupon_redemptions JOIN coupon_definitions ON coupon_definitions.id=coupon_redemptions.coupon_id`

func redemptionByKeyTx(ctx context.Context, tx *sql.Tx, userID, key string) (coupons.RedemptionResult, error) {
	result, err := scanRedemption(tx.QueryRowContext(ctx, redemptionSelect+` WHERE coupon_redemptions.user_id=? AND coupon_redemptions.idempotency_key=?`, userID, key))
	if err != nil {
		return coupons.RedemptionResult{}, err
	}
	if result.Grant != nil {
		grant, grantErr := grantByIDTx(ctx, tx, result.Grant.ID)
		if grantErr != nil {
			return coupons.RedemptionResult{}, grantErr
		}
		result.Grant = &grant
	}
	return result, nil
}

func scanRedemption(row rowScanner) (coupons.RedemptionResult, error) {
	var result coupons.RedemptionResult
	var grantID sql.NullString
	var created string
	var percentCap, globalLimit, userLimit sql.NullInt64
	var comboJSON, squadJSON string
	var expires sql.NullString
	var active int
	var couponCreated, couponUpdated string
	if err := row.Scan(&result.ID, &result.BalanceDeltaMinor, &result.BalanceAfterMinor, &result.IdempotencyKey, &created, &grantID,
		&result.Coupon.ID, &result.Coupon.Code, &result.Coupon.Name, &result.Coupon.Kind, &result.Coupon.DiscountMode,
		&result.Coupon.ValueMinorOrBPS, &percentCap, &comboJSON, &squadJSON, &expires, &globalLimit, &userLimit, &active, &couponCreated, &couponUpdated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coupons.RedemptionResult{}, ErrNotFound
		}
		return coupons.RedemptionResult{}, fmt.Errorf("scan coupon redemption: %w", err)
	}
	result.Coupon.PercentCapMinor = int64Pointer(percentCap)
	result.Coupon.GlobalUseLimit = int64Pointer(globalLimit)
	result.Coupon.PerUserUseLimit = int64Pointer(userLimit)
	result.Coupon.Active = active == 1
	if grantID.Valid {
		result.Grant = &coupons.Grant{ID: grantID.String}
	}
	if err := json.Unmarshal([]byte(comboJSON), &result.Coupon.EligibleComboIDs); err != nil {
		return coupons.RedemptionResult{}, err
	}
	if err := json.Unmarshal([]byte(squadJSON), &result.Coupon.EligibleSquadIDs); err != nil {
		return coupons.RedemptionResult{}, err
	}
	var err error
	if result.CreatedAt, err = parseStamp(created); err != nil {
		return coupons.RedemptionResult{}, err
	}
	if expires.Valid {
		value, parseErr := parseStamp(expires.String)
		if parseErr != nil {
			return coupons.RedemptionResult{}, parseErr
		}
		result.Coupon.ExpiresAt = &value
	}
	if result.Coupon.CreatedAt, err = parseStamp(couponCreated); err != nil {
		return coupons.RedemptionResult{}, err
	}
	result.Coupon.UpdatedAt, err = parseStamp(couponUpdated)
	return result, err
}
