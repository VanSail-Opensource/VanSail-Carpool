package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// SaveLuckyDraw is the raw fixture adapter. Administrative requests use
// SaveAdminLuckyDraw so the current actor and audit share this transaction.
func (s *Store) retryAttempt_SaveLuckyDraw(ctx context.Context, input activity.LuckyDrawInput, now time.Time) (activity.LuckyDraw, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	defer tx.Rollback()
	draw, err := saveLuckyDrawTx(ctx, tx, input, now.UTC())
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.LuckyDraw{}, err
	}
	return draw, nil
}
