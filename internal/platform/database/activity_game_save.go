package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SaveActivityGame creates or edits a current definition without resurrecting a
// removed game. A short SQL transaction serializes edits with admission/removal.
func (s *Store) SaveActivityGame(ctx context.Context, input activity.GameInput, now time.Time) (activity.Game, error) {
	if err := input.Validate(); err != nil {
		return activity.Game{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Icon = strings.TrimSpace(input.Icon)
	input.Description = strings.TrimSpace(input.Description)
	create := input.ID == ""
	if create {
		id, err := ids.New()
		if err != nil {
			return activity.Game{}, err
		}
		input.ID = id
	}
	for retry := 0; ; retry++ {
		game, err := s.saveActivityGameAttempt(ctx, input, create, now.UTC())
		if !s.retryTransaction(ctx, err, retry) {
			return game, err
		}
	}
}

func (s *Store) saveActivityGameAttempt(ctx context.Context, input activity.GameInput, create bool, now time.Time) (activity.Game, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.Game{}, err
	}
	defer tx.Rollback()
	if create {
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_games(id,name,icon,description,enabled,win_chance_bps,minimum_stake_minor,maximum_stake_minor,return_multiplier_bps,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, input.ID, input.Name, input.Icon, input.Description, boolInt(input.Enabled), input.WinChanceBPS,
			input.MinimumStakeMinor, input.MaximumStakeMinor, input.ReturnMultiplierBPS, stamp(now), stamp(now))
	} else {
		// Lock first, then inspect the tombstone. This also handles an editor
		// whose statement began before a concurrent removal committed.
		if err = requireRowTx(ctx, tx, `SELECT 1 FROM activity_games WHERE id=$1 FOR UPDATE`, input.ID); err != nil {
			return activity.Game{}, err
		}
		if _, err = activityGameByID(ctx, tx, input.ID); err != nil {
			return activity.Game{}, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE activity_games SET name=$1,icon=$2,description=$3,enabled=$4,win_chance_bps=$5,minimum_stake_minor=$6,maximum_stake_minor=$7,return_multiplier_bps=$8,updated_at=$9 WHERE id=$10`,
			input.Name, input.Icon, input.Description, boolInt(input.Enabled), input.WinChanceBPS, input.MinimumStakeMinor, input.MaximumStakeMinor, input.ReturnMultiplierBPS, stamp(now), input.ID)
	}
	if err != nil {
		return activity.Game{}, fmt.Errorf("save activity game: %w", err)
	}
	game, err := activityGameByID(ctx, tx, input.ID)
	if err != nil {
		return activity.Game{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.Game{}, err
	}
	return game, nil
}
