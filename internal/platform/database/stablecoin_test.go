package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func stablecoinOrder(t *testing.T, s *Store, user, rail string) usdt.Quote {
	t.Helper()
	spec, _ := usdt.Network(rail)
	o, err := s.CreatePaymentOrder(context.Background(), model.PaymentOrder{UserID: user, Provider: usdt.Provider, MethodID: usdt.Provider + ":" + rail, ProviderRail: rail, TXBMinor: 1000, PayableAmount: "1.000", PayableCurrency: spec.Currency(), RateSnapshot: "10", RateDirection: "txb_per_currency", ExpiresAt: time.Now().UTC().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.ReserveUSDTQuote(context.Background(), o.ID, spec, []string{usdtTestWallet}, o.PayableAmount)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestStablecoinEvidenceAndProgressAreTokenIsolated(t *testing.T) {
	s := newTestStore(t)
	user := createTestUser(t, s, 99031)
	ctx := context.Background()
	tether := stablecoinOrder(t, s, user.ID, "usdt.erc20")
	circle := stablecoinOrder(t, s, user.ID, "usdc.erc20")
	if tether.AtomicAmount != circle.AtomicAmount {
		t.Fatal("separate token fingerprints collided")
	}
	for i, q := range []usdt.Quote{tether, circle} {
		spec, _ := usdt.TokenNetwork(q.Network, q.Token)
		transfer := usdtTestTransfer(q, spec.Rail, time.Now().UTC())
		value := []string{"100", "200"}[i]
		if err := s.SaveUSDTBatch(ctx, spec.Rail, usdt.Batch{Transfers: []usdt.Transfer{transfer}, Cursor: usdt.Cursor{Value: value}}); err != nil {
			t.Fatal(err)
		}
		cursor, err := s.USDTCursor(ctx, spec.Rail)
		if err != nil || cursor.Value != value {
			t.Fatal("cursor contamination", cursor, err)
		}
	}
	ready, err := s.USDTReady(ctx, 10)
	if err != nil || len(ready) != 2 {
		t.Fatal(ready, err)
	}
	for _, transfer := range ready {
		wrong := tether.OrderID
		if transfer.OrderID == wrong {
			wrong = circle.OrderID
		}
		if _, _, err := s.SettleUSDTTransfer(ctx, transfer.ID, wrong, "", ""); !errors.Is(err, usdt.ErrConflict) {
			t.Fatal("cross-token settlement", err)
		}
		if _, changed, err := s.SettleUSDTTransfer(ctx, transfer.ID, transfer.OrderID, "", ""); err != nil || !changed {
			t.Fatal(changed, err)
		}
	}
}

func TestCourtesySettlementAndDismissalRaceRetainsOneFinancialOutcome(t *testing.T) {
	s := newTestStore(t)
	user := createTestUser(t, s, 99032)
	admin := createTestUser(t, s, 99033)
	ctx := context.Background()
	if _, err := s.db.Exec("UPDATE users SET role='admin' WHERE id=?", admin.ID); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		q := stablecoinOrder(t, s, user.ID, "usdc.erc20")
		if _, err := s.db.Exec("UPDATE payment_orders SET status='expired',cancelled_at=created_at WHERE id=?", q.OrderID); err != nil {
			t.Fatal(err)
		}
		txHash := time.Now().UTC().Format(time.RFC3339Nano)
		transfer := usdtTestTransfer(q, txHash, time.Now().UTC())
		if err := s.SaveUSDTBatch(ctx, "usdc.erc20", usdt.Batch{Transfers: []usdt.Transfer{transfer}}); err != nil {
			t.Fatal(err)
		}
		page, err := s.USDTMatchingEvidence(ctx, q.OrderID)
		if err != nil || len(page.Items) != 1 {
			t.Fatal(page, err)
		}
		id := page.Items[0].ID
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, action := range []func(){
			func() { _, _, _ = s.SettleUSDTTransfer(ctx, id, q.OrderID, admin.ID, "verified transfer") },
			func() { _, _ = s.CourtesyCreditPayment(ctx, admin.ID, q.OrderID, "provider failure", time.Now()) },
			func() { _, _ = s.DismissUSDTTransfer(ctx, id, admin.ID, "unrelated evidence") },
		} {
			wg.Go(func() { <-start; action() })
		}
		close(start)
		wg.Wait()
		var credits int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM ledger_entries WHERE reference_id=? AND kind IN ('payment_credit','payment_courtesy_credit')", q.OrderID).Scan(&credits); err != nil || credits != 1 {
			t.Fatal("race duplicated or lost credit", i, credits, err)
		}
		var status string
		if err := s.db.QueryRow("SELECT status FROM usdt_transfers WHERE id=?", id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "dismissed" {
			if _, _, err := s.SettleUSDTTransfer(ctx, id, q.OrderID, admin.ID, "retry"); !errors.Is(err, usdt.ErrConflict) {
				t.Fatal("dismissed transfer settled", err)
			}
			if changed, err := s.DismissUSDTTransfer(ctx, id, admin.ID, "retry dismissal"); err != nil || changed {
				t.Fatal("dismissal replay changed evidence", changed, err)
			}
		}
		if status == "credited" {
			if _, err := s.DismissUSDTTransfer(ctx, id, admin.ID, "attempt overwrite"); !errors.Is(err, usdt.ErrConflict) {
				t.Fatal("credited transfer dismissed", err)
			}
		}
		if err := s.SaveUSDTBatch(ctx, "usdc.erc20", usdt.Batch{Transfers: []usdt.Transfer{transfer}}); err != nil {
			t.Fatal("scanner replay lost evidence", err)
		}
	}
}
