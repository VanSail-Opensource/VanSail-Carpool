package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

const pricingContractSelect = `SELECT id,purchase_id,user_id,product_id,mode,contractual_price_minor,price_limit_minor,max_change_bps,policy_version,recorded_at,version,source_purchase_id,addon_terms_json FROM pricing_contracts`

func scanPricingContract(row rowScanner) (renewal.Contract, error) {
	var c renewal.Contract
	var at, addons string
	err := row.Scan(&c.ID, &c.PurchaseID, &c.UserID, &c.ProductID, &c.Mode, &c.Price, &c.Ceiling, &c.MaxChangeBPS, &c.PolicyVersion, &at, &c.Version, &c.SourcePurchaseID, &addons)
	if err != nil {
		return c, err
	}
	c.RecordedAt, err = parseStamp(at)
	if err == nil {
		err = json.Unmarshal([]byte(addons), &c.Addons)
	}
	return c, err
}

func pricingContractTx(ctx context.Context, tx *sql.Tx, purchase string) (renewal.Contract, error) {
	return scanPricingContract(tx.QueryRowContext(ctx, pricingContractSelect+` WHERE purchase_id=? ORDER BY version DESC LIMIT 1`, purchase))
}

func insertPricingContractTx(ctx context.Context, tx *sql.Tx, c renewal.Contract) error {
	if err := c.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(c.Addons)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_contracts(id,purchase_id,user_id,product_id,mode,contractual_price_minor,price_limit_minor,max_change_bps,policy_version,recorded_at,version,source_purchase_id,addon_terms_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.PurchaseID, c.UserID, c.ProductID, c.Mode, c.Price, c.Ceiling, c.MaxChangeBPS, c.PolicyVersion, pricingStamp(c.RecordedAt), c.Version, c.SourcePurchaseID, string(raw))
	return err
}

// PricingContract returns only the owner's contract; no row means legacy terms.
func (s *Store) PricingContract(ctx context.Context, user, purchase string) (*renewal.Contract, error) {
	var owner string
	if err := s.db.QueryRowContext(ctx, `SELECT user_id FROM purchases WHERE id=?`, purchase).Scan(&owner); err != nil {
		return nil, err
	}
	if owner != user {
		return nil, ErrNotFound
	}
	c, err := scanPricingContract(s.db.QueryRowContext(ctx, pricingContractSelect+` WHERE purchase_id=? ORDER BY version DESC LIMIT 1`, purchase))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}

func snapshotPricingContractTx(ctx context.Context, tx *sql.Tx, purchase, user, product string, gross int64, now time.Time) error {
	p, err := loadPricingPolicy(ctx, tx, "combo", product, gross)
	if err != nil {
		return err
	}
	if !p.DynamicEnabled {
		return nil
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	addons, err := paidPricingAddonTermsTx(ctx, tx, purchase)
	if err != nil {
		return err
	}
	return insertPricingContractTx(ctx, tx, renewal.Contract{ID: id, PurchaseID: purchase, UserID: user, ProductID: product, Mode: "FIXED", Price: gross, Ceiling: gross, PolicyVersion: p.Version, RecordedAt: now, Version: 1, SourcePurchaseID: purchase, Addons: addons})
}

func inheritPricingContractTx(ctx context.Context, tx *sql.Tx, source, target string, gross int64, now time.Time) error {
	c, err := pricingContractTx(ctx, tx, source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	c.ID, err = ids.New()
	if err != nil {
		return err
	}
	c.PurchaseID = target
	c.Version = 1
	c.Price = gross
	if c.Mode == "DYNAMIC" {
		var discount int64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(d.discount_minor),0) FROM pricing_discount_spend d JOIN pricing_renewal_quotes q ON d.reference_id='renewal:'||q.id WHERE q.purchase_id=? AND d.status IN ('RESERVED','CONSUMED') AND q.state IN ('AUTHORIZED','READY_TO_SETTLE','DEFERRED_HEALTH','SETTLED')`, source).Scan(&discount); err != nil {
			return err
		}
		c.Price += discount // A one-term campaign cannot become an unlimited future discount.
	}
	c.Addons, err = paidPricingAddonTermsTx(ctx, tx, target)
	if err != nil {
		return err
	}
	c.Ceiling = max(c.Ceiling, c.Price)
	c.RecordedAt = now
	if err = insertPricingContractTx(ctx, tx, c); err != nil {
		return err
	}
	if c.Mode == "DYNAMIC" {
		if _, err = tx.ExecContext(ctx, `UPDATE pricing_discount_spend SET status='CONSUMED' WHERE status='RESERVED' AND reference_id IN (SELECT 'renewal:'||id FROM pricing_renewal_quotes WHERE purchase_id=? AND state IN ('AUTHORIZED','READY_TO_SETTLE','DEFERRED_HEALTH'))`, source); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET state='SETTLED',outcome_reason='',settled_purchase_id=? WHERE purchase_id=? AND state IN ('AUTHORIZED','READY_TO_SETTLE','DEFERRED_HEALTH') AND valid_until>?`, target, source, pricingStamp(now))
	}
	return err
}
