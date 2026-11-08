package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CancelHourlyPreparation releases only an unpaid preparing activation after a
// definitive failure. Original owner/wallet locks wait out an uncertain commit;
// a paid or otherwise transitioned enrollment is never cancelled by this path.
func (s *Store) CancelHourlyPreparation(ctx context.Context, owner string, generation int64, enrollment string, _ time.Time) (model.HourlyEnrollment, error) {
	if !validHourlyCreditName(owner) || generation < 0 || enrollment == "" {
		return model.HourlyEnrollment{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyEnrollment, error) {
		if err := lockHourlyOwnerTx(ctx, tx, owner, generation); err != nil {
			return model.HourlyEnrollment{}, err
		}
		if _, err := balanceTx(ctx, tx, owner); err != nil {
			return model.HourlyEnrollment{}, err
		}
		e, err := scanHourlyEnrollment(tx.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments
			WHERE id=$1 AND owner_id=$2 AND account_generation=$3 FOR UPDATE`, enrollment, owner, generation))
		if errors.Is(err, ErrNotFound) {
			return e, ErrConflict
		}
		if err != nil {
			return e, err
		}
		if e.Status != model.HourlyPreparing || e.ActivatedAt != nil || e.PaidThrough != nil {
			return e, ErrConflict
		}
		var charged bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_settlements WHERE enrollment_id=$1)`, e.ID).Scan(&charged); err != nil {
			return e, err
		}
		if charged {
			return e, ErrConflict
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return e, err
		}
		e, err = scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET status='disabled',disabled_at=$1,
			cooldown_until=NULL,updated_at=$1 WHERE id=$2 AND status='preparing' RETURNING `+hourlyEnrollmentColumns, now, e.ID))
		if err == nil {
			err = enqueueHourlySyncTx(ctx, tx, owner, now)
		}
		return e, err
	})
}
