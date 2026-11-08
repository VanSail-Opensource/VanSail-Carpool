package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// PrepareHourlyActivation persists an unpaid activation intent. Native
// credentials may be prepared afterward without granting or charging access.
// The scheduler time is deliberately not used as a financial timestamp.
func (s *Store) PrepareHourlyActivation(ctx context.Context, input HourlyActivationInput,
	snapshot model.HourlyPolicySnapshot, _ time.Time) (model.HourlyEnrollment, error) {
	fingerprint, err := hourlyActivationFingerprint(input)
	if err != nil {
		return model.HourlyEnrollment{}, err
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyEnrollment, error) {
		if err := lockHourlyOwnerTx(ctx, tx, input.OwnerID, input.AccountGeneration); err != nil {
			return model.HourlyEnrollment{}, err
		}
		balance, err := balanceTx(ctx, tx, input.OwnerID)
		if err != nil {
			return model.HourlyEnrollment{}, err
		}
		existing, err := scanHourlyEnrollment(tx.QueryRowContext(ctx, `SELECT `+hourlyEnrollmentColumns+
			` FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2 AND activation_key=$3 FOR UPDATE`,
			input.OwnerID, input.AccountGeneration, input.ActivationKey))
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return model.HourlyEnrollment{}, ErrConflict
			}
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return model.HourlyEnrollment{}, err
		}
		if _, err = hourlyActivationChecksTx(ctx, tx, input, snapshot, balance); err != nil {
			return model.HourlyEnrollment{}, err
		}
		var duplicate bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2
			AND group_id=$3 AND status IN ('preparing','active'))`, input.OwnerID, input.AccountGeneration, input.GroupID).Scan(&duplicate); err != nil {
			return model.HourlyEnrollment{}, err
		}
		if duplicate {
			return model.HourlyEnrollment{}, ErrConflict
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2
			AND status IN ('preparing','active')`, input.OwnerID, input.AccountGeneration).Scan(&count); err != nil {
			return model.HourlyEnrollment{}, err
		}
		if count >= 128 {
			return model.HourlyEnrollment{}, ErrConflict
		}
		id, err := ids.New()
		if err != nil {
			return model.HourlyEnrollment{}, err
		}
		now, err := hourlyBillingClockTx(ctx, tx)
		if err != nil {
			return model.HourlyEnrollment{}, err
		}
		return scanHourlyEnrollment(tx.QueryRowContext(ctx, `INSERT INTO hourly_enrollments
			(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,'preparing',$7,$7) RETURNING `+hourlyEnrollmentColumns,
			id, input.OwnerID, input.AccountGeneration, input.GroupID, input.ActivationKey, fingerprint, now))
	})
}

func hourlyActivationChecksTx(ctx context.Context, tx *sql.Tx, input HourlyActivationInput,
	snapshot model.HourlyPolicySnapshot, balance int64) (model.HourlyOffer, error) {
	if err := hourlyActivationGroupTx(ctx, tx, input.GroupID); err != nil {
		return model.HourlyOffer{}, err
	}
	if input.ExpectedPolicyVersion != snapshot.Version {
		return model.HourlyOffer{}, ErrConflict
	}
	if err := hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
		return model.HourlyOffer{}, err
	}
	if !snapshot.Policy.SalesEnabled {
		return model.HourlyOffer{}, ErrHourlySalesDisabled
	}
	offer, err := hourlyBillingOfferTx(ctx, tx, input.GroupID)
	if err != nil {
		return offer, err
	}
	if !offer.Enabled || offer.Revision != input.ExpectedOfferRevision {
		return offer, ErrConflict
	}
	minimum, err := model.HourlyActivationMinimum(snapshot.Policy, offer.HourlyPriceTXBMinor)
	if err != nil {
		return offer, err
	}
	if balance < minimum {
		return offer, ErrInsufficientBalance
	}
	codes := map[string]string{}
	if input.ActivationCode != "" {
		codes[input.GroupID] = input.ActivationCode
	}
	if err := validateSquadActivationCodesTx(ctx, tx, model.Combo{}, []model.SquadProduct{{GroupID: input.GroupID}}, codes); err != nil {
		return offer, err
	}
	now, err := hourlyBillingClockTx(ctx, tx)
	if err != nil {
		return offer, err
	}
	if err = rejectLegacyHourlyActivationTx(ctx, tx, input.OwnerID, now); err != nil {
		return offer, err
	}
	var cooling bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments WHERE owner_id=$1 AND account_generation=$2
		AND group_id=$3 AND cooldown_until>$4)`, input.OwnerID, input.AccountGeneration, input.GroupID, now).Scan(&cooling)
	if err == nil && cooling {
		err = ErrHourlyCooldown
	}
	return offer, err
}

func hourlyActivationGroupTx(ctx context.Context, tx *sql.Tx, group string) error {
	var enabled int
	err := tx.QueryRowContext(ctx, `SELECT enabled FROM xnode_groups WHERE id=$1 FOR SHARE`, group).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err == nil && enabled != 1 {
		err = ErrConflict
	}
	return err
}
