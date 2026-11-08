package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ReserveHourlyCredit commits a shared byte obligation before its installation
// can be sent. Only the existing account generation and current UTC day admit.
func (s *Store) ReserveHourlyCredit(ctx context.Context, request model.HourlyCreditRequest) (model.HourlyCreditReservation, error) {
	if err := validateHourlyCreditRequest(request); err != nil {
		return model.HourlyCreditReservation{}, err
	}
	value, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyCreditReservation, error) {
		if err := lockHourlyOwnerTx(ctx, tx, request.OwnerID, request.AccountGeneration); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		if err := lockHourlyCreditNodeTx(ctx, tx, request); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		existing, err := hourlyCreditReservationTx(ctx, tx, request.ReservationID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return model.HourlyCreditReservation{}, err
		}
		found := err == nil
		day, err := hourlyCreditDayTx(ctx, tx, request.OwnerID, request.AccountGeneration, request.DayUTC)
		if err != nil {
			return model.HourlyCreditReservation{}, err
		}
		if err = currentHourlyCreditDayTx(ctx, tx, request.DayUTC); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		if found {
			if existing.HourlyCreditRequest != request {
				return model.HourlyCreditReservation{}, ErrConflict
			}
			return existing, nil
		}
		available := day.FreeAvailableBytes()
		if request.Kind == "extra" {
			available = day.ExtraAvailableBytes()
		}
		if request.ByteCeiling > available {
			return model.HourlyCreditReservation{}, errors.Join(ErrHourlyCreditUnavailable, model.ErrHourlyCreditNotReserved)
		}
		sequence, err := nextHourlyCreditSequenceTx(ctx, tx, request.AgentID)
		if err != nil {
			return model.HourlyCreditReservation{}, err
		}
		if err = adjustHourlyCreditDayTx(ctx, tx, request, 0, request.ByteCeiling); err != nil {
			return model.HourlyCreditReservation{}, err
		}
		value, err := insertHourlyCreditReservationTx(ctx, tx, request, sequence)
		if errors.Is(err, ErrNotFound) {
			err = ErrConflict
		}
		return value, err
	})
	// Cancellation/network failures cannot authorize forgetting an intent. This
	// marker exists only on the absent-row, locked, read-only quota rejection.
	if errors.Is(err, model.ErrHourlyCreditNotReserved) && ctx.Err() != nil {
		return model.HourlyCreditReservation{}, ctx.Err()
	}
	return value, err
}

func insertHourlyCreditReservationTx(ctx context.Context, tx *sql.Tx, request model.HourlyCreditRequest, sequence int64) (model.HourlyCreditReservation, error) {
	return scanHourlyCreditReservation(tx.QueryRowContext(ctx, `INSERT INTO hourly_credit_reservations(reservation_id,owner_id,account_generation,day_utc,node_id,agent_id,boot_id,session_id,kind,byte_ceiling,issue_sequence,created_at) VALUES($1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING RETURNING `+hourlyCreditReservationColumns, request.ReservationID, request.OwnerID, request.AccountGeneration, request.DayUTC, request.NodeID, request.AgentID, request.BootID, request.SessionID, request.Kind, request.ByteCeiling, sequence, stamp(time.Now().UTC())))
}

func lockHourlyCreditNodeTx(ctx context.Context, tx *sql.Tx, r model.HourlyCreditRequest) error {
	var agent sql.NullString
	var enabled int
	var removing, retired sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT agent_id,enabled,removing_at,retired_at FROM xnode_nodes WHERE id=$1 FOR SHARE`, r.NodeID).Scan(&agent, &enabled, &removing, &retired)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err == nil && (!agent.Valid || agent.String != r.AgentID || enabled != 1 || removing.Valid || retired.Valid) {
		return ErrConflict
	}
	return err
}

func nextHourlyCreditSequenceTx(ctx context.Context, tx *sql.Tx, agent string) (int64, error) {
	now := stamp(time.Now().UTC())
	if _, err := tx.ExecContext(ctx, `INSERT INTO hourly_credit_agent_sequences(agent_id,last_issued,updated_at) VALUES($1,0,$2) ON CONFLICT DO NOTHING`, agent, now); err != nil {
		return 0, err
	}
	var sequence int64
	err := tx.QueryRowContext(ctx, `UPDATE hourly_credit_agent_sequences SET last_issued=last_issued+1,updated_at=$1 WHERE agent_id=$2 AND last_issued<$3 RETURNING last_issued`, now, agent, int64(math.MaxInt64)).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrHourlyCreditUnavailable
	}
	return sequence, err
}
