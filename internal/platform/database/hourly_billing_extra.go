package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyExtraBlockBytes int64 = 1_000_000_000

// HourlyExtraFundingInput is constructed only from a current authenticated
// server-observed demand. Member requests cannot supply pending-byte evidence.
type HourlyExtraFundingInput struct {
	ReceiptID, OwnerID, DayUTC, ExpectedPolicyVersion string
	AccountGeneration                                 int64
	PendingBytes                                      uint64
	PendingObservedAt                                 time.Time
}

// FundHourlyExtraGB atomically buys one decimal GB after every previously free
// or funded byte is confirmed admitted. Held or unknown credit never counts as
// exhaustion. Stable receipt replay precedes current consent/price checks.
func (s *Store) FundHourlyExtraGB(ctx context.Context, input HourlyExtraFundingInput,
	snapshot model.HourlyPolicySnapshot, _ time.Time) (model.HourlyExtraPurchase, error) {
	if !validHourlyCreditName(input.ReceiptID) || !validHourlyCreditName(input.OwnerID) ||
		input.AccountGeneration < 0 || !validHourlyCreditDay(input.DayUTC) || !validHourlyCreditHex(input.ExpectedPolicyVersion, 64) {
		return model.HourlyExtraPurchase{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyExtraPurchase, error) {
		if err := lockHourlyOwnerTx(ctx, tx, input.OwnerID, input.AccountGeneration); err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		if _, err := balanceTx(ctx, tx, input.OwnerID); err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		existing, err := hourlyExtraPurchaseTx(ctx, tx, input.ReceiptID)
		if err == nil {
			if existing.OwnerID != input.OwnerID || existing.AccountGeneration != input.AccountGeneration ||
				existing.DayUTC != input.DayUTC || existing.PolicyVersion != input.ExpectedPolicyVersion {
				return model.HourlyExtraPurchase{}, ErrConflict
			}
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return model.HourlyExtraPurchase{}, err
		}
		if input.ExpectedPolicyVersion != snapshot.Version {
			return model.HourlyExtraPurchase{}, ErrConflict
		}
		if err = hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		pref, err := hourlyExtraPreferenceRead(ctx, tx, input.OwnerID, input.AccountGeneration, true)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		price := snapshot.Policy.ExtraGBPriceTXBMinor
		if !pref.ExtraEnabled || pref.AcceptedExtraPriceTXBMinor == nil || price <= 0 || price > *pref.AcceptedExtraPriceTXBMinor {
			return model.HourlyExtraPurchase{}, ErrHourlyExtraDisabled
		}
		day, err := hourlyCreditDayTx(ctx, tx, input.OwnerID, input.AccountGeneration, input.DayUTC)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		if err = validateHourlyExtraFundingTx(ctx, tx, input, day, now); err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		if day.ExtraFundedBytes > math.MaxInt64-hourlyExtraBlockBytes {
			return model.HourlyExtraPurchase{}, ErrConflict
		}
		balance, err := debitBalanceTx(ctx, tx, input.OwnerID, price, now)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		ledger, err := insertLedgerTx(ctx, tx, input.OwnerID, -price, balance, "hourly_extra_gb", input.ReceiptID, input.DayUTC, now)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		var block int64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(block_index),0)+1 FROM hourly_extra_purchases
			WHERE owner_id=$1 AND account_generation=$2 AND day_utc=$3::date`, input.OwnerID, input.AccountGeneration, input.DayUTC).Scan(&block); err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE hourly_credit_days SET extra_funded_bytes=extra_funded_bytes+$1,updated_at=$2
			WHERE owner_id=$3 AND account_generation=$4 AND day_utc=$5::date`, hourlyExtraBlockBytes, stamp(now), input.OwnerID, input.AccountGeneration, input.DayUTC)
		if err != nil {
			return model.HourlyExtraPurchase{}, err
		}
		p, err := scanHourlyExtraPurchase(tx.QueryRowContext(ctx, `INSERT INTO hourly_extra_purchases
			(id,owner_id,account_generation,day_utc,block_index,byte_count,unit_price_minor,policy_version,ledger_id,created_at)
			VALUES($1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10) RETURNING `+hourlyExtraPurchaseColumns, input.ReceiptID, input.OwnerID,
			input.AccountGeneration, input.DayUTC, block, hourlyExtraBlockBytes, price, snapshot.Version, ledger, now))
		if err == nil {
			current, clockErr := hourlyBillingClockTx(ctx, tx)
			if clockErr != nil {
				return p, clockErr
			}
			err = hourlyExtraFundingLiveTx(ctx, tx, input, current)
		}
		return p, err
	})
}

func validateHourlyExtraFundingTx(ctx context.Context, tx *sql.Tx, input HourlyExtraFundingInput, day model.HourlyCreditDay, now time.Time) error {
	if err := hourlyExtraFundingLiveTx(ctx, tx, input, now); err != nil {
		return err
	}
	if day.FreeAdmittedBytes != day.FreeAllowanceBytes || day.FreeReservedBytes != 0 ||
		day.ExtraAdmittedBytes != day.ExtraFundedBytes || day.ExtraReservedBytes != 0 {
		return ErrHourlyNotExhausted
	}
	return nil
}

func hourlyExtraFundingLiveTx(ctx context.Context, tx *sql.Tx, input HourlyExtraFundingInput, now time.Time) error {
	if input.PendingBytes == 0 || input.PendingObservedAt.IsZero() || input.PendingObservedAt.After(now) || now.Sub(input.PendingObservedAt) > 4*time.Second {
		return ErrHourlyDemandStale
	}
	if err := currentHourlyCreditDayTx(ctx, tx, input.DayUTC); err != nil {
		return err
	}
	var paid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2
		AND status='active' AND paid_through>$3)`, input.OwnerID, input.AccountGeneration, now).Scan(&paid)
	if err != nil {
		return err
	}
	if !paid {
		return ErrAccountUnavailable
	}
	return nil
}
