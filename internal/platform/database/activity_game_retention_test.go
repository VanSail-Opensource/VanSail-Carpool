package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGameRemovalTombstoneSurvivesMaintenanceActorPurgeAndRawEdits(t *testing.T) {
	f, ctx := newGameRemovalFixture(t), context.Background()
	if _, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "retained", fixedActivityRandom{}, f.now); err != nil {
		t.Fatal(err)
	}
	before := removalLedgerSnapshot(t, f)
	if err := f.store.DeleteActivityGame(ctx, f.actor.ID, f.game.ID, f.now.Add(-365*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CompactAndPrune(ctx, f.now.Add(-7*24*time.Hour), f.now.Add(-24*time.Hour), f.now); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, f.store, f.actor, f.now)
	if err := f.store.CompleteAccountPurge(ctx, f.actor.ID, 1, f.now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var tombstones int
	if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE id=$1 AND action='activity_game.delete' AND target_id=$2`, activityGameRemovalPrefix+f.game.ID, f.game.ID).Scan(&tombstones); err != nil || tombstones != 1 {
		t.Fatal("retention or actor purge removed the admission tombstone", err)
	}
	// The reviewed editor can change ordinary game fields, but cannot modify
	// protected audit evidence. Such a change must not restore live admission.
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE activity_games SET enabled=1 WHERE id=$1`, f.game.ID); err != nil {
		t.Fatal(err)
	}
	for _, enabledOnly := range []bool{true, false} {
		games, err := f.store.ListActivityGames(ctx, enabledOnly)
		if err != nil || len(games) != 0 {
			t.Fatal("raw edit restored a removed catalog entry", err)
		}
	}
	if _, err := f.store.SaveActivityGame(ctx, f.game.GameInput, f.now); !errors.Is(err, ErrNotFound) {
		t.Fatal("raw edit bypassed typed tombstone guard", err)
	}
	rng := &countingActivityRandom{}
	if _, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "late", rng, f.now); !errors.Is(err, ErrNotFound) || rng.calls != 0 {
		t.Fatal("raw edit admitted a new financial mutation", err)
	}
	if replay, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "retained", rng, f.now); err != nil || !replay.Replayed || rng.calls != 0 {
		t.Fatal("durable removal lost a settled replay", err)
	}
	if removalLedgerSnapshot(t, f) != before {
		t.Fatal("maintenance, actor cleanup or blocked edits changed bettor ledger")
	}
}

func TestGameRemovalFailureDoesNotPublishPartialTombstoneOrDisable(t *testing.T) {
	f, ctx := newGameRemovalFixture(t), context.Background()
	// A failed audit FK must roll back the definition update as well.
	if err := f.store.DeleteActivityGame(ctx, "missing-actor", f.game.ID, f.now); err == nil {
		t.Fatal("removal accepted a missing audit actor")
	}
	games, err := f.store.ListActivityGames(ctx, true)
	if err != nil || len(games) != 1 || games[0].ID != f.game.ID {
		t.Fatal("failed removal changed current admission", err)
	}
	var tombstones int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE id=$1`, activityGameRemovalPrefix+f.game.ID).Scan(&tombstones); err != nil || tombstones != 0 {
		t.Fatal("failed removal published permanent evidence", err)
	}
}
