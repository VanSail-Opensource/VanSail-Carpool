package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// DuePricingRenewals bounds preparation work to contracts with sufficient notice.
func (s *Store) DuePricingRenewals(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id FROM purchases p JOIN pricing_product_settings policy ON policy.product_type='combo' AND policy.product_id=p.combo_id WHERE policy.dynamic_enabled=1 AND policy.renewal_pricing_mode='DYNAMIC' AND p.status IN ('active','activating','queued') AND p.valid_until<=? AND p.valid_until>? AND NOT EXISTS(SELECT 1 FROM pricing_renewal_quotes q WHERE q.purchase_id=p.id AND q.state NOT IN ('CANCELLED','EXPIRED')) ORDER BY p.valid_until,p.id LIMIT 32`, pricingStamp(now.Add(7*24*time.Hour)), pricingStamp(now.Add(-72*time.Hour)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DeferPricingRenewal preserves an accepted quote for a bounded 72-hour retry window.
// It grants no additional access and never extends an entitlement.
func (s *Store) DeferPricingRenewal(ctx context.Context, purchase string, now time.Time) (bool, error) {
	var mode, until string
	err := s.db.QueryRowContext(ctx, `SELECT c.mode,p.valid_until FROM purchases p JOIN pricing_contracts c ON c.purchase_id=p.id WHERE p.id=? ORDER BY c.version DESC LIMIT 1`, purchase).Scan(&mode, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	due, err := parseStamp(until)
	if err != nil {
		return false, err
	}
	if mode != "DYNAMIC" || !now.Before(due.Add(72*time.Hour)) {
		return false, nil
	}
	_, err = s.db.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET state='DEFERRED_HEALTH' WHERE purchase_id=? AND state IN ('AUTHORIZED','READY_TO_SETTLE')`, purchase)
	return true, err
}

// IsPricingRenewalDeferral identifies optional authorization/health problems.
func IsPricingRenewalDeferral(err error) bool {
	return errors.Is(err, pricing.RenewalQuoteUnavailable) || errors.Is(err, pricing.RenewalHealthDeferred) || errors.Is(err, pricing.CapacityUnsafe)
}
