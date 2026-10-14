package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

const couponSelect = `SELECT id,code,name,kind,discount_mode,value_minor_or_bps,percent_cap_minor,
	eligible_combo_ids,eligible_squad_ids,expires_at,global_use_limit,per_user_use_limit,active,created_at,updated_at,
	(SELECT COUNT(*) FROM coupon_uses WHERE coupon_id=coupon_definitions.id) +
	(SELECT COUNT(*) FROM coupon_redemptions WHERE coupon_id=coupon_definitions.id AND balance_delta_minor<>0)
	FROM coupon_definitions`

// SaveCoupon creates or updates a canonical coupon definition.

func (s *Store) SaveCoupon(ctx context.Context, input coupons.CouponInput, now time.Time) (coupons.Coupon, error) {
	normalized, err := input.Normalize()
	if err != nil {
		return coupons.Coupon{}, err
	}
	now = now.UTC()
	comboJSON, err := json.Marshal(normalized.EligibleComboIDs)
	if err != nil {
		return coupons.Coupon{}, fmt.Errorf("encode coupon combo eligibility: %w", err)
	}
	squadJSON, err := json.Marshal(normalized.EligibleSquadIDs)
	if err != nil {
		return coupons.Coupon{}, fmt.Errorf("encode coupon squad eligibility: %w", err)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if normalized.ID == "" {
		normalized.ID, err = ids.New()
		if err != nil {
			return coupons.Coupon{}, err
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO coupon_definitions(id,code,name,kind,discount_mode,value_minor_or_bps,percent_cap_minor,
			eligible_combo_ids,eligible_squad_ids,expires_at,global_use_limit,per_user_use_limit,active,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, normalized.ID, normalized.Code, normalized.Name, normalized.Kind, normalized.DiscountMode,
			normalized.ValueMinorOrBPS, nullableInt64(normalized.PercentCapMinor), string(comboJSON), string(squadJSON), nullableStamp(normalized.ExpiresAt),
			nullableInt64(normalized.GlobalUseLimit), nullableInt64(normalized.PerUserUseLimit), boolInt(normalized.Active), stamp(now), stamp(now))
	} else {
		var result sql.Result
		result, err = s.db.ExecContext(ctx, `UPDATE coupon_definitions SET code=?,name=?,kind=?,discount_mode=?,value_minor_or_bps=?,percent_cap_minor=?,
			eligible_combo_ids=?,eligible_squad_ids=?,expires_at=?,global_use_limit=?,per_user_use_limit=?,active=?,updated_at=? WHERE id=?`,
			normalized.Code, normalized.Name, normalized.Kind, normalized.DiscountMode, normalized.ValueMinorOrBPS, nullableInt64(normalized.PercentCapMinor),
			string(comboJSON), string(squadJSON), nullableStamp(normalized.ExpiresAt), nullableInt64(normalized.GlobalUseLimit), nullableInt64(normalized.PerUserUseLimit),
			boolInt(normalized.Active), stamp(now), normalized.ID)
		if err == nil {
			if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
				return coupons.Coupon{}, fmt.Errorf("inspect coupon update: %w", rowsErr)
			} else if affected == 0 {
				return coupons.Coupon{}, ErrNotFound
			}
		}
	}
	if err != nil {
		if isUniqueConstraint(err) {
			return coupons.Coupon{}, ErrConflict
		}
		return coupons.Coupon{}, fmt.Errorf("save coupon: %w", err)
	}
	return couponByID(ctx, s.db, normalized.ID)
}

// ListCoupons returns coupon definitions newest first.

func (s *Store) ListCoupons(ctx context.Context, activeOnly bool) ([]coupons.Coupon, error) {
	query := couponSelect
	args := make([]any, 0, 1)
	if activeOnly {
		query += ` WHERE admin_visible=1 AND active=1 AND (expires_at IS NULL OR expires_at>?)`
		args = append(args, stamp(time.Now().UTC()))
	} else {
		query += ` WHERE admin_visible=1`
	}
	query += ` ORDER BY created_at DESC,id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list coupons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]coupons.Coupon, 0)
	for rows.Next() {
		coupon, scanErr := scanCoupon(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, coupon)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate coupons: %w", err)
	}
	return result, nil
}

// RedeemCoupon atomically creates a wallet grant or applies an immediate balance effect.
