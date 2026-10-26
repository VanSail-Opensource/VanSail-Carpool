package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Account projections carry the currently eligible prospective price so funding
// admission and connection observation agree with the budgeted debit transaction.
// The stored public accounting basis is never replaced by this personal preview.
func (s *Store) retryAttempt_pricingPAYGOffers(ctx context.Context, accounts []xnode.PAYGAccount, now time.Time) ([]xnode.PAYGAccount, error) {
	if len(accounts) == 0 {
		return accounts, nil
	}
	enabled := false
	for _, a := range accounts {
		enabled = enabled || a.PersonalPricing
	}
	if !enabled {
		return accounts, nil
	}
	grants, err := s.pricingGrantScopes(ctx, now)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return accounts, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for i := range accounts {
		a := &accounts[i]
		if !a.PersonalPricing || !grants[a.UserID+":payg:"+a.GroupID] {
			continue
		}
		price, e := personalPricingTx(ctx, tx, a.UserID, "payg", a.GroupID, a.UnitPriceMinor, 1, "", false, now)
		if e != nil {
			return nil, e
		}
		a.UnitPriceMinor = price
	}
	return accounts, nil
}
