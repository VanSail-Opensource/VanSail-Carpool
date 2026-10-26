package database

import (
	"context"
	"database/sql"

	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

const pricingQuoteSelect = `SELECT id,purchase_id,contract_id,price_minor,policy_version,contract_version,state,generated_at,authorized_at,settle_after,valid_until,settled_purchase_id,probability,contribution,health_state,model_version,trace_json FROM pricing_renewal_quotes`

func scanPricingQuote(row rowScanner) (renewal.Quote, error) {
	var q renewal.Quote
	var generated, after, until string
	var authorized sql.NullString
	err := row.Scan(&q.ID, &q.PurchaseID, &q.ContractID, &q.Price, &q.PolicyVersion, &q.ContractVersion, &q.State, &generated, &authorized, &after, &until, &q.SettledPurchaseID, &q.Probability, &q.Contribution, &q.HealthState, &q.ModelVersion, &q.Trace)
	if err != nil {
		return q, err
	}
	for _, v := range []struct {
		s string
		p *time.Time
	}{{generated, &q.GeneratedAt}, {after, &q.SettleAfter}, {until, &q.ValidUntil}} {
		*v.p, err = parseStamp(v.s)
		if err != nil {
			return q, err
		}
	}
	if authorized.Valid {
		at, e := parseStamp(authorized.String)
		if e != nil {
			return q, e
		}
		q.AuthorizedAt = &at
	}
	return q, nil
}

// PricingRenewalQuote reads an immutable latest proposal for an owned purchase.
func (s *Store) PricingRenewalQuote(ctx context.Context, user, purchase string) (*renewal.Quote, error) {
	c, err := s.PricingContract(ctx, user, purchase)
	if err != nil || c == nil {
		return nil, err
	}
	q, err := scanPricingQuote(s.db.QueryRowContext(ctx, pricingQuoteSelect+` WHERE contract_id=? ORDER BY generated_at DESC LIMIT 1`, c.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &q, err
}
