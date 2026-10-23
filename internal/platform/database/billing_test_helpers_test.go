package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Full-schema renames validate every SQLite relation under the race detector;
// retain a bounded setup budget separate from each business operation deadline.
const testDatabaseOpenTimeout = 90 * time.Second

// testDatabaseOpenGate prevents parallel tests from compiling and applying the
// full SQLite schema simultaneously under the race detector. The timeout starts
// only after a test enters the gate, so scheduler contention cannot consume it.
var testDatabaseOpenGate = make(chan struct{}, 2)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	return NewStore(openTestDatabase(t, "tx-carpool-test.db"))
}

func openTestDatabase(t *testing.T, filename string) *sql.DB {
	t.Helper()

	testDatabaseOpenGate <- struct{}{}
	defer func() { <-testDatabaseOpenGate }()

	ctx, cancel := context.WithTimeout(context.Background(), testDatabaseOpenTimeout)
	defer cancel()
	db, err := Open(ctx, filepath.Join(t.TempDir(), filename))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return db
}

func createTestUser(t *testing.T, store *Store, telegramID int64) model.User {
	t.Helper()

	user, created, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{
		ID:        telegramID,
		FirstName: "Test",
		Username:  fmt.Sprintf("telegram%d", telegramID),
	}, false)
	if err != nil {
		t.Fatalf("UpsertTelegramUser(): %v", err)
	}
	if !created {
		t.Fatal("test user was not created")
	}
	return user
}

func saveTestSquad(t *testing.T, store *Store, uuid string, price int64, visible bool) model.SquadProduct {
	t.Helper()

	imported, err := store.ImportSquad(context.Background(), uuid, uuid)
	if err != nil {
		t.Fatalf("ImportSquad(%q): %v", uuid, err)
	}
	product, err := store.SaveSquadProduct(context.Background(), SquadProductInput{
		ID:              imported.ID,
		GroupID:         uuid,
		Name:            uuid,
		Description:     "test squad",
		PriceTXBMinor:   price,
		Visible:         visible,
		UpstreamPresent: true,
	})
	if err != nil {
		t.Fatalf("SaveSquadProduct(%q): %v", uuid, err)
	}
	return product
}

func saveTestCombo(t *testing.T, store *Store, name string, price int64, validityDays int, squadIDs ...string) model.Combo {
	t.Helper()

	combo, err := store.SaveCombo(context.Background(), ComboInput{
		Name:              name,
		Description:       "test combo",
		PriceTXBMinor:     price,
		ValidityDays:      validityDays,
		TrafficLimitBytes: 100 * 1024 * 1024,
		ResetStrategy:     "MONTH_ROLLING",
		Active:            true,
		SquadProductIDs:   squadIDs,
	})
	if err != nil {
		t.Fatalf("SaveCombo(%q): %v", name, err)
	}
	return combo
}

func createTestPaymentOrder(t *testing.T, store *Store, userID, provider string, txbMinor int64, now time.Time) model.PaymentOrder {
	t.Helper()

	currency := "CNY"
	switch provider {
	case "bepusdt":
		currency = "USD"
	case "stars":
		currency = "XTR"
	}
	order, err := store.CreatePaymentOrder(context.Background(), model.PaymentOrder{
		UserID:          userID,
		Provider:        provider,
		Status:          "pending",
		TXBMinor:        txbMinor,
		PayableAmount:   "10.00",
		PayableCurrency: currency,
		RateSnapshot:    "1",
		ExpiresAt:       now.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder(): %v", err)
	}
	return order
}

func countLedgerKind(entries []model.LedgerEntry, kind string) int {
	count := 0
	for _, entry := range entries {
		if entry.Kind == kind {
			count++
		}
	}
	return count
}

func assertRowCount(t *testing.T, store *Store, table string, want int) {
	t.Helper()

	var query string
	switch table {
	case "outbox_jobs":
		query = `SELECT COUNT(*) FROM outbox_jobs`
	case "purchases":
		query = `SELECT COUNT(*) FROM purchases`
	case "refunds":
		query = `SELECT COUNT(*) FROM refunds`
	case "webhook_events":
		query = `SELECT COUNT(*) FROM webhook_events`
	case "payment_callback_tombstones":
		query = `SELECT COUNT(*) FROM payment_callback_tombstones`
	default:
		t.Fatalf("assertRowCount called with unsupported table %q", table)
	}
	var got int
	if err := store.DB().QueryRowContext(context.Background(), query).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func equalStrings(left, right []string) bool {
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	if len(leftCopy) != len(rightCopy) {
		return false
	}
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}
