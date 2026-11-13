package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

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
