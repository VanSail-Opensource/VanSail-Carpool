package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// SettleHourlyOwnerAvailable prepays only the deliverable remainder of the current UTC
// hour. It never back-bills missed hours or automatically resumes suspension.
// One wallet lock makes simultaneous squad renewals deterministic.
func (s *Store) SettleHourlyOwnerAvailable(ctx context.Context, owner string, generation int64,
	snapshot model.HourlyPolicySnapshot, delivery []HourlyDeliveryReady, _ time.Time) (HourlyOwnerSettlementResult, error) {
	if !validHourlyCreditName(owner) || generation < 0 {
		return HourlyOwnerSettlementResult{}, ErrConflict
	}
	guards, err := hourlyDeliveryGuards(delivery)
	if err != nil {
		return HourlyOwnerSettlementResult{}, err
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (HourlyOwnerSettlementResult, error) {
		if err := lockHourlyOwnerTx(ctx, tx, owner, generation); err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		if _, err := balanceTx(ctx, tx, owner); err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		if err := hourlyBillingPolicyTx(ctx, tx, snapshot); err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		enrollments, err := hourlyActiveEnrollmentsTx(ctx, tx, owner, generation)
		if err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		offers, err := hourlySettlementOffersTx(ctx, tx, enrollments)
		if err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		// Compare once before our own paid-through updates bump this revision.
		revision, err := hourlyCompileRevisionTx(ctx, tx)
		if err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		result := HourlyOwnerSettlementResult{Enrollments: []model.HourlyEnrollment{}, Settlements: []model.HourlySettlement{}, SkippedEnrollmentIDs: []string{}}
		for _, e := range enrollments {
			guard := guards[e.ID]
			if guard.CompileRevision <= 0 || guard.CompileRevision != revision {
				guard = HourlyDeliveryReady{}
			}
			e, receipt, err := settleHourlyEnrollmentTx(ctx, tx, e, snapshot, offers[e.GroupID], guard)
			if errors.Is(err, ErrHourlyDeliveryPending) {
				result.Enrollments = append(result.Enrollments, e)
				result.SkippedEnrollmentIDs = append(result.SkippedEnrollmentIDs, e.ID)
				continue
			}
			if err != nil {
				return HourlyOwnerSettlementResult{}, err
			}
			result.Enrollments = append(result.Enrollments, e)
			if receipt != nil {
				result.Settlements = append(result.Settlements, *receipt)
			}
		}
		if err = hourlyPaidDeliveryStillReadyTx(ctx, tx, result.Settlements, guards); err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		if err = hourlyPaidWindowStillLiveTx(ctx, tx, result.Settlements); err != nil {
			return HourlyOwnerSettlementResult{}, err
		}
		return result, nil
	})
}

func hourlyActiveEnrollmentsTx(ctx context.Context, tx *sql.Tx, owner string, generation int64) ([]model.HourlyEnrollment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+hourlyEnrollmentColumns+` FROM hourly_enrollments WHERE owner_id=$1
		AND account_generation=$2 AND status='active' ORDER BY activated_at,group_id,id LIMIT 129 FOR UPDATE`, owner, generation)
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
	if len(result) > 128 {
		return nil, ErrConflict
	}
	return result, nil
}

func settleHourlyEnrollmentTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment,
	snapshot model.HourlyPolicySnapshot, offer model.HourlyOffer, delivery HourlyDeliveryReady) (model.HourlyEnrollment, *model.HourlySettlement, error) {
	now, err := hourlyBillingClockTx(ctx, tx)
	if err != nil || e.PaidThrough == nil {
		if err == nil {
			err = ErrConflict
		}
		return e, nil, err
	}
	if e.PaidThrough.After(now) {
		return e, nil, nil
	}
	if !hourlyDeliveryFresh(delivery, now) {
		return e, nil, ErrHourlyDeliveryPending
	}
	if offer.HourlyPriceTXBMinor <= 0 {
		e, err = suspendHourlyEnrollmentTx(ctx, tx, e, now)
		return e, nil, err
	}
	receipt, err := scanHourlySettlement(tx.QueryRowContext(ctx, `SELECT `+hourlySettlementColumns+
		` FROM hourly_settlements WHERE enrollment_id=$1 AND hour_start=$2`, e.ID, now.Truncate(time.Hour)))
	if errors.Is(err, ErrNotFound) {
		receipt, err = payHourlyIntervalTx(ctx, tx, e, offer, snapshot, now)
	}
	if errors.Is(err, ErrInsufficientBalance) {
		e, err = suspendHourlyEnrollmentTx(ctx, tx, e, now)
		return e, nil, err
	}
	if err != nil {
		return e, nil, err
	}
	e, err = scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET paid_through=$1,updated_at=$2
		WHERE id=$3 AND status='active' RETURNING `+hourlyEnrollmentColumns, receipt.PaidUntil, now, e.ID))
	if err == nil {
		err = enqueueHourlySyncTx(ctx, tx, e.OwnerID, now)
	}
	return e, &receipt, err
}

func suspendHourlyEnrollmentTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment, at time.Time) (model.HourlyEnrollment, error) {
	e, err := scanHourlyEnrollment(tx.QueryRowContext(ctx, `UPDATE hourly_enrollments SET status='funding_suspended',
		disabled_at=$1,cooldown_until=NULL,updated_at=$1 WHERE id=$2 AND status='active' RETURNING `+hourlyEnrollmentColumns, at, e.ID))
	if err == nil {
		err = enqueueHourlySyncTx(ctx, tx, e.OwnerID, at)
	}
	return e, err
}

// Acquire every offer before the global revision row. No later native/offer
// row lookup may wait behind a mutation that itself needs that revision lock.
func hourlySettlementOffersTx(ctx context.Context, tx *sql.Tx, enrollments []model.HourlyEnrollment) (map[string]model.HourlyOffer, error) {
	groups := make([]string, 0, len(enrollments))
	for _, e := range enrollments {
		groups = append(groups, e.GroupID)
	}
	result := make(map[string]model.HourlyOffer, len(groups))
	for _, group := range uniqueSorted(groups) {
		offer, err := hourlyBillingOfferTx(ctx, tx, group)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[group] = offer
	}
	return result, nil
}
