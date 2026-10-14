package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func scanCoupon(row rowScanner) (coupons.Coupon, error) {
	var coupon coupons.Coupon
	var percentCap, globalLimit, userLimit sql.NullInt64
	var comboJSON, squadJSON string
	var expires sql.NullString
	var active int
	var created, updated string
	if err := row.Scan(&coupon.ID, &coupon.Code, &coupon.Name, &coupon.Kind, &coupon.DiscountMode, &coupon.ValueMinorOrBPS,
		&percentCap, &comboJSON, &squadJSON, &expires, &globalLimit, &userLimit, &active, &created, &updated, &coupon.UsageCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coupons.Coupon{}, ErrNotFound
		}
		return coupons.Coupon{}, fmt.Errorf("scan coupon: %w", err)
	}
	coupon.Active = active == 1
	coupon.PercentCapMinor = int64Pointer(percentCap)
	coupon.GlobalUseLimit = int64Pointer(globalLimit)
	coupon.PerUserUseLimit = int64Pointer(userLimit)
	if err := json.Unmarshal([]byte(comboJSON), &coupon.EligibleComboIDs); err != nil {
		return coupons.Coupon{}, fmt.Errorf("decode coupon combo eligibility: %w", err)
	}
	if err := json.Unmarshal([]byte(squadJSON), &coupon.EligibleSquadIDs); err != nil {
		return coupons.Coupon{}, fmt.Errorf("decode coupon squad eligibility: %w", err)
	}
	var err error
	if expires.Valid {
		value, parseErr := parseStamp(expires.String)
		if parseErr != nil {
			return coupons.Coupon{}, parseErr
		}
		coupon.ExpiresAt = &value
	}
	if coupon.CreatedAt, err = parseStamp(created); err != nil {
		return coupons.Coupon{}, err
	}
	coupon.UpdatedAt, err = parseStamp(updated)
	return coupon, err
}
