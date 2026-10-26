package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Host exports show the latest committed prospective rate independently of
// funding or old prepaid block bases. An unavailable rate never erases inventory.
func (s *Store) retryAttempt_HostPAYGRate(ctx context.Context, user, group string, now time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	fixed, err := pricingReference(ctx, tx, "payg", group)
	if err != nil {
		return 0, err
	}
	p, err := loadPricingPolicy(ctx, tx, "payg", group, fixed)
	if err != nil {
		return 0, err
	}
	public := fixed
	if p.DynamicEnabled && p.PurchaseMode == "DYNAMIC" {
		price, e := approvedPricingTx(ctx, tx, p, fixed, now)
		if e == nil {
			public = price
		} else if p.LastApprovedVersion != "" {
			e = tx.QueryRowContext(ctx, `SELECT price_minor FROM pricing_published_prices WHERE id=?`, p.LastApprovedVersion).Scan(&public)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return 0, e
			}
		}
	}
	return personalPricingTx(ctx, tx, user, "payg", group, public, 1, "", false, now)
}
