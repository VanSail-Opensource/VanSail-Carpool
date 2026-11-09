package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type gameRemovalFixture struct {
	store         *Store
	actor, bettor model.User
	game          activity.Game
	now           time.Time
}

func newGameRemovalFixture(t *testing.T) gameRemovalFixture {
	t.Helper()
	f := gameRemovalFixture{store: newTestStore(t), now: time.Now().UTC()}
	f.actor = createTestUser(t, f.store, 398101)
	f.bettor = createTestUser(t, f.store, 398102)
	if _, err := f.store.AdjustBalance(context.Background(), f.bettor.ID, 1000, "game-removal-seed", "fixture", f.now); err != nil {
		t.Fatal(err)
	}
	var err error
	f.game, err = f.store.SaveActivityGame(context.Background(), activity.GameInput{Name: "Retained coin", Icon: "coin", Enabled: true,
		WinChanceBPS: 5000, MinimumStakeMinor: 100, MaximumStakeMinor: 500, ReturnMultiplierBPS: 20000}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func removalLedgerSnapshot(t *testing.T, f gameRemovalFixture) string {
	t.Helper()
	var raw string
	if err := f.store.DB().QueryRowContext(context.Background(), `SELECT COALESCE(jsonb_agg(to_jsonb(l) ORDER BY l.id),'[]'::jsonb)::text FROM ledger_entries l WHERE l.user_id=$1`, f.bettor.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGameRemovalPreservesSettledOutcomesLedgerAndReplays(t *testing.T) {
	t.Parallel()
	for _, roll := range []int64{0, 9999} {
		t.Run(map[bool]string{true: "win", false: "loss"}[roll == 0], func(t *testing.T) {
			f, ctx := newGameRemovalFixture(t), context.Background()
			rng := &countingActivityRandom{value: roll}
			played, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "settled-bet", rng, f.now)
			if err != nil {
				t.Fatal(err)
			}
			before := removalLedgerSnapshot(t, f)
			balance, err := f.store.Balance(ctx, f.bettor.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.store.DeleteActivityGame(ctx, f.actor.ID, f.game.ID, f.now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			for _, enabledOnly := range []bool{true, false} {
				games, err := f.store.ListActivityGames(ctx, enabledOnly)
				if err != nil || len(games) != 0 {
					t.Fatal("removed game remained in a current catalog", enabledOnly, err)
				}
			}
			var enabled int
			if err = f.store.DB().QueryRowContext(ctx, `SELECT enabled FROM activity_games WHERE id=$1`, f.game.ID).Scan(&enabled); err != nil || enabled != 0 {
				t.Fatal("historical definition was removed or enabled", err)
			}
			replayed, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "settled-bet", rng, f.now.Add(time.Second))
			played.Replayed = true
			if err != nil || !reflect.DeepEqual(played, replayed) || rng.calls != 1 {
				t.Fatal("settled replay was lost or sampled again", err, rng.calls)
			}
			if _, err = f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "new-bet", rng, f.now); !errors.Is(err, ErrNotFound) {
				t.Fatal("removed game admitted a new bet", err)
			}
			if _, err = f.store.SaveActivityGame(ctx, f.game.GameInput, f.now); !errors.Is(err, ErrNotFound) {
				t.Fatal("typed edit resurrected removed game", err)
			}
			if err = f.store.DeleteActivityGame(ctx, f.actor.ID, f.game.ID, f.now); !errors.Is(err, ErrNotFound) {
				t.Fatal("repeated removal replaced permanent evidence", err)
			}
			afterBalance, err := f.store.Balance(ctx, f.bettor.ID)
			if err != nil || afterBalance != balance || removalLedgerSnapshot(t, f) != before {
				t.Fatal("removal changed settled financial evidence", err)
			}
			var exact bool
			if err = f.store.DB().QueryRowContext(ctx, `SELECT b.txb_minor=(SELECT SUM(delta_txb_minor) FROM ledger_entries WHERE user_id=b.user_id) FROM balances b WHERE b.user_id=$1`, f.bettor.ID).Scan(&exact); err != nil || !exact {
				t.Fatal("retained ledger no longer reconciles", err)
			}
		})
	}
}

func TestBetReplayCannotBeRelabeledAsAnotherGame(t *testing.T) {
	f, ctx := newGameRemovalFixture(t), context.Background()
	rng := &countingActivityRandom{}
	if _, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "request", rng, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, "another-game", 100, "request", rng, f.now); !errors.Is(err, ErrConflict) || rng.calls != 1 {
		t.Fatal("replay crossed a game boundary", err, rng.calls)
	}
}
