package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyCreditReservationColumns = `reservation_id,owner_id,account_generation,day_utc::text,node_id,agent_id,boot_id,session_id,kind,byte_ceiling,issue_sequence,admitted_upload_bytes,admitted_download_bytes,unused_bytes,unknown_bytes,final_proof_digest,final_recovery,created_at,finalized_at`

// HourlyCreditReservation reads an immutable obligation and its current projection.
func (s *Store) HourlyCreditReservation(ctx context.Context, id string) (model.HourlyCreditReservation, error) {
	value, err := scanHourlyCreditReservation(s.db.QueryRowContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE reservation_id=$1`, id))
	if errors.Is(err, ErrNotFound) {
		err = errors.Join(err, model.ErrHourlyCreditNotFound)
	}
	return value, err
}

// UnfinalizedHourlyCredits pages original obligations for reconnect recovery.
// A finalized recovery's unknown residual remains retained, never reclassified
// as an absent installation or reclaimed merely because its session ended.
func (s *Store) UnfinalizedHourlyCredits(ctx context.Context, node, agent string, afterIssue int64, limit int) ([]model.HourlyCreditReservation, error) {
	if !validHourlyCreditName(node) || !validHourlyCreditHex(agent, 32) || afterIssue < 0 || limit < 1 || limit > maxHourlyCreditProgress {
		return nil, ErrConflict
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE node_id=$1 AND agent_id=$2 AND final_proof_digest='' AND issue_sequence>$3 ORDER BY issue_sequence LIMIT $4`, node, agent, afterIssue, limit)
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

func hourlyCreditReservationTx(ctx context.Context, tx *sql.Tx, id string) (model.HourlyCreditReservation, error) {
	return scanHourlyCreditReservation(tx.QueryRowContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE reservation_id=$1 FOR UPDATE`, id))
}

func scanHourlyCreditReservation(row rowScanner) (model.HourlyCreditReservation, error) {
	var r model.HourlyCreditReservation
	var created string
	var final sql.NullString
	err := row.Scan(&r.ReservationID, &r.OwnerID, &r.AccountGeneration, &r.DayUTC, &r.NodeID, &r.AgentID, &r.BootID, &r.SessionID, &r.Kind, &r.ByteCeiling, &r.IssueSequence, &r.AdmittedUploadBytes, &r.AdmittedDownloadBytes, &r.UnusedBytes, &r.UnknownBytes, &r.FinalProofDigest, &r.Recovery, &created, &final)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if r.CreatedAt, err = parseStamp(created); err != nil {
		return r, err
	}
	if final.Valid {
		at, err := parseStamp(final.String)
		if err != nil {
			return r, err
		}
		r.FinalizedAt = &at
	}
	return r, nil
}
