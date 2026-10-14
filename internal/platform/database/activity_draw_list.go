package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// ListLuckyDraws returns only open instant draws to members.
func (s *Store) ListLuckyDraws(ctx context.Context, enabledOnly bool) ([]activity.LuckyDraw, error) {
	query := luckyDrawSelect
	if enabledOnly {
		query += ` WHERE kind='instant' AND status='open'`
	} else {
		query += ` WHERE status<>'cancelled'`
	}
	query += ` ORDER BY name,id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	draws := make([]activity.LuckyDraw, 0)
	for rows.Next() {
		draw, scanErr := scanLuckyDraw(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		draws = append(draws, draw)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range draws {
		draws[index].Prizes, err = luckyPrizes(ctx, s.db, draws[index].ID, false)
		if err != nil {
			return nil, err
		}
		if draws[index].Kind == "raffle" {
			draws[index].Seats, err = s.RaffleProgress(ctx, draws[index].ID)
			if err != nil {
				return nil, err
			}
		}
	}
	return draws, nil
}
