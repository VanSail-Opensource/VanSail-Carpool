package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyCreditDayColumns = `owner_id,account_generation,day_utc::text,free_allowance_bytes,free_admitted_bytes,free_reserved_bytes,extra_funded_bytes,extra_admitted_bytes,extra_reserved_bytes,created_at,updated_at`

// EnsureHourlyCreditDay snapshots the first accepted free allowance for today.
// Later policy changes cannot mint or remove already issued same-day allowance.
func (s *Store) EnsureHourlyCreditDay(ctx context.Context, owner string, generation int64, day string, allowance int64) (model.HourlyCreditDay, error) {
	if !validHourlyCreditName(owner) || generation < 0 || !validHourlyCreditDay(day) || allowance < 0 {
		return model.HourlyCreditDay{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyCreditDay, error) {
		if err := lockHourlyOwnerTx(ctx, tx, owner, generation); err != nil {
			return model.HourlyCreditDay{}, err
		}
		if err := currentHourlyCreditDayTx(ctx, tx, day); err != nil {
			return model.HourlyCreditDay{}, err
		}
		now := stamp(time.Now().UTC())
		_, err := tx.ExecContext(ctx, `INSERT INTO hourly_credit_days(owner_id,account_generation,day_utc,free_allowance_bytes,created_at,updated_at) VALUES($1,$2,$3::date,$4,$5,$5) ON CONFLICT DO NOTHING`, owner, generation, day, allowance, now)
		if err != nil {
			return model.HourlyCreditDay{}, err
		}
		value, err := hourlyCreditDayTx(ctx, tx, owner, generation, day)
		if err == nil {
			err = currentHourlyCreditDayTx(ctx, tx, day)
		}
		return value, err
	})
}

// HourlyCreditDay reads historical accounting without requiring a live account.
func (s *Store) HourlyCreditDay(ctx context.Context, owner string, generation int64, day string) (model.HourlyCreditDay, error) {
	if !validHourlyCreditDay(day) {
		return model.HourlyCreditDay{}, ErrConflict
	}
	return scanHourlyCreditDay(s.db.QueryRowContext(ctx, `SELECT `+hourlyCreditDayColumns+` FROM hourly_credit_days WHERE owner_id=$1 AND account_generation=$2 AND day_utc=$3::date`, owner, generation, day))
}

func hourlyCreditDayTx(ctx context.Context, tx *sql.Tx, owner string, generation int64, day string) (model.HourlyCreditDay, error) {
	return scanHourlyCreditDay(tx.QueryRowContext(ctx, `SELECT `+hourlyCreditDayColumns+` FROM hourly_credit_days WHERE owner_id=$1 AND account_generation=$2 AND day_utc=$3::date FOR UPDATE`, owner, generation, day))
}

func scanHourlyCreditDay(row rowScanner) (model.HourlyCreditDay, error) {
	var d model.HourlyCreditDay
	var created, updated string
	err := row.Scan(&d.OwnerID, &d.AccountGeneration, &d.DayUTC, &d.FreeAllowanceBytes, &d.FreeAdmittedBytes, &d.FreeReservedBytes, &d.ExtraFundedBytes, &d.ExtraAdmittedBytes, &d.ExtraReservedBytes, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if d.CreatedAt, err = parseStamp(created); err != nil {
		return d, err
	}
	d.UpdatedAt, err = parseStamp(updated)
	return d, err
}

func adjustHourlyCreditDayTx(ctx context.Context, tx *sql.Tx, r model.HourlyCreditRequest, admitted, reserved int64) error {
	if admitted == 0 && reserved == 0 {
		return nil
	}
	prefix := "free"
	if r.Kind == "extra" {
		prefix = "extra"
	}
	query := `UPDATE hourly_credit_days SET ` + prefix + `_admitted_bytes=` + prefix + `_admitted_bytes+$1,` + prefix + `_reserved_bytes=` + prefix + `_reserved_bytes+$2,updated_at=$3 WHERE owner_id=$4 AND account_generation=$5 AND day_utc=$6::date`
	result, err := tx.ExecContext(ctx, query, admitted, reserved, stamp(time.Now().UTC()), r.OwnerID, r.AccountGeneration, r.DayUTC)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return ErrConflict
	}
	return err
}
