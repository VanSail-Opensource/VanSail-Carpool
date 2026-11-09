package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AdminHourlyEnrollment is a safe administrator projection of one historical or
// current enrollment. It intentionally omits credentials, proxy/period
// references, generations, activation keys, and fingerprints.
type AdminHourlyEnrollment struct {
	EnrollmentID        string
	GroupID             string
	Name                string
	HourlyPriceTXBMinor int64
	OfferEnabled        bool
	Status              string
	ActivatedAt         *time.Time
	PaidThrough         *time.Time
	DisabledAt          *time.Time
	CooldownUntil       *time.Time
	CreatedAt           time.Time
}

// AdminHourlyDisableInput is one administrator stop without member cooldown.
type AdminHourlyDisableInput struct {
	OwnerID, EnrollmentID string
	AccountGeneration     int64
}

// ListAdminHourlyEnrollments returns every enrollment status for the current
// account generation, including disabled and closed history, with live names.
func (s *Store) ListAdminHourlyEnrollments(ctx context.Context, owner string, generation int64) ([]AdminHourlyEnrollment, error) {
	if !validHourlyCreditName(owner) || generation < 0 {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.group_id,COALESCE(g.name,''),COALESCE(o.enabled,0),
		COALESCE(o.hourly_price_minor,0),e.status,e.activated_at,e.paid_through,e.disabled_at,e.cooldown_until,e.created_at
		FROM hourly_enrollments e
		LEFT JOIN xnode_groups g ON g.id=e.group_id
		LEFT JOIN hourly_offers o ON o.group_id=e.group_id
		WHERE e.owner_id=$1 AND e.account_generation=$2
		ORDER BY e.created_at DESC,e.id DESC LIMIT 8193`, owner, generation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AdminHourlyEnrollment{}
	for rows.Next() {
		var item AdminHourlyEnrollment
		var enabled int
		var activated, paid, disabled, cooldown sql.NullTime
		if err := rows.Scan(&item.EnrollmentID, &item.GroupID, &item.Name, &enabled, &item.HourlyPriceTXBMinor, &item.Status,
			&activated, &paid, &disabled, &cooldown, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.OfferEnabled = enabled == 1
		item.ActivatedAt, item.PaidThrough = hourlyTimePointer(activated), hourlyTimePointer(paid)
		item.DisabledAt, item.CooldownUntil = hourlyTimePointer(disabled), hourlyTimePointer(cooldown)
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) > 8192 {
		return nil, ErrConflict
	}
	return result, nil
}

// DisableAdminHourlyEnrollment stops an enrollment on behalf of an
// administrator: it records a stop without granting a member cooldown and
// without classifying the stop as a funding suspension.
func (s *Store) DisableAdminHourlyEnrollment(ctx context.Context, input AdminHourlyDisableInput,
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
			WHERE id=$1 AND owner_id=$2 AND account_generation=$3 FOR UPDATE`,
			input.EnrollmentID, input.OwnerID, input.AccountGeneration))
		if err != nil {
			return e, err
		}
		if e.Status == model.HourlyDisabled || e.Status == model.HourlyClosed {
			return e, nil
		}
		if err = hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
			return e, err
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return e, err
		}
		e, err = scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET status=$1,
			disabled_at=COALESCE(disabled_at,$2),cooldown_until=NULL,updated_at=$2 WHERE id=$3
			RETURNING `+hourlyEnrollmentColumns, model.HourlyDisabled, now, e.ID))
		if err == nil {
			err = enqueueHourlySyncTx(ctx, tx, e.OwnerID, now)
		}
		return e, err
	})
}
