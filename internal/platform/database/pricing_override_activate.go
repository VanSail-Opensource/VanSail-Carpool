package database

import (
	"context"
	"database/sql"

	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func activatePricingOverrideTx(ctx context.Context, tx *sql.Tx, actor string, in *PricingIntervention, now time.Time) error {
	if err := pricingCommandEffectTx(ctx, tx, *in, actor, now); err != nil {
		return err
	}
	if in.ScopeType != "product" {
		return nil
	}
	parts := strings.SplitN(in.ScopeID, ":", 2)
	p, err := loadPricingPolicy(ctx, tx, parts[0], parts[1], 0)
	if err != nil {
		return err
	}
	if p.Version != in.ExpectedVersion {
		return pricing.VersionConflict
	}
	status := p.OverrideStatus
	if in.Command == "FREEZE" {
		status = "FROZEN"
	}
	if in.Command == "MANUAL_RATE" {
		status = "MANUAL"
	}
	enabled, purchase, renew := p.DynamicEnabled, p.PurchaseMode, p.RenewalMode
	if in.Command == "DISABLE" {
		enabled = false
		purchase = "FIXED"
		renew = "FIXED"
		status = "NONE"
	}
	priceVersion := p.LastApprovedVersion
	if in.Command == "RESTORE_PRICE" {
		var price int64
		if err = tx.QueryRowContext(ctx, `SELECT price_minor FROM pricing_published_prices WHERE id=$1 AND product_type=$2 AND product_id=$3 AND source<>'MANUAL' AND valid_until>$4`, in.PriceVersion, p.ProductType, p.ProductID, pricingStamp(now)).Scan(&price); err != nil {
			return err
		}
		bounded, e := pricing.BoundPrice(p.FixedPrice, price, p.MaxIncreaseBPS, p.MaxDiscountBPS)
		if e != nil || bounded != price {
			return ErrConflict
		}
		priceVersion = in.PriceVersion
	}
	result, err := tx.ExecContext(ctx, `UPDATE pricing_product_settings SET dynamic_enabled=$1,purchase_pricing_mode=$2,renewal_pricing_mode=$3,override_status=$4,last_approved_price_version=$5,version=version+1,updated_by=$6,updated_at=$7 WHERE product_type=$8 AND product_id=$9 AND version=$10`, boolInt(enabled), purchase, renew, status, priceVersion, actor, pricingStamp(now), p.ProductType, p.ProductID, p.Version)
	if err != nil {
		return err
	}
	if n, e := result.RowsAffected(); e != nil {
		return e
	} else if n != 1 {
		return pricing.VersionConflict
	}
	in.ExpectedVersion++
	if in.Command == "MANUAL_RATE" {
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_published_prices(id,product_type,product_id,price_minor,policy_version,model_version,source,valid_from,valid_until,created_by,created_at,reason) VALUES($1,$2,$3,$4,$5,'manual-v1','MANUAL',$6,$7,$8,$9,$10)`, in.ID, p.ProductType, p.ProductID, in.Price, in.ExpectedVersion, pricingStamp(now), pricingStamp(in.ExpiresAt), actor, pricingStamp(now), in.Reason)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE pricing_admin_overrides SET expected_version=$1 WHERE id=$2`, in.ExpectedVersion, in.ID)
	return err
}
