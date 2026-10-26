package database

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) renewWithOutcome(ctx context.Context, input RenewalInput, excluded []string, now time.Time) (model.RenewalBatch, error) {
	var b model.RenewalBatch
	var err error
	for attempt := 0; ; attempt++ {
		b, err = s.renew(ctx, input, excluded, now)
		if !s.retryTransaction(ctx, err, attempt) {
			break
		}
	}
	if errors.Is(err, ErrInsufficientBalance) {
		_, e := s.db.ExecContext(ctx, `UPDATE pricing_renewal_quotes SET outcome_reason='INSUFFICIENT_FUNDS' WHERE purchase_id=? AND state<>'SETTLED'`, input.PurchaseID)
		if e != nil {
			return b, errors.Join(err, e)
		}
	}
	return b, err
}
