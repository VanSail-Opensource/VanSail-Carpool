package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Automatic discovery may finish after an administrator saves or re-enables an
// account. Only the exact revision that was probed may be disabled.
func (s *Store) retryAttempt_DisablePaymentProfileIfUnchanged(ctx context.Context, id string, expectedUpdatedAt time.Time) (model.PaymentProfile, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentProfile{}, false, err
	}
	defer tx.Rollback()
	record, err := scanPaymentProfile(tx.QueryRowContext(ctx, paymentProfileSelect+` WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return model.PaymentProfile{}, false, err
	}
	if !record.Enabled || !record.UpdatedAt.Equal(expectedUpdatedAt) {
		return record.PaymentProfile, false, nil
	}
	now, err := paymentProfileRevision(stamp(record.UpdatedAt))
	if err != nil {
		return model.PaymentProfile{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_profiles SET enabled=0,updated_at=$1 WHERE id=$2`, stamp(now), id); err != nil {
		return model.PaymentProfile{}, false, fmt.Errorf("disable unchanged payment profile: %w", err)
	}
	if err := paymentProfileAuditTx(ctx, tx, "", "payment_profile.availability", id, now); err != nil {
		return model.PaymentProfile{}, false, err
	}
	record.Enabled, record.UpdatedAt = false, now
	if err := tx.Commit(); err != nil {
		return model.PaymentProfile{}, false, err
	}
	return record.PaymentProfile, true, nil
}

// Keep revisions strictly increasing even after a clock correction or two
// writes inside one clock tick. The timestamp stays private to the server.
func paymentProfileRevision(previous string) (time.Time, error) {
	now := nowUTC()
	if previous == "" {
		return now, nil
	}
	parsed, err := parseStamp(previous)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse previous payment profile revision: %w", err)
	}
	if !now.After(parsed) {
		now = parsed.Add(time.Nanosecond)
	}
	return now, nil
}
