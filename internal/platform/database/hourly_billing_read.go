package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyEnrollmentColumns = `id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,
	activated_at,paid_through,disabled_at,cooldown_until,proxy_user_id,period_id,created_at,updated_at`

func scanHourlyEnrollment(row rowScanner) (model.HourlyEnrollment, error) {
	var e model.HourlyEnrollment
	var activated, paid, disabled, cooldown sql.NullTime
	var proxy sql.NullInt64
	var period sql.NullString
	err := row.Scan(&e.ID, &e.OwnerID, &e.AccountGeneration, &e.GroupID, &e.ActivationKey, &e.Fingerprint, &e.Status,
		&activated, &paid, &disabled, &cooldown, &proxy, &period, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	e.ActivatedAt, e.PaidThrough = hourlyTimePointer(activated), hourlyTimePointer(paid)
	e.DisabledAt, e.CooldownUntil = hourlyTimePointer(disabled), hourlyTimePointer(cooldown)
	e.ProxyUserID, e.PeriodID = proxy.Int64, period.String
	e.CreatedAt, e.UpdatedAt = e.CreatedAt.UTC(), e.UpdatedAt.UTC()
	return e, nil
}

func hourlyTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	at := value.Time.UTC()
	return &at
}

// HourlyEnrollment reads a stable activation, including its historical receipt
// references. Service callers must still authorize OwnerID before presenting it.
func (s *Store) HourlyEnrollment(ctx context.Context, id string) (model.HourlyEnrollment, error) {
	return scanHourlyEnrollment(s.db.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments WHERE id=$1`, id))
}

// HourlyEnrollmentForActivation discovers an immutable keyed intent before
// current policy or native readiness can obscure an already accepted receipt.
func (s *Store) HourlyEnrollmentForActivation(ctx context.Context, owner string, generation int64, key string) (model.HourlyEnrollment, error) {
	if !validHourlyCreditName(owner) || generation < 0 || !validHourlyCreditName(key) {
		return model.HourlyEnrollment{}, ErrConflict
	}
	return scanHourlyEnrollment(s.db.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+
		` FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND activation_key=$3`, owner, generation, key))
}

// HourlyViewEnrollments includes each squad's newest activation and any older
// unexpired cooldown. It is disposable presentation input, never admission proof.
func (s *Store) HourlyViewEnrollments(ctx context.Context, owner string, generation int64, at time.Time) ([]model.HourlyEnrollment, error) {
	if !validHourlyCreditName(owner) || generation < 0 {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `WITH latest AS (SELECT DISTINCT ON(group_id) id FROM hourly_enrollments
		WHERE owner_id=$1 AND account_generation=$2 ORDER BY group_id,created_at DESC,id DESC)
		SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2
		AND (id IN(SELECT id FROM latest) OR cooldown_until>$3) ORDER BY created_at DESC,id DESC LIMIT 8193`, owner, generation, at.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.HourlyEnrollment{}
	for rows.Next() {
		e, err := scanHourlyEnrollment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) > 8192 {
		return nil, ErrConflict
	}
	return result, nil
}

// HourlyLegacyBlocked is an advisory member-view check. Checkout repeats the
// same accepted-benefit predicate under the owner and wallet admission locks.
func (s *Store) HourlyLegacyBlocked(ctx context.Context, owner string, at time.Time) (bool, error) {
	if !validHourlyCreditName(owner) {
		return false, ErrConflict
	}
	var blocked bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=$1
		AND status IN ('active','activating','queued') AND `+purchaseTimestampOrderSQL("valid_until")+`>$2)`, owner, purchaseTimestampOrder(at)).Scan(&blocked)
	return blocked, err
}

// ListHourlyEnrollments bounds historical reads while retaining active rows at
// the front for account controls. Financial evidence is never deleted here.
func (s *Store) ListHourlyEnrollments(ctx context.Context, owner string, generation int64, limit int) ([]model.HourlyEnrollment, error) {
	if !validHourlyCreditName(owner) || generation < 0 || limit < 1 || limit > 200 {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments WHERE owner_id=$1
		AND account_generation=$2 ORDER BY (status IN ('preparing','active')) DESC,created_at DESC,id LIMIT $3`, owner, generation, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.HourlyEnrollment{}
	for rows.Next() {
		e, err := scanHourlyEnrollment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func hourlyBillingOfferTx(ctx context.Context, tx *sql.Tx, group string) (model.HourlyOffer, error) {
	return scanHourlyOffer(tx.QueryRowContext(ctx, `SELECT `+hourlyOfferColumns+
		` FROM hourly_offers WHERE group_id=$1 FOR SHARE`, group))
}
