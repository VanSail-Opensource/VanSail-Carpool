package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// HourlyDisableInput distinguishes voluntary stops from funding suspension.
type HourlyDisableInput struct {
	OwnerID, EnrollmentID string
	AccountGeneration     int64
	Voluntary             bool
}

// DisableHourlyEnrollment stops authorization without refunding the paid hour.
// An unpaid preparation can be cancelled without imposing a usage cooldown.
func (s *Store) DisableHourlyEnrollment(ctx context.Context, input HourlyDisableInput,
	snapshot model.HourlyPolicySnapshot, _ time.Time) (model.HourlyEnrollment, error) {
	if !validHourlyCreditName(input.OwnerID) || input.AccountGeneration < 0 || input.EnrollmentID == "" {
		return model.HourlyEnrollment{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyEnrollment, error) {
		if err := lockHourlyOwnerTx(ctx, tx, input.OwnerID, input.AccountGeneration); err != nil {
			return model.HourlyEnrollment{}, err
		}
		if _, err := balanceTx(ctx, tx, input.OwnerID); err != nil {
			return model.HourlyEnrollment{}, err
		}
		e, err := scanHourlyEnrollment(tx.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments
			WHERE id=$1 AND owner_id=$2 AND account_generation=$3 FOR UPDATE`, input.EnrollmentID, input.OwnerID, input.AccountGeneration))
		if err != nil {
			return e, err
		}
		if e.Status == model.HourlyDisabled || e.Status == model.HourlyClosed || e.Status == model.HourlyFundingSuspended && !input.Voluntary {
			return e, nil
		}
		if err = hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
			return e, err
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return e, err
		}
		status := model.HourlyDisabled
		var cooldown any
		if e.ActivatedAt != nil {
			if input.Voluntary {
				cooldown = now.Add(time.Duration(snapshot.Policy.CooldownSeconds) * time.Second)
			} else {
				status = model.HourlyFundingSuspended
			}
		}
		e, err = scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET status=$1,disabled_at=$2,
			cooldown_until=$3,updated_at=$2 WHERE id=$4 RETURNING `+hourlyEnrollmentColumns, status, now, cooldown, e.ID))
		if err == nil {
			err = enqueueHourlySyncTx(ctx, tx, e.OwnerID, now)
		}
		return e, err
	})
}

// HourlyDueOwner is one bounded scheduler admission unit, not a separate queue.
type HourlyDueOwner struct {
	OwnerID           string
	AccountGeneration int64
}

// DueHourlyOwners selects overdue active subscriptions for the existing queue.
func (s *Store) DueHourlyOwners(ctx context.Context, at time.Time, limit int) ([]HourlyDueOwner, error) {
	return s.DueHourlyOwnersPage(ctx, at, "", limit)
}

// DueHourlyOwnersPage lets the existing scheduler rotate past offline owners;
// an old unpaid boundary can never monopolize the first bounded page forever.
// Owners inside the bounded prefetch window are admitted early only while their
// wallet currently covers the next whole hour, so a squad the transaction would
// immediately skip is not re-admitted on every scheduler tick. An expired
// boundary is always admitted so the definitive suspension still happens there.
func (s *Store) DueHourlyOwnersPage(ctx context.Context, at time.Time, afterOwner string, limit int) ([]HourlyDueOwner, error) {
	if limit < 1 || limit > 200 || afterOwner != "" && !validHourlyCreditName(afterOwner) {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.owner_id,e.account_generation FROM hourly_enrollments e
		JOIN users u ON u.id=e.owner_id AND u.account_status='active' AND u.account_generation=e.account_generation
		WHERE e.status='active' AND (e.paid_through<=$1 OR (e.paid_through<=$2 AND EXISTS(
		SELECT 1 FROM balances b JOIN hourly_offers o ON o.group_id=e.group_id
		WHERE b.user_id=e.owner_id AND o.hourly_price_minor>0 AND b.txb_minor>=o.hourly_price_minor)))
		AND e.owner_id>$3 GROUP BY e.owner_id,e.account_generation
		ORDER BY e.owner_id LIMIT $4`, at.UTC(), at.UTC().Add(hourlyPrefetchLead), afterOwner, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []HourlyDueOwner{}
	for rows.Next() {
		var value HourlyDueOwner
		if err := rows.Scan(&value.OwnerID, &value.AccountGeneration); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
