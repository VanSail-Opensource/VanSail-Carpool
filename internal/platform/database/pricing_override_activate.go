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
		if err = tx.QueryRowContext(ctx, `SELECT price_minor FROM pricing_published_prices WHERE id=? AND product_type=? AND product_id=? AND source<>'MANUAL' AND valid_until>?`, in.PriceVersion, p.ProductType, p.ProductID, pricingStamp(now)).Scan(&price); err != nil {
			return err
		}
		bounded, e := pricing.BoundPrice(p.FixedPrice, price, p.MaxIncreaseBPS, p.MaxDiscountBPS)
		if e != nil || bounded != price {
			return ErrConflict
		}
		priceVersion = in.PriceVersion
	}
	result, err := tx.ExecContext(ctx, `UPDATE pricing_product_settings SET dynamic_enabled=?,purchase_pricing_mode=?,renewal_pricing_mode=?,override_status=?,last_approved_price_version=?,version=version+1,updated_by=?,updated_at=? WHERE product_type=? AND product_id=? AND version=?`, boolInt(enabled), purchase, renew, status, priceVersion, actor, pricingStamp(now), p.ProductType, p.ProductID, p.Version)
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
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_published_prices(id,product_type,product_id,price_minor,policy_version,model_version,source,valid_from,valid_until,created_by,created_at,reason) VALUES(?,?,?,?,?,'manual-v1','MANUAL',?,?,?,?,?)`, in.ID, p.ProductType, p.ProductID, in.Price, in.ExpectedVersion, pricingStamp(now), pricingStamp(in.ExpiresAt), actor, pricingStamp(now), in.Reason)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE pricing_admin_overrides SET expected_version=? WHERE id=?`, in.ExpectedVersion, in.ID)
	return err
}
