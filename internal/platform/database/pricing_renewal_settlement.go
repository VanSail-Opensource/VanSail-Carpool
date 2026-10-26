package database

import (
	"context"
	"database/sql"

	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingRenewalGrossTx(ctx context.Context, tx *sql.Tx, purchase string, gross int64, addons []model.SquadProduct, now time.Time) (int64, error) {
	c, err := pricingContractTx(ctx, tx, purchase)
	if errors.Is(err, sql.ErrNoRows) {
		return gross, nil
	}
	if err != nil {
		return 0, err
	}
	if c.Mode == "FIXED" {
		return currentPricingContractGross(c, addons), nil
	}
	q, err := scanPricingQuote(tx.QueryRowContext(ctx, pricingQuoteSelect+` WHERE contract_id=? AND state NOT IN ('CANCELLED','EXPIRED') ORDER BY generated_at DESC LIMIT 1`, c.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return c.Price, nil
	}
	if err != nil {
		return 0, err
	}
	if q.Price > c.Ceiling {
		return 0, pricing.RenewalQuoteUnavailable
	}
	return q.Price, nil
}

func validatePricingRenewalSettlementTx(ctx context.Context, tx *sql.Tx, purchase string, addons []model.SquadProduct, automatic bool, now time.Time) error {
	c, err := pricingContractTx(ctx, tx, purchase)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.Mode != "DYNAMIC" {
		return nil
	}
	if !pricingAddonTermsMatch(c, addons) {
		return pricing.RenewalHealthDeferred
	}
	q, err := scanPricingQuote(tx.QueryRowContext(ctx, pricingQuoteSelect+` WHERE contract_id=? AND state NOT IN ('CANCELLED','EXPIRED') ORDER BY generated_at DESC LIMIT 1`, c.ID))
	if errors.Is(err, sql.ErrNoRows) {
		p, e := loadPricingPolicy(ctx, tx, "combo", c.ProductID, c.Price)
		if e != nil {
			return e
		}
		if !p.DynamicEnabled || p.RenewalMode != "DYNAMIC" {
			return pricingOperatorAdmissionTx(ctx, tx, "combo", c.ProductID, now)
		}
		return pricing.RenewalQuoteUnavailable
	}
	if err != nil {
		return err
	}
	if !automatic && now.Before(q.SettleAfter) {
		q.SettleAfter = now
	}
	if err = q.Settleable(c, now); err != nil {
		return err
	}
	var held int
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pricing_admin_overrides WHERE command='HOLD_RENEWAL' AND scope_type='purchase' AND scope_id=? AND status='ACTIVE' AND starts_at<=? AND expires_at>?)`, purchase, pricingStamp(now), pricingStamp(now)).Scan(&held); err != nil {
		return err
	}
	if held != 0 {
		return pricing.RenewalHealthDeferred
	}
	if err = pricingAdmissionTx(ctx, tx, "combo", c.ProductID, now); err != nil {
		return pricing.RenewalHealthDeferred
	}
	return nil
}
