package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// SettleHourlyOwnerAvailable prepays the deliverable remainder of the current UTC
// hour after a missed boundary, or the next whole hour inside the bounded
// prefetch window before it. It never back-bills missed hours or automatically
// resumes suspension. One wallet lock makes simultaneous squad renewals
// deterministic.
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
			e, receipt, err := s.settleHourlyEnrollmentTx(ctx, tx, e, snapshot, offers[e.GroupID], guard)
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

// settleHourlyEnrollmentTx commits one deterministic wallet outcome. Inside the
// bounded prefetch window it buys the next whole hour starting at the current
// paid boundary, so the compiled native ValidUntil is extended before the old
// boundary denies access. A genuine late/offline recovery still pays only the
// deliverable remainder of the current UTC hour and never back-bills missed
// hours. Nothing here resumes a funding suspension.
func (s *Store) settleHourlyEnrollmentTx(ctx context.Context, tx *sql.Tx, e model.HourlyEnrollment,
	snapshot model.HourlyPolicySnapshot, offer model.HourlyOffer, delivery HourlyDeliveryReady) (model.HourlyEnrollment, *model.HourlySettlement, error) {
	now, err := hourlyBillingClockTx(ctx, tx)
	if err != nil || e.PaidThrough == nil {
		if err == nil {
			err = ErrConflict
		}
		return e, nil, err
	}
	from, prefetch := now, false
	if e.PaidThrough.After(now) {
		start, ok := hourlyPrefetchStart(*e.PaidThrough, now)
		if !ok {
			return e, nil, nil
		}
		from, prefetch = start, true
	}
	if !hourlyDeliveryFresh(delivery, now) {
		return e, nil, ErrHourlyDeliveryPending
	}
	if offer.HourlyPriceTXBMinor <= 0 {
		if prefetch {
			// A future interval is not purchasable right now. Skipping leaves the
			// committed remainder intact; the boundary path decides there.
			return e, nil, nil
		}
		e, err = suspendHourlyEnrollmentTx(ctx, tx, e, now)
		return e, nil, err
	}
	// Probe the exact interval start of this charge, not the current hour: a
	// prefetch closes the next hour while a late recovery closes the current one.
	receipt, err := scanHourlySettlement(tx.QueryRowContext(ctx, `SELECT `+hourlySettlementColumns+
		` FROM hourly_settlements WHERE enrollment_id=$1 AND hour_start=$2`, e.ID, from.Truncate(time.Hour)))
	if errors.Is(err, ErrNotFound) {
		receipt, err = payHourlyIntervalAtTx(ctx, tx, e, offer, snapshot, from, now)
	}
	if errors.Is(err, ErrInsufficientBalance) {
		if prefetch {
			// Bounded safe skip: the member keeps the paid hour and the definitive
			// suspension still happens once the boundary passes.
			return e, nil, nil
		}
		e, err = suspendHourlyEnrollmentTx(ctx, tx, e, now)
		if err != nil {
			return e, nil, err
		}
		// The definitive stop and its member receipt share this transaction so
		// the notification cannot claim a state the enrollment did not reach.
		if err = s.enqueueHourlyFundingSuspendedTx(ctx, tx, e, offer, now); err != nil {
			return e, nil, err
		}
		return e, nil, nil
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
