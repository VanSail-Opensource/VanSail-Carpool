package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// SaveAdminActivityGame binds the current administrator and full committed game
// terms to the same transaction as the catalog write.
func (s *Store) retryAttempt_SaveAdminActivityGame(ctx context.Context, actorID string, input activity.GameInput, now time.Time) (activity.Game, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.Game{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return activity.Game{}, err
	}
	input, create, err := prepareActivityGame(input)
	if err != nil {
		return activity.Game{}, err
	}
	game, err := saveActivityGameTx(ctx, tx, input, create, now.UTC())
	if err != nil {
		return activity.Game{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "activity_game.save", "activity_game", game.ID, game, now.UTC()); err != nil {
		return activity.Game{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.Game{}, err
	}
	return game, nil
}

// SaveAdminLuckyDraw commits prizes, revision history, reservations, delivery
// jobs and the acting administrator's audit together.
func (s *Store) retryAttempt_SaveAdminLuckyDraw(ctx context.Context, actorID string, input activity.LuckyDrawInput, now time.Time) (activity.LuckyDraw, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return activity.LuckyDraw{}, err
	}
	draw, err := saveLuckyDrawTx(ctx, tx, input, now.UTC())
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "lucky_draw.save", "lucky_draw", draw.ID, draw, now.UTC()); err != nil {
		return activity.LuckyDraw{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.LuckyDraw{}, err
	}
	return draw, nil
}

// PublishAdminRaffle records the exact revision whose announcement was queued.
func (s *Store) retryAttempt_PublishAdminRaffle(ctx context.Context, actorID, id string, groupID int64, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	draw, err := publishRaffleTx(ctx, tx, id, groupID, now.UTC())
	if err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "lucky_draw.publish", "lucky_draw", id, map[string]any{
		"revision": draw.Revision, "groupChatId": groupID,
	}, now.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
