package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// HourlyCreditAccounting is a read-only owner/generation projection. The bool
// preserves OutstandingHourlyCredits' all-day semantics, including open credits
// with zero residual bytes. Display details never authorize settlement or reuse.
func (s *Store) HourlyCreditAccounting(ctx context.Context, owner string, generation int64, day string) (model.HourlyAccountingStatus, bool, error) {
	var status model.HourlyAccountingStatus
	if !validHourlyCreditName(owner) || generation < 0 || !validHourlyCreditDay(day) {
		return status, false, ErrConflict
	}
	var pending bool
	err := s.db.QueryRowContext(ctx, `SELECT
		COUNT(*) > 0,
		COALESCE(SUM(byte_ceiling-admitted_upload_bytes-admitted_download_bytes-unused_bytes)
			FILTER (WHERE day_utc=$3::date AND final_proof_digest=''),0)::text,
		COALESCE(SUM(unknown_bytes)
			FILTER (WHERE day_utc=$3::date AND final_recovery),0)::text,
		COALESCE(SUM(unknown_bytes)
			FILTER (WHERE day_utc=$3::date AND final_recovery AND kind='free'),0)::text,
		COALESCE(SUM(unknown_bytes)
			FILTER (WHERE day_utc=$3::date AND final_recovery AND kind='extra'),0)::text,
		COALESCE(SUM(unknown_bytes)
			FILTER (WHERE day_utc<>$3::date AND final_recovery),0)::text,
		COALESCE(SUM(byte_ceiling-admitted_upload_bytes-admitted_download_bytes-unused_bytes)
			FILTER (WHERE day_utc<>$3::date AND final_proof_digest=''),0)::text,
		COUNT(*) FILTER (WHERE day_utc<>$3::date AND final_proof_digest='')
		FROM hourly_credit_reservations
		WHERE owner_id=$1 AND account_generation=$2
		AND (final_proof_digest='' OR final_recovery AND unknown_bytes>0)`, owner, generation, day).
		Scan(&pending, &status.OpenReservedBytes, &status.RecoveryUnknownBytes,
			&status.FreeRecoveryUnknownBytes, &status.ExtraRecoveryUnknownBytes,
			&status.HistoricalRecoveryUnknownBytes, &status.HistoricalUnsealedReservedBytes,
			&status.HistoricalUnsealedCount)
	return status, pending, err
}
