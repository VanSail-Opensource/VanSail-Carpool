package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func personalDecisionTx(ctx context.Context, tx *sql.Tx, version string, now time.Time) (model.PricingDecision, error) {
	var d model.PricingDecision
	var public, personal, net int64
	var at, until, couponRaw string
	var probability sql.NullFloat64
	err := tx.QueryRowContext(ctx, `SELECT version,status,reason,public_minor,personal_minor,net_minor,probability,model_version,calibration,generated_at,valid_until,coupon_json FROM pricing_personal_decisions WHERE version=? AND valid_until>?`, version, pricingStamp(now)).Scan(&d.Version, &d.Status, &d.Reason, &public, &personal, &net, &probability, &d.ModelVersion, &d.Calibration, &at, &until, &couponRaw)
	if err != nil {
		return d, err
	}
	d.CouponFingerprint = compute.Digest([]byte(couponRaw))
	d.PublicPrice, d.PersonalPrice, d.NetPrice = model.TXBMoney(public), model.TXBMoney(personal), model.TXBMoney(net)
	if probability.Valid {
		d.Probability = &probability.Float64
	}
	d.GeneratedAt, err = parseStamp(at)
	if err == nil {
		d.ValidUntil, err = parseStamp(until)
	}
	return d, err
}

func personalProductPriceTx(ctx context.Context, tx *sql.Tx, user, kind, product, combo string, squads []string, now time.Time, required bool) (int64, *model.PricingDecision, error) {
	in, err := personalPriceSnapshotTx(ctx, tx, user, kind, product, combo, squads, now)
	if err != nil {
		return 0, nil, err
	}
	if in.Unavailable != "" || required && in.GlobalPending {
		return 0, nil, ErrPricingDecisionPending
	}
	d, err := personalDecisionTx(ctx, tx, in.Version, now)
	if err == sql.ErrNoRows {
		if required {
			return 0, nil, ErrPricingDecisionPending
		}
		return in.Public, &model.PricingDecision{CouponFingerprint: couponFingerprint(in.Coupons), Version: in.Version, Status: "PENDING", Reason: "CALCULATION_PENDING", PublicPrice: model.TXBMoney(in.Public), PersonalPrice: model.TXBMoney(in.Public), NetPrice: model.TXBMoney(in.Public), GeneratedAt: now, ValidUntil: in.Until}, nil
	}
	if err != nil {
		return 0, nil, err
	}
	d.DynamicEnabled = in.Policy.DynamicEnabled
	return moneyMinor(d.PersonalPrice), &d, nil
}

func moneyMinor(m model.Money) int64 {
	value, _ := strconv.ParseInt(m.Minor, 10, 64)
	return value
}
