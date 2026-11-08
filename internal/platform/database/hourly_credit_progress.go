package database

import (
	"context"
	"database/sql"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const maxHourlyCreditProgress = 128

// RecordHourlyCreditProgress applies one authenticated admitted-byte maximum.
func (s *Store) RecordHourlyCreditProgress(ctx context.Context, node, agent string, progress model.HourlyCreditProgress) (model.HourlyCreditReservation, error) {
	values, err := s.RecordHourlyCreditProgressBatch(ctx, node, agent, []model.HourlyCreditProgress{progress})
	if err != nil {
		return model.HourlyCreditReservation{}, err
	}
	return values[0], nil
}

// RecordHourlyCreditProgressBatch commits a bounded frame atomically. It locks
// reservations and then distinct owner/day rows in deterministic order, without
// locking account admission: old generations must still settle after closure.
func (s *Store) RecordHourlyCreditProgressBatch(ctx context.Context, node, agent string, progress []model.HourlyCreditProgress) ([]model.HourlyCreditReservation, error) {
	if !validHourlyCreditName(node) || !validHourlyCreditHex(agent, 32) || len(progress) > maxHourlyCreditProgress {
		return nil, ErrConflict
	}
	if len(progress) == 0 {
		return nil, nil
	}
	progress = append([]model.HourlyCreditProgress(nil), progress...)
	ids, seen := make([]string, 0, len(progress)), map[string]bool{}
	for _, p := range progress {
		if !validHourlyCreditName(p.ReservationID) || p.IssueSequence <= 0 || p.AdmittedUploadBytes < 0 || p.AdmittedDownloadBytes < 0 || seen[p.ReservationID] {
			return nil, ErrConflict
		}
		ids, seen[p.ReservationID] = append(ids, p.ReservationID), true
	}
	sort.Strings(ids)
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) ([]model.HourlyCreditReservation, error) {
		stored, err := hourlyCreditProgressRowsTx(ctx, tx, ids)
		if err != nil {
			return nil, err
		}
		result := make([]model.HourlyCreditReservation, 0, len(progress))
		var changed []model.HourlyCreditReservation
		for _, p := range progress {
			previous := stored[p.ReservationID]
			if previous.NodeID != node || previous.AgentID != agent || previous.IssueSequence != p.IssueSequence {
				return nil, ErrConflict
			}
			next, err := nextHourlyCreditProgress(previous, p)
			if err != nil {
				return nil, err
			}
			result = append(result, next)
			if next.AdmittedUploadBytes != previous.AdmittedUploadBytes || next.AdmittedDownloadBytes != previous.AdmittedDownloadBytes {
				changed = append(changed, next)
			}
		}
		if err = applyHourlyCreditProgressTx(ctx, tx, stored, changed); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func nextHourlyCreditProgress(r model.HourlyCreditReservation, p model.HourlyCreditProgress) (model.HourlyCreditReservation, error) {
	up, down := max(r.AdmittedUploadBytes, p.AdmittedUploadBytes), max(r.AdmittedDownloadBytes, p.AdmittedDownloadBytes)
	if up > r.ByteCeiling || down > r.ByteCeiling-up || r.FinalProofDigest != "" && !r.Recovery && (up != r.AdmittedUploadBytes || down != r.AdmittedDownloadBytes) {
		return r, ErrConflict
	}
	r.AdmittedUploadBytes, r.AdmittedDownloadBytes = up, down
	if r.Recovery {
		r.UnknownBytes = r.ByteCeiling - up - down
	}
	return r, nil
}

func hourlyCreditProgressRowsTx(ctx context.Context, tx *sql.Tx, ids []string) (map[string]model.HourlyCreditReservation, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE reservation_id=ANY($1::text[]) ORDER BY reservation_id FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]model.HourlyCreditReservation, len(ids))
	for rows.Next() {
		value, err := scanHourlyCreditReservation(rows)
		if err != nil {
			return nil, err
		}
		values[value.ReservationID] = value
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(values) != len(ids) {
		return nil, ErrNotFound
	}
	return values, rows.Close()
}
