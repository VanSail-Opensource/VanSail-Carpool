package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Purge admission supplies the exact current lifecycle generation separately;
// this bounded accounting reader can only inspect older retained obligations.
func (s *Store) OutstandingHourlyCreditsThrough(ctx context.Context, owner string, through int64, limit int) ([]model.HourlyCreditReservation, error) {
	if owner == "" || through < 0 || limit < 1 || limit > 128 {
		return nil, xnode.ErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+hourlyCreditReservationColumns+` FROM hourly_credit_reservations WHERE owner_id=$1 AND account_generation<=$2 AND (final_proof_digest='' OR final_recovery AND unknown_bytes>0) ORDER BY node_id,agent_id,issue_sequence LIMIT $3`, owner, through, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.HourlyCreditReservation{}
	for rows.Next() {
		r, e := scanHourlyCreditReservation(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
