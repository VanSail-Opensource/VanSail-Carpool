package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const activityGameRemovalPrefix = "activity-game-delete:"

// The immutable, primary-keyed audit tombstone survives retention and account
// cleanup. Check it independently of mutable game fields and raw editor writes.
const currentActivityGame = `NOT EXISTS(SELECT 1 FROM audit_events removed WHERE removed.id='activity-game-delete:'||activity_games.id)`

func featureDeletionAuditID(action, target string) (string, error) {
	if action == "activity_game.delete" {
		return activityGameRemovalPrefix + target, nil
	}
	return ids.New()
}

// Shared game admission precedes the wallet lock; definition edits and removal
// take an incompatible row lock. A settled replay never needs this live gate.
func lockedActivityGameTx(ctx context.Context, tx *sql.Tx, id string) (activity.Game, error) {
	if err := requireRowTx(ctx, tx, `SELECT 1 FROM activity_games WHERE id=$1 FOR SHARE`, id); err != nil {
		return activity.Game{}, err
	}
	// A lock waiter must inspect the audit in a new statement: a prior
	// ReadCommitted statement snapshot can predate removal and a raw re-enable.
	return scanActivityGame(tx.QueryRowContext(ctx, activityGameSelect+` WHERE id=$1 AND enabled=1 AND `+currentActivityGame, id))
}
