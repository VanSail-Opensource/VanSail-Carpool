package database

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func runTogether(actions ...func() error) []error {
	start := make(chan struct{})
	results := make(chan error, len(actions))
	for _, action := range actions {
		go func() { <-start; results <- action() }()
	}
	close(start)
	errors := make([]error, 0, len(actions))
	for range actions {
		errors = append(errors, <-results)
	}
	return errors
}

func TestPostgresPaymentReplayRacesDebit(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createTestUser(t, s, 917101)
	if _, err := s.AdjustBalance(ctx, user.ID, 100, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	order := createTestPaymentOrder(t, s, user.ID, "ezpay", 2500, now)
	var applied atomic.Int64
	settle := func() error {
		_, credited, err := s.SettlePayment(ctx, "ezpay", "same-event", "same-payload", order.ID, "same-trade", "same-charge", now)
		if credited {
			applied.Add(1)
		}
		return err
	}
	debit := func() error { _, err := s.DeductBalance(ctx, user.ID, 80, "racing-debit", "debit", now); return err }
	for _, err := range runTogether(settle, settle, debit) {
		if err != nil {
			t.Fatal(err)
		}
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "2520" || applied.Load() != 1 {
		t.Fatalf("balance=%+v applied=%d error=%v", balance, applied.Load(), err)
	}
	entries, err := s.ListLedger(ctx, user.ID, 10)
	if err != nil || len(entries) != 3 || countLedgerKind(entries, "payment_credit") != 1 {
		t.Fatal("payment/debit ledger differs", err)
	}
}

func TestPostgresLimitedStockConcurrentPurchases(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	limit := 1
	combo := saveTestCombo(t, s, "concurrent-stock", 100, 30)
	addon, err := s.SaveSquadProduct(ctx, SquadProductInput{GroupID: "concurrent-addon", Name: "Concurrent", Visible: true, UpstreamPresent: true, StockLimit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	var actions []func() error
	for _, id := range []int64{917102, 917103} {
		user := createTestUser(t, s, id)
		if _, err = s.AdjustBalance(ctx, user.ID, 1000, "stock-seed:"+user.ID, "seed", now); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, func() error {
			_, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "limited-stock"}, now)
			return err
		})
	}
	success, unavailable := 0, 0
	for _, err := range runTogether(actions...) {
		if err == nil {
			success++
		} else if errors.Is(err, ErrStockUnavailable) {
			unavailable++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || unavailable != 1 {
		t.Fatalf("stock results %d/%d", success, unavailable)
	}
	assertRowCount(t, s, "purchases", 1)
}

func TestPostgresRenewalRacesCancellation(t *testing.T) {
	if os.Getenv("POSTGRES_CONTRACT_URL") == "" {
		t.Skip("hosted PostgreSQL required")
	}
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createTestUser(t, s, 917104)
	combo := saveTestCombo(t, s, "renewal-race", 100, 30)
	if _, err := s.AdjustBalance(ctx, user.ID, 500, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatal(err)
	}

	for _, err = range runTogether(func() error { _, e := s.CommitAutoRenewal(ctx, source.ID, source.ValidUntil); return e }, func() error { return s.SetAutoRenewal(ctx, user.ID, source.ID, false, source.ValidUntil) }) {
		if err != nil && !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	var successors, ledger int64
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE auto_renew_source_purchase_id=?`, source.ID).Scan(&successors); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='automatic_renewal'`, user.ID).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || successors > 1 || ledger != successors || balance.MinorInt64() != 400-100*successors {
		t.Fatalf("renewal consistency: successors=%d ledger=%d balance=%+v error=%v", successors, ledger, balance, err)
	}
}
