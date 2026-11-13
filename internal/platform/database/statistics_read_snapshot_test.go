package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestProductStatisticsUseOneSnapshotDuringMembershipChange(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	at := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "Snapshot conversion", 100, 30)
	createAdminWorkflowPurchase(t, store, 93_301, combo, at)
	other, _ := createAdminWorkflowPurchase(t, store, 93_302, combo, at)
	usageBreakdownExec(t, store, `UPDATE users SET role='admin' WHERE id=$1`, other.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gate, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	if _, err = gate.ExecContext(ctx, `LOCK TABLE purchases IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		value model.DatabaseStatistics
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		value, err := store.ProductDatabaseStatistics(ctx, at)
		done <- outcome{value, err}
	}()
	// The user count has completed before the buyer query waits. Changing a
	// role now must not produce two buyers divided by one previously counted user.
	waitStatisticQueryLock(t, ctx, store, "SELECT COUNT(*) FROM users member")
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	if err = gate.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case first := <-done:
		if first.err != nil || first.value.NewUserConversion != 100 || shareTotal(first.value.SubscriptionStates) != 1 || shareTotal(first.value.ComboShares) != 1 {
			t.Fatalf("membership change mixed report snapshots: %+v, %v", first.value, first.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second, err := store.ProductDatabaseStatistics(ctx, at)
	if err != nil || second.NewUserConversion != 100 || shareTotal(second.SubscriptionStates) != 2 || shareTotal(second.ComboShares) != 2 {
		t.Fatalf("next report missed committed membership: %+v, %v", second, err)
	}
}

func TestComboStatisticsKeepTotalsAndDistributionInOneSnapshot(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	at := time.Date(2026, 11, 13, 19, 0, 0, 0, time.UTC)
	squad := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 0, true)
	combo := saveTestCombo(t, store, "Snapshot distribution", 100, 30, squad.ID)
	_, purchase := createAdminWorkflowPurchase(t, store, 93_303, combo, at)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gate, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	if _, err = gate.ExecContext(ctx, `LOCK TABLE combos IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		value model.AdminStatistics
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		value, err := store.ComboStatistics(ctx, combo.ID, at.Add(-time.Minute), at.Add(time.Minute), "daily", time.UTC)
		done <- outcome{value, err}
	}()
	waitStatisticQueryLock(t, ctx, store, "SELECT kind,uuid,COUNT(*) FROM")
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=$1`, purchase.ID); err != nil {
		t.Fatal(err)
	}
	if err = gate.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case first := <-done:
		if first.err != nil || first.value.Count != 1 || first.value.InputMinor != 100 || len(first.value.Distribution) != 1 || first.value.Distribution[0].Count != 1 {
			t.Fatalf("cancellation mixed report snapshots: %+v, %v", first.value, first.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second, err := store.ComboStatistics(ctx, combo.ID, at.Add(-time.Minute), at.Add(time.Minute), "daily", time.UTC)
	if err != nil || second.Count != 0 || len(second.Distribution) != 0 {
		t.Fatalf("next report missed committed cancellation: %+v, %v", second, err)
	}
}

func waitStatisticQueryLock(t *testing.T, ctx context.Context, store *Store, queryPart string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,$1)>0)`, queryPart).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("report did not establish its snapshot before the relation wait")
}
