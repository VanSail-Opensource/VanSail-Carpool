package database

import (
	"context"
	"testing"
	"time"
)

func TestAdminBalanceReferenceReplayIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 31982)
	admin := createTestUser(t, store, 31983)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 13, 0, 0, 0, time.UTC)
	first, err := store.AdjustAdminBalance(ctx, admin.ID, user.ID, 250, "telegram-pm:addtxb:600", "PM command", now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := store.AdjustAdminBalance(ctx, admin.ID, user.ID, 250, "telegram-pm:addtxb:600", "PM command", now.Add(time.Second))
	if err != nil || replay.ID != first.ID {
		t.Fatalf("adjust replay = %+v, %v", replay, err)
	}
	if _, err := store.AdjustAdminBalance(ctx, admin.ID, user.ID, 300, "telegram-pm:addtxb:600", "PM command", now.Add(2*time.Second)); err == nil {
		t.Fatal("reference reuse with a different amount succeeded")
	}
	deducted, err := store.DeductAdminBalance(ctx, admin.ID, user.ID, 100, "telegram-pm:deducttxb:601", "PM command", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	deductReplay, err := store.DeductAdminBalance(ctx, admin.ID, user.ID, 100, "telegram-pm:deducttxb:601", "PM command", now.Add(4*time.Second))
	if err != nil || deductReplay.ID != deducted.ID {
		t.Fatalf("deduct replay = %+v, %v", deductReplay, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "150" {
		t.Fatalf("balance = %+v, %v", balance, err)
	}
	entries, err := store.ListLedger(ctx, user.ID, 10)
	if err != nil || len(entries) != 2 {
		t.Fatalf("ledger rows = %d, %v", len(entries), err)
	}
}
