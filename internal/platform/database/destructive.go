package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// DeleteActivityGame removes feature rows and their ledger evidence without
// reversing any already-settled balance effects.
func (s *Store) DeleteActivityGame(ctx context.Context, actorID, gameID string, now time.Time) error {
	return s.deleteFeature(ctx, actorID, "activity_game.delete", "activity_game", gameID, now, func(tx *sql.Tx) error {
		if err := requireRowTx(ctx, tx, `SELECT 1 FROM activity_games WHERE id=?`, gameID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM ledger_entries WHERE kind IN ('activity_bet_stake','activity_bet_payout') AND reference_id IN (SELECT id FROM activity_bets WHERE game_id=?)`, gameID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_bets WHERE game_id=?`, gameID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM activity_games WHERE id=?`, gameID)
		return err
	})
}

// DeleteLuckyDraw preserves settled results and financial evidence. Raffles
// refund every active seat and retain a cancelled tombstone for delivery cleanup.
func (s *Store) DeleteLuckyDraw(ctx context.Context, actorID, drawID string, now time.Time) error {
	return s.deleteFeature(ctx, actorID, "lucky_draw.delete", "lucky_draw", drawID, now, func(tx *sql.Tx) error {
		draw, err := luckyDrawByID(ctx, tx, drawID, false)
		if err != nil {
			return err
		}
		if draw.Kind == "instant" {
			_, err = tx.ExecContext(ctx, `DELETE FROM activity_lucky_draws WHERE id=?`, drawID)
			return err
		}
		if draw.Status == "cancelled" {
			return ErrConflict
		}
		if draw.Status == "completed" {
			var pending int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='draw_raffle_complete'
				AND json_extract(payload,'$.drawId')=?`, drawID).Scan(&pending); err != nil {
				return err
			}
			if pending > 0 {
				return ErrConflict
			}
		}
		tickets, err := activeRaffleTicketsTx(ctx, tx, drawID)
		if err != nil {
			return err
		}
		for _, ticket := range tickets {
			if err = refundRaffleTicketTx(ctx, tx, ticket, draw.Name, now); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='cancelled',updated_at=? WHERE id=?`, stamp(now), drawID); err != nil {
			return err
		}
		return insertOutboxTx(ctx, tx, "draw_telegram_delete", `{"drawId":"`+drawID+`"}`, now, now)
	})
}

func (s *Store) retryAttempt_deleteFeature(ctx context.Context, actorID, action, targetType, targetID string, now time.Time, mutate func(*sql.Tx) error) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := mutate(tx); err != nil {
		return err
	}
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, action, targetType, targetID, `{}`, now.UTC()); err != nil {
		return fmt.Errorf("append deletion audit: %w", err)
	}
	return tx.Commit()
}

func requireRowTx(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	var exists int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
