package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// HourlyExtraPreferenceInput accepts the displayed current price as a ceiling.
// Disabling extras never discards already-funded credit before UTC midnight.
type HourlyExtraPreferenceInput struct {
	OwnerID               string
	AccountGeneration     int64
	Enabled               bool
	AcceptedPriceTXBMinor int64
}

// HourlyExtraPreference defaults to opt-out without creating a database row.
func (s *Store) HourlyExtraPreference(ctx context.Context, owner string, generation int64) (model.HourlyExtraPreference, error) {
	return hourlyExtraPreferenceRead(ctx, s.db, owner, generation, false)
}

type hourlyPreferenceReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func hourlyExtraPreferenceRead(ctx context.Context, reader hourlyPreferenceReader, owner string, generation int64, lock bool) (model.HourlyExtraPreference, error) {
	value := model.HourlyExtraPreference{OwnerID: owner, AccountGeneration: generation}
	if !validHourlyCreditName(owner) || generation < 0 {
		return value, ErrConflict
	}
	query := `SELECT extra_enabled,accepted_extra_price_minor,updated_at FROM hourly_account_preferences WHERE owner_id=$1 AND account_generation=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	var enabled int
	var cap sql.NullInt64
	err := reader.QueryRowContext(ctx, query, owner, generation).Scan(&enabled, &cap, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return value, nil
	}
	if err == nil {
		value.ExtraEnabled, value.AcceptedExtraPriceTXBMinor = enabled == 1, int64Pointer(cap)
		value.UpdatedAt = value.UpdatedAt.UTC()
	}
	return value, err
}

// SetHourlyExtraPreference records fresh price consent under the policy CAS.
func (s *Store) SetHourlyExtraPreference(ctx context.Context, input HourlyExtraPreferenceInput,
	snapshot model.HourlyPolicySnapshot, _ time.Time) (model.HourlyExtraPreference, error) {
	if !validHourlyCreditName(input.OwnerID) || input.AccountGeneration < 0 {
		return model.HourlyExtraPreference{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyExtraPreference, error) {
		if err := lockHourlyOwnerTx(ctx, tx, input.OwnerID, input.AccountGeneration); err != nil {
			return model.HourlyExtraPreference{}, err
		}
		if _, err := balanceTx(ctx, tx, input.OwnerID); err != nil {
			return model.HourlyExtraPreference{}, err
		}
		var cap any
		if input.Enabled {
			if err := hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
				return model.HourlyExtraPreference{}, err
			}
			if input.AcceptedPriceTXBMinor <= 0 || input.AcceptedPriceTXBMinor != snapshot.Policy.ExtraGBPriceTXBMinor {
				return model.HourlyExtraPreference{}, ErrConflict
			}
			cap = input.AcceptedPriceTXBMinor
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return model.HourlyExtraPreference{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO hourly_account_preferences(owner_id,account_generation,extra_enabled,accepted_extra_price_minor,updated_at)
			VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_id,account_generation) DO UPDATE SET extra_enabled=excluded.extra_enabled,
			accepted_extra_price_minor=excluded.accepted_extra_price_minor,updated_at=excluded.updated_at`,
			input.OwnerID, input.AccountGeneration, boolInt(input.Enabled), cap, now)
		if err != nil {
			return model.HourlyExtraPreference{}, err
		}
		return hourlyExtraPreferenceRead(ctx, tx, input.OwnerID, input.AccountGeneration, false)
	})
}
