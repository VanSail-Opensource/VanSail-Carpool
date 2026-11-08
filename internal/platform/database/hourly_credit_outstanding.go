package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// HourlyCreditHeldBytes derives current-session obligations from the ledger.
// It never treats a disconnect or an unconfirmed progress hint as consumption.
func (s *Store) HourlyCreditHeldBytes(ctx context.Context, r model.HourlyCreditRequest) (int64, error) {
	if !validHourlyCreditName(r.OwnerID) || r.AccountGeneration < 0 || !validHourlyCreditDay(r.DayUTC) || !validHourlyCreditName(r.NodeID) || !validHourlyCreditHex(r.AgentID, 32) || !validHourlyCreditHex(r.BootID, 32) || !validHourlyCreditHex(r.SessionID, 32) {
		return 0, ErrConflict
	}
	var bytes int64
	err := s.db.QueryRowContext(ctx, `SELECT LEAST(COALESCE(SUM(byte_ceiling-admitted_upload_bytes-admitted_download_bytes),0),9223372036854775807)::bigint FROM hourly_credit_reservations WHERE owner_id=$1 AND account_generation=$2 AND day_utc=$3::date AND node_id=$4 AND agent_id=$5 AND boot_id=$6 AND session_id=$7 AND final_proof_digest=''`, r.OwnerID, r.AccountGeneration, r.DayUTC, r.NodeID, r.AgentID, r.BootID, r.SessionID).Scan(&bytes)
	return bytes, err
}

// OutstandingHourlyCredits includes open and finalized recovery-unknown debt.
// Generation -1 is allowed only within an explicit owner/node scope, including
// purge jobs whose lifecycle generation already advanced before settlement.
func (s *Store) OutstandingHourlyCredits(ctx context.Context, owner string, generation int64, node string, limit int) ([]model.HourlyCreditReservation, error) {
	if owner == "" && node == "" || owner != "" && !validHourlyCreditName(owner) || node != "" && !validHourlyCreditName(node) || generation < -1 || limit < 1 || limit > maxHourlyCreditProgress {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE ($1='' OR owner_id=$1) AND ($2=-1 OR account_generation=$2) AND ($3='' OR node_id=$3) AND (final_proof_digest='' OR final_recovery AND unknown_bytes>0) ORDER BY node_id,agent_id,issue_sequence LIMIT $4`, owner, generation, node, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.HourlyCreditReservation
	for rows.Next() {
		value, err := scanHourlyCreditReservation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, rows.Close()
}
