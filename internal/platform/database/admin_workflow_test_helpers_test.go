package database

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func createAdminWorkflowPurchase(t *testing.T, store *Store, telegramID int64, combo model.Combo,
	now time.Time, addons ...string) (model.User, model.Purchase) {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, store, telegramID)
	credit := combo.PriceTXBMinor + 10_000
	if _, err := store.AdjustBalance(ctx, user.ID, credit, fmt.Sprintf("admin-workflow-seed:%d", telegramID), "test credit", now); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
		AddonSquadIDs: addons, IdempotencyKey: "admin-workflow-purchase"}, now)
	if err != nil {
		t.Fatalf("CreatePurchase(): %v", err)
	}
	return user, activateAdminWorkflowPurchase(t, store, purchase.ID, now)
}

func activateAdminWorkflowPurchase(t *testing.T, store *Store, purchaseID string, now time.Time) model.Purchase {
	t.Helper()
	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=?,valid_until=?,updated_at=? WHERE id=?`,
		stamp(now.Add(-time.Hour)), stamp(now.Add(30*24*time.Hour)), stamp(now), purchaseID); err != nil {
		t.Fatalf("activate purchase: %v", err)
	}
	purchase, err := store.PurchaseByID(ctx, purchaseID)
	if err != nil {
		t.Fatalf("PurchaseByID(): %v", err)
	}
	return purchase
}

func adminWorkflowBalance(t *testing.T, store *Store, userID string) int64 {
	t.Helper()
	balance, err := store.Balance(context.Background(), userID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	value, err := strconv.ParseInt(balance.Minor, 10, 64)
	if err != nil {
		t.Fatalf("parse balance %q: %v", balance.Minor, err)
	}
	return value
}

func adminWorkflowLedgerCount(t *testing.T, store *Store, userID, kind string) int {
	t.Helper()
	var count int
	if err := store.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind=?`, userID, kind).Scan(&count); err != nil {
		t.Fatalf("count %s ledger entries: %v", kind, err)
	}
	return count
}
