package database

import (
	"context"
	"testing"
	"time"
)

func TestRenewalBatchHydrationAndReplayWithOneConnection(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	user := createTestUser(t, store, 51_021)
	combo := saveTestCombo(t, store, "single-connection-renewal", 100, 30)
	now := time.Now().UTC()
	ctx := context.Background()
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "single-connection-seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{
		UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "single-connection-source",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	store.DB().SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	input := RenewalInput{
		UserID: user.ID, PurchaseID: source.ID, TermCount: 2, IdempotencyKey: "single-connection-renewal",
	}
	batch, err := store.Renew(ctx, input, now)
	if err != nil {
		t.Fatalf("renew with one connection: %v", err)
	}
	if batch.PurchaseID != source.ID || batch.TermCount != 2 || batch.TotalPrice.MinorInt64() != 200 || len(batch.Purchases) != 2 {
		t.Fatalf("batch = %+v", batch)
	}
	for index, purchase := range batch.Purchases {
		from := source.ValidUntil.AddDate(0, 0, 30*index)
		if !purchase.ValidFrom.Equal(from) || !purchase.ValidUntil.Equal(from.AddDate(0, 0, 30)) || purchase.PriceTXBMinor != 100 {
			t.Fatalf("term %d = %+v", index, purchase)
		}
	}
	replay, err := store.Renew(ctx, input, now.Add(time.Minute))
	if err != nil || replay.ID != batch.ID || len(replay.Purchases) != 2 {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	for index := range batch.Purchases {
		if replay.Purchases[index].ID != batch.Purchases[index].ID {
			t.Fatalf("replay changed purchase order: %+v", replay)
		}
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.MinorInt64() != 200 {
		t.Fatalf("balance = %+v, %v", balance, err)
	}
	var debits int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries WHERE user_id=$1 AND kind='purchase_debit' AND reference_id=$2`, user.ID, batch.ID).Scan(&debits); err != nil || debits != 1 {
		t.Fatalf("batch ledger entries = %d, %v", debits, err)
	}
}
