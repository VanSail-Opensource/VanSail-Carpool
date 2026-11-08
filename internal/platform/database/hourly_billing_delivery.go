package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func hourlyDeliveryGuards(delivery []HourlyDeliveryReady) (map[string]HourlyDeliveryReady, error) {
	if len(delivery) > 128 {
		return nil, ErrConflict
	}
	result := make(map[string]HourlyDeliveryReady, len(delivery))
	for _, proof := range delivery {
		if !validHourlyCreditName(proof.EnrollmentID) {
			return nil, ErrConflict
		}
		if _, exists := result[proof.EnrollmentID]; exists {
			return nil, ErrConflict
		}
		result[proof.EnrollmentID] = proof
	}
	return result, nil
}

func hourlyDeliveryFresh(proof HourlyDeliveryReady, now time.Time) bool {
	return proof.EnrollmentID != "" && !proof.ObservedAt.IsZero() && !proof.ObservedAt.After(now) &&
		now.Sub(proof.ObservedAt) <= 4*time.Second && proof.ReadyUntil.After(now)
}

func hourlyCompileRevisionTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 FOR UPDATE`).Scan(&revision)
	return revision, err
}

func hourlyPaidDeliveryStillReadyTx(ctx context.Context, tx *sql.Tx, receipts []model.HourlySettlement, guards map[string]HourlyDeliveryReady) error {
	if len(receipts) == 0 {
		return nil
	}
	now, err := hourlyBillingClockTx(ctx, tx)
	if err != nil {
		return err
	}
	for _, receipt := range receipts {
		if !hourlyDeliveryFresh(guards[receipt.EnrollmentID], now) {
			return ErrHourlyDeliveryPending
		}
	}
	return nil
}
