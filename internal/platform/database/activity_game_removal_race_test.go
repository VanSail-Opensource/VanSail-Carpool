package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func waitGameRemovalLock(t *testing.T, f gameRemovalFixture, ctx context.Context, query string) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		if err := f.store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,$1)>0)`, query).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-timer.C:
			t.Fatal("expected game mutation did not wait for its row fence")
		case <-tick.C:
		}
	}
}

func TestGameEditWaitingForRemovalCannotResurrectDefinition(t *testing.T) {
	f := newGameRemovalFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	actorLock, err := f.store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer actorLock.Rollback()
	// The deletion's audit FK waits here after its game update. The real
	// removal transaction therefore retains its game row lock until released.
	if err = requireRowTx(ctx, actorLock, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, f.actor.ID); err != nil {
		t.Fatal(err)
	}
	removed := make(chan error, 1)
	go func() { removed <- f.store.DeleteActivityGame(ctx, f.actor.ID, f.game.ID, f.now) }()
	waitGameRemovalLock(t, f, ctx, "INSERT INTO audit_events")
	edited := make(chan error, 1)
	go func() {
		_, err := f.store.SaveActivityGame(ctx, f.game.GameInput, f.now.Add(time.Second))
		edited <- err
	}()
	waitGameRemovalLock(t, f, ctx, "SELECT 1 FROM activity_games WHERE id=$1 FOR UPDATE")
	if err = actorLock.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-removed; err != nil {
		t.Fatal(err)
	}
	if err = <-edited; !errors.Is(err, ErrNotFound) {
		t.Fatal("a stale typed edit modified a removed game", err)
	}
	var enabled int
	if err = f.store.DB().QueryRowContext(ctx, `SELECT enabled FROM activity_games WHERE id=$1`, f.game.ID).Scan(&enabled); err != nil || enabled != 0 {
		t.Fatal("removed definition was re-enabled by a waiting editor", err)
	}
}

type pausedGameRemovalRandom struct {
	ctx              context.Context
	entered, release chan struct{}
}

func (r pausedGameRemovalRandom) Int63n(int64) (int64, error) {
	close(r.entered)
	select {
	case <-r.release:
		return 0, nil
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

func TestGameRemovalWaitsForAdmittedBetAndPreservesItsSettlement(t *testing.T) {
	f := newGameRemovalFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rng := pausedGameRemovalRandom{ctx: ctx, entered: make(chan struct{}), release: make(chan struct{})}
	settled := make(chan error, 1)
	go func() {
		_, err := f.store.PlaceActivityBet(ctx, f.bettor.ID, f.game.ID, 100, "concurrent", rng, f.now)
		settled <- err
	}()
	select {
	case <-rng.entered:
	case <-ctx.Done():
		t.Fatal("bet did not reach its post-debit settlement boundary", ctx.Err())
	}
	removed := make(chan error, 1)
	go func() { removed <- f.store.DeleteActivityGame(ctx, f.actor.ID, f.game.ID, f.now) }()
	waitGameRemovalLock(t, f, ctx, "FOR UPDATE")
	select {
	case err := <-removed:
		t.Fatal("removal bypassed an admitted settlement", err)
	default:
	}
	close(rng.release)
	if err := <-settled; err != nil {
		t.Fatal(err)
	}
	if err := <-removed; err != nil {
		t.Fatal(err)
	}
	var balance, bets, entries int64
	if err := f.store.DB().QueryRowContext(ctx, `SELECT txb_minor,
 (SELECT COUNT(*) FROM activity_bets WHERE game_id=$2),
 (SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind IN('activity_bet_stake','activity_bet_payout'))
 FROM balances WHERE user_id=$1`, f.bettor.ID, f.game.ID).Scan(&balance, &bets, &entries); err != nil || balance != 1100 || bets != 1 || entries != 2 {
		t.Fatal("removal lost or repeated the concurrent financial settlement", err)
	}
}

func TestWaitingGameAdmissionRechecksCommittedTombstoneAfterRawReenable(t *testing.T) {
	f := newGameRemovalFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	writer, err := f.store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if err = requireRowTx(ctx, writer, `SELECT 1 FROM activity_games WHERE id=$1 FOR UPDATE`, f.game.ID); err != nil {
		t.Fatal(err)
	}
	admitted := make(chan error, 1)
	go func() {
		// Explicitly exercise statement snapshots as well as production's
		// Serializable retry behavior. The first statement begins pre-removal.
		reader, err := f.store.DB().BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err == nil {
			defer reader.Rollback()
			_, err = lockedActivityGameTx(ctx, reader, f.game.ID)
		}
		admitted <- err
	}()
	waitGameRemovalLock(t, f, ctx, "FOR SHARE")
	// Publish the final state after removal plus a raw definition re-enable.
	// Its mutable enabled flag cannot invalidate permanent audit evidence.
	if _, err = writer.ExecContext(ctx, `UPDATE activity_games SET enabled=1,updated_at=$2 WHERE id=$1`, f.game.ID, stamp(f.now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	if err = insertAuditTx(ctx, writer, activityGameRemovalPrefix+f.game.ID, &f.actor.ID, "activity_game.delete", "activity_game", f.game.ID, `{}`, f.now); err != nil {
		t.Fatal(err)
	}
	if err = writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-admitted; !errors.Is(err, ErrNotFound) {
		t.Fatal("a pre-removal statement snapshot bypassed the permanent tombstone", err)
	}
}
