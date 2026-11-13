package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func saveCouponTx(ctx context.Context, tx *sql.Tx, input coupons.CouponInput, now time.Time) (coupons.Coupon, error) {
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
	if normalized.ID == "" {
		normalized.ID, err = ids.New()
		if err != nil {
			return coupons.Coupon{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO coupon_definitions(id,code,name,kind,discount_mode,value_minor_or_bps,percent_cap_minor,
			eligible_combo_ids,eligible_squad_ids,expires_at,global_use_limit,per_user_use_limit,active,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, normalized.ID, normalized.Code, normalized.Name, normalized.Kind, normalized.DiscountMode,
			normalized.ValueMinorOrBPS, nullableInt64(normalized.PercentCapMinor), string(comboJSON), string(squadJSON), nullableStamp(normalized.ExpiresAt),
			nullableInt64(normalized.GlobalUseLimit), nullableInt64(normalized.PerUserUseLimit), boolInt(normalized.Active), stamp(now), stamp(now))
	} else {
		previous, loadErr := scanCoupon(tx.QueryRowContext(ctx, couponSelect+` WHERE id=$1 FOR UPDATE`, normalized.ID))
		if loadErr != nil {
			return coupons.Coupon{}, loadErr
		}
		if !now.After(previous.UpdatedAt) {
			now = previous.UpdatedAt.Add(time.Nanosecond)
		}
		_, err = tx.ExecContext(ctx, `UPDATE coupon_definitions SET code=$1,name=$2,kind=$3,discount_mode=$4,value_minor_or_bps=$5,percent_cap_minor=$6,
			eligible_combo_ids=$7,eligible_squad_ids=$8,expires_at=$9,global_use_limit=$10,per_user_use_limit=$11,active=$12,updated_at=$13 WHERE id=$14`,
			normalized.Code, normalized.Name, normalized.Kind, normalized.DiscountMode, normalized.ValueMinorOrBPS, nullableInt64(normalized.PercentCapMinor),
			string(comboJSON), string(squadJSON), nullableStamp(normalized.ExpiresAt), nullableInt64(normalized.GlobalUseLimit), nullableInt64(normalized.PerUserUseLimit),
			boolInt(normalized.Active), stamp(now), normalized.ID)
	}
	if err != nil {
		if isUniqueConstraint(err) {
			return coupons.Coupon{}, ErrConflict
		}
		return coupons.Coupon{}, fmt.Errorf("save coupon: %w", err)
	}
	return couponByID(ctx, tx, normalized.ID)
}
