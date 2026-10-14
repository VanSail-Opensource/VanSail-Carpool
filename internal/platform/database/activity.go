package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
)

const activityGameSelect = `SELECT id,name,icon,description,enabled,win_chance_bps,minimum_stake_minor,maximum_stake_minor,
	return_multiplier_bps,created_at,updated_at FROM activity_games`

// SaveActivityGame creates or updates one betting game.

func (s *Store) SaveActivityGame(ctx context.Context, input activity.GameInput, now time.Time) (activity.Game, error) {
	if err := input.Validate(); err != nil {
		return activity.Game{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Icon = strings.TrimSpace(input.Icon)
	input.Description = strings.TrimSpace(input.Description)
	now = now.UTC()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var err error
	if input.ID == "" {
		input.ID, err = ids.New()
		if err != nil {
			return activity.Game{}, err
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO activity_games(id,name,icon,description,enabled,win_chance_bps,minimum_stake_minor,maximum_stake_minor,return_multiplier_bps,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, input.ID, input.Name, input.Icon, input.Description, boolInt(input.Enabled), input.WinChanceBPS, input.MinimumStakeMinor,
			input.MaximumStakeMinor, input.ReturnMultiplierBPS, stamp(now), stamp(now))
	} else {
		var result sql.Result
		result, err = s.db.ExecContext(ctx, `UPDATE activity_games SET name=?,icon=?,description=?,enabled=?,win_chance_bps=?,minimum_stake_minor=?,maximum_stake_minor=?,return_multiplier_bps=?,updated_at=? WHERE id=?`,
			input.Name, input.Icon, input.Description, boolInt(input.Enabled), input.WinChanceBPS, input.MinimumStakeMinor, input.MaximumStakeMinor, input.ReturnMultiplierBPS, stamp(now), input.ID)
		if err == nil {
			if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
				return activity.Game{}, rowsErr
			} else if affected == 0 {
				return activity.Game{}, ErrNotFound
			}
		}
	}
	if err != nil {
		return activity.Game{}, fmt.Errorf("save activity game: %w", err)
	}
	return activityGameByID(ctx, s.db, input.ID, false)
}

// ListActivityGames returns enabled member games or every administrator game.

func (s *Store) ListActivityGames(ctx context.Context, enabledOnly bool) ([]activity.Game, error) {
	query := activityGameSelect
	if enabledOnly {
		query += ` WHERE enabled=1`
	}
	query += ` ORDER BY name,id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list activity games: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]activity.Game, 0)
	for rows.Next() {
		game, scanErr := scanActivityGame(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, game)
	}
	return result, rows.Err()
}

// PlaceActivityBet atomically debits the stake before selecting and recording an outcome.

func (s *Store) PlaceActivityBet(ctx context.Context, userID, gameID string, stakeMinor int64, idempotencyKey string, rng activity.RandomSource, now time.Time) (activity.BetResult, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(gameID) == "" || stakeMinor <= 0 || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 128 || rng == nil {
		return activity.BetResult{}, activity.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.BetResult{}, fmt.Errorf("begin activity bet: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := activityBetByKeyTx(ctx, tx, userID, idempotencyKey); loadErr == nil {
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return activity.BetResult{}, loadErr
	}
	game, err := activityGameByID(ctx, tx, gameID, true)
	if err != nil {
		return activity.BetResult{}, err
	}
	if stakeMinor < game.MinimumStakeMinor || stakeMinor > game.MaximumStakeMinor {
		return activity.BetResult{}, activity.ErrInvalidInput
	}
	betID, err := ids.New()
	if err != nil {
		return activity.BetResult{}, err
	}
	balance, err := changeBalanceTx(ctx, tx, userID, -stakeMinor, now)
	if err != nil {
		return activity.BetResult{}, err
	}
	roll, err := rng.Int63n(10_000)
	if err != nil {
		return activity.BetResult{}, fmt.Errorf("select activity bet outcome: %w", err)
	}
	won := roll < int64(game.WinChanceBPS)
	payout := int64(0)
	if won {
		payout, err = fixedMultiplyFloor(stakeMinor, game.ReturnMultiplierBPS, 10_000)
		if err != nil {
			return activity.BetResult{}, err
		}
		balance, err = changeBalanceTx(ctx, tx, userID, payout, now)
		if err != nil {
			return activity.BetResult{}, err
		}
	}
	snapshot, err := json.Marshal(game)
	if err != nil {
		return activity.BetResult{}, fmt.Errorf("encode activity game snapshot: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_bets(id,user_id,game_id,stake_minor,won,payout_minor,balance_after_minor,configuration_snapshot,idempotency_key,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, betID, userID, game.ID, stakeMinor, boolInt(won), payout, balance, string(snapshot), idempotencyKey, stamp(now))
	if err != nil {
		return activity.BetResult{}, fmt.Errorf("record activity bet: %w", err)
	}
	stakeBalance := balance
	if won {
		stakeBalance -= payout
	}
	if _, err := insertLedgerTx(ctx, tx, userID, -stakeMinor, stakeBalance, "activity_bet_stake", betID, game.Name, now); err != nil {
		return activity.BetResult{}, err
	}
	if won {
		if _, err := insertLedgerTx(ctx, tx, userID, payout, balance, "activity_bet_payout", betID, game.Name, now); err != nil {
			return activity.BetResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return activity.BetResult{}, fmt.Errorf("commit activity bet: %w", err)
	}
	return s.activityBetByID(ctx, betID)
}

// ClaimDailyActivity credits at most one reward per configured local calendar date.

func (s *Store) ClaimDailyActivity(ctx context.Context, userID, localDate, timezone string, rewardMinor int64, now time.Time) (activity.DailyCheckIn, error) {
	return s.claimDailyActivity(ctx, userID, localDate, timezone, rewardMinor, rewardMinor, 2, nil, now)
}

// ClaimDailyActivityRange chooses the inclusive reward exactly once, after the
// idempotency row check and while holding the write transaction lock.

func (s *Store) ClaimDailyActivityRange(ctx context.Context, userID, localDate, timezone string, minimumMinor, maximumMinor int64, rng activity.RandomSource, now time.Time) (activity.DailyCheckIn, error) {
	return s.claimDailyActivity(ctx, userID, localDate, timezone, minimumMinor, maximumMinor, 2, rng, now)
}

// ClaimBoostedDailyActivityRange scales the sampled base reward inside settlement.
func (s *Store) ClaimBoostedDailyActivityRange(ctx context.Context, userID, localDate, timezone string, minimumMinor, maximumMinor int64, boostCount int, rng activity.RandomSource, now time.Time) (activity.DailyCheckIn, error) {
	if boostCount <= 0 {
		return activity.DailyCheckIn{}, activity.ErrGroupBoostRequired
	}
	return s.claimDailyActivity(ctx, userID, localDate, timezone, minimumMinor, maximumMinor, boostCount, rng, now)
}
