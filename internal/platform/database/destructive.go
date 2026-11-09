package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DeleteActivityGame removes current admission and catalog visibility. The
// disabled definition, settled bets, replay keys and ledger remain evidence.
func (s *Store) DeleteActivityGame(ctx context.Context, actorID, gameID string, now time.Time) error {
	return s.deleteFeature(ctx, actorID, "activity_game.delete", "activity_game", gameID, now, func(tx *sql.Tx) error {
		if err := requireRowTx(ctx, tx, `SELECT 1 FROM activity_games WHERE id=$1 AND `+currentActivityGame+` FOR UPDATE`, gameID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE activity_games SET enabled=0,updated_at=$1 WHERE id=$2`, stamp(now.UTC()), gameID)
		return err
	})
}

// DeleteLuckyDraw preserves settled results and financial evidence. Raffles
// refund every active seat and retain a cancelled tombstone for delivery cleanup.
func (s *Store) DeleteLuckyDraw(ctx context.Context, actorID, drawID string, now time.Time) error {
	return s.deleteFeature(ctx, actorID, "lucky_draw.delete", "lucky_draw", drawID, now, func(tx *sql.Tx) error {
		draw, err := lockedLuckyDrawTx(ctx, tx, drawID, false)
		if err != nil {
			return err
		}
		if draw.Kind == "instant" {
			_, err = tx.ExecContext(ctx, `DELETE FROM activity_lucky_draws WHERE id=$1`, drawID)
			return err
		}
		if draw.Status == "cancelled" {
			return ErrConflict
		}
		if draw.Status == "completed" {
			var pending int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='draw_raffle_complete'
				AND (CASE ((payload)::jsonb #> '{drawId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{drawId}' END)=$1`, drawID).Scan(&pending); err != nil {
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
		if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='cancelled',updated_at=$1 WHERE id=$2`, stamp(now), drawID); err != nil {
			return err
		}
		return insertOutboxTx(ctx, tx, "draw_telegram_delete", `{"drawId":"`+drawID+`"}`, now, now)
	})
}

func (s *Store) retryAttempt_deleteFeature(ctx context.Context, actorID, action, targetType, targetID string, now time.Time, mutate func(*sql.Tx) error) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := mutate(tx); err != nil {
		return err
	}
	auditID, err := featureDeletionAuditID(action, targetID)
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
