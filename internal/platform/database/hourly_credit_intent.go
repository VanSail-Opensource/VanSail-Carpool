package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ResolveHourlyCreditIntent proves absence only under a permanent admission
// fence. Taking the original owner/node locks also orders uncertain earlier
// Reserve transactions; any later one must recheck these same fences.
// Active/unfenced absence remains unavailable and must retry the exact request.
func (s *Store) ResolveHourlyCreditIntent(ctx context.Context, request model.HourlyCreditRequest) (model.HourlyCreditReservation, error) {
	if err := validateHourlyCreditRequest(request); err != nil {
		return model.HourlyCreditReservation{}, err
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyCreditReservation, error) {
		var status, currentDay string
		var generation int64
		if err := tx.QueryRowContext(ctx, `SELECT account_status,account_generation FROM users WHERE id=$1 FOR NO KEY UPDATE`, request.OwnerID).Scan(&status, &generation); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		var enabled int
		var removing, retired sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT enabled,removing_at,retired_at FROM xnode_nodes WHERE id=$1 FOR SHARE`, request.NodeID).Scan(&enabled, &removing, &retired); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		if err := tx.QueryRowContext(ctx, `SELECT (clock_timestamp() AT TIME ZONE 'UTC')::date::text`).Scan(&currentDay); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		value, err := hourlyCreditReservationTx(ctx, tx, request.ReservationID)
		if err == nil {
			if value.HourlyCreditRequest != request {
				return value, ErrConflict
			}
			return value, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return value, err
		}
		if status != "active" || generation != request.AccountGeneration || removing.Valid || retired.Valid || currentDay > request.DayUTC {
			return value, model.ErrHourlyCreditNotFound
		}
		// A merely disabled node can be re-enabled. Only the persisted removal
		// fence, lifecycle change or expired day proves permanent non-admission.
		return value, model.ErrHourlyCreditUnavailable
	})
}
