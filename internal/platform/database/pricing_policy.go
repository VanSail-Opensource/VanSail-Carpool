package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const pricingPolicySelect = `SELECT product_type,product_id,dynamic_enabled,purchase_pricing_mode,renewal_pricing_mode,personal_discounts_enabled,fixed_price_minor,max_increase_bps,max_discount_bps,override_status,last_approved_price_version,version,updated_by,updated_at FROM pricing_product_settings`

func pricingReference(ctx context.Context, q pricingReader, kind, id string) (int64, error) {
	var price int64
	var query string
	switch kind {
	case "combo":
		query = `SELECT price_txb_minor FROM combos WHERE id=?`
	case "addon":
		query = `SELECT price_txb_minor FROM squad_product_overrides WHERE group_id=? AND billing_mode='combo'`
	case "payg":
		query = `SELECT payg_price_minor FROM squad_product_overrides WHERE group_id=? AND billing_mode='payg'`
	default:
		return 0, ErrNotFound
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&price)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return price, err
}

func loadPricingPolicy(ctx context.Context, q pricingReader, kind, id string, fixed int64) (pricing.Policy, error) {
	p := pricing.DefaultPolicy(kind, id, fixed)
	var enabled, personal int
	var updated string
	err := q.QueryRowContext(ctx, pricingPolicySelect+` WHERE product_type=? AND product_id=?`, kind, id).Scan(&p.ProductType, &p.ProductID, &enabled, &p.PurchaseMode, &p.RenewalMode, &personal, &p.FixedPrice, &p.MaxIncreaseBPS, &p.MaxDiscountBPS, &p.OverrideStatus, &p.LastApprovedVersion, &p.Version, &p.UpdatedBy, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return pricing.DefaultPolicy(kind, id, fixed), nil
	}
	if err != nil {
		return p, err
	}
	p.DynamicEnabled = enabled == 1
	p.PersonalDiscounts = personal == 1
	p.UpdatedAt, err = parseStamp(updated)
	return p, err
}

// PricingPolicy reads an existing product's policy; missing settings are fixed.
func (s *Store) PricingPolicy(ctx context.Context, kind, id string) (pricing.Policy, error) {
	fixed, err := pricingReference(ctx, s.db, kind, id)
	if err != nil {
		return pricing.Policy{}, err
	}
	return loadPricingPolicy(ctx, s.db, kind, id, fixed)
}

func pricingAdminTx(ctx context.Context, tx *sql.Tx, actor string) error {
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, actor).Scan(&role); err != nil {
		return err
	}
	if role != "admin" {
		return ErrConflict
	}
	return nil
}

func pricingAuditTx(ctx context.Context, tx *sql.Tx, actor, action, target, reason string, value any, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"reason": reason, "value": value})
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &actor, "pricing."+action, "pricing", target, string(raw), now)
}

// SavePricingPolicy uses a version precondition and audits activation atomically.
func (s *Store) retryAttempt_SavePricingPolicy(ctx context.Context, actor, reason string, p pricing.Policy, now time.Time) (pricing.Policy, error) {
	// Bounds are computed by the evidence-gated optimizer, never administrator input.
	p.MaxIncreaseBPS, p.MaxDiscountBPS = 0, 0
	if err := p.Validate(); err != nil {
		return p, err
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 1000 || p.OverrideStatus == "MANUAL" {
		return p, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	var serial int
	if err = tx.QueryRowContext(ctx, `SELECT singleton FROM pricing_worker_watermark WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return p, err
	}
	if err = pricingAdminTx(ctx, tx, actor); err != nil {
		return p, err
	}
	fixed, err := pricingReference(ctx, tx, p.ProductType, p.ProductID)
	if err != nil {
		return p, err
	}
	old, err := loadPricingPolicy(ctx, tx, p.ProductType, p.ProductID, fixed)
	if err != nil {
		return p, err
	}
	if old.Version != p.Version {
		return p, pricing.VersionConflict
	}
	p.FixedPrice = fixed
	if fixed == old.FixedPrice {
		p.MaxIncreaseBPS, p.MaxDiscountBPS = old.MaxIncreaseBPS, old.MaxDiscountBPS
	}
	p.LastApprovedVersion = old.LastApprovedVersion
	p.Version++
	p.UpdatedBy = actor
	p.UpdatedAt = now.UTC()
	if p.LastApprovedVersion == "" {
		p.LastApprovedVersion, err = ids.New()
		if err != nil {
			return p, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pricing_published_prices(id,product_type,product_id,price_minor,policy_version,model_version,source,valid_from,valid_until,created_by,created_at,reason) VALUES(?,?,?,?,?,?,'FIXED',?,?,?,?,?)`, p.LastApprovedVersion, p.ProductType, p.ProductID, fixed, p.Version, "fixed-v1", pricingStamp(now), pricingStamp(now.AddDate(1, 0, 0)), actor, pricingStamp(now), reason)
		if err != nil {
			return p, err
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO pricing_product_settings(product_type,product_id,dynamic_enabled,purchase_pricing_mode,renewal_pricing_mode,personal_discounts_enabled,fixed_price_minor,max_increase_bps,max_discount_bps,override_status,last_approved_price_version,version,updated_by,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(product_type,product_id) DO UPDATE SET dynamic_enabled=excluded.dynamic_enabled,purchase_pricing_mode=excluded.purchase_pricing_mode,renewal_pricing_mode=excluded.renewal_pricing_mode,personal_discounts_enabled=excluded.personal_discounts_enabled,fixed_price_minor=excluded.fixed_price_minor,max_increase_bps=excluded.max_increase_bps,max_discount_bps=excluded.max_discount_bps,override_status=excluded.override_status,last_approved_price_version=excluded.last_approved_price_version,version=excluded.version,updated_by=excluded.updated_by,updated_at=excluded.updated_at WHERE pricing_product_settings.version=?`, p.ProductType, p.ProductID, boolInt(p.DynamicEnabled), p.PurchaseMode, p.RenewalMode, boolInt(p.PersonalDiscounts), fixed, p.MaxIncreaseBPS, p.MaxDiscountBPS, p.OverrideStatus, p.LastApprovedVersion, p.Version, actor, pricingStamp(now), old.Version)
	if err != nil {
		return p, fmt.Errorf("save pricing policy: %w", err)
	}
	if n, e := result.RowsAffected(); e != nil {
		return p, e
	} else if n != 1 {
		return p, pricing.VersionConflict
	}
	if err = pricingAuditTx(ctx, tx, actor, "policy", p.ProductType+":"+p.ProductID, reason, p, now); err != nil {
		return p, err
	}
	if err = pricingChangeNoticeTx(ctx, tx, actor, "POLICY", p.ProductType+":"+p.ProductID, reason, now); err != nil {
		return p, err
	}
	if err = tx.Commit(); err == nil {
		s.pricingCache.Clear()
		s.pricingExposureCache.Clear()
	}
	return p, err
}
