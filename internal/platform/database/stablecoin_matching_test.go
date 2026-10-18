package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func TestStablecoinMatchingKeepsExactPriorityToleranceAndAmbiguity(t *testing.T) {
	for _, rail := range []string{"usdt.erc20", "usdc.erc20"} {
		t.Run(rail, func(t *testing.T) {
			s := newTestStore(t)
			user := createTestUser(t, s, 99035)
			admin := createTestUser(t, s, 99036)
			ctx := context.Background()
			if _, err := s.db.Exec("UPDATE users SET role='admin' WHERE id=?", admin.ID); err != nil {
				t.Fatal(err)
			}
			q := stablecoinOrder(t, s, user.ID, rail)
			other := stablecoinOrder(t, s, user.ID, rail)
			save := func(amount, hash string) usdt.Transfer {
				t.Helper()
				tr := usdtTestTransfer(q, hash, time.Now().UTC())
				tr.AtomicAmount = amount
				tr.Amount, _ = usdt.TransferAmount(amount, 6)
				if err := s.SaveUSDTBatch(ctx, rail, usdt.Batch{Transfers: []usdt.Transfer{tr}}); err != nil {
					t.Fatal(err)
				}
				stored, err := scanUSDTTransfer(s.db.QueryRow(usdtTransferSelect+" WHERE tx_hash=?", hash))
				if err != nil {
					t.Fatal(err)
				}
				return stored
			}
			exact := save("1000000", "exact")
			if exact.OrderID != q.OrderID || exact.Status != "ready" {
				t.Fatal("exact lost priority to tolerated quote", exact)
			}
			boundary := save("999000", "boundary")
			if boundary.OrderID != q.OrderID || boundary.Status != "ready" {
				t.Fatal("0.001 shortfall rejected", boundary)
			}
			tooShort := save("998999", "too-short")
			if tooShort.OrderID != "" || tooShort.Status != "review" {
				t.Fatal("excess shortfall matched", tooShort)
			}
			overpaid := save("1001001", "overpaid")
			if overpaid.OrderID != "" || overpaid.Status != "review" {
				t.Fatal("overpayment matched", overpaid)
			}
			// Imported evidence can contain finer precision than new three-place quotes.
			if _, err := s.db.Exec("UPDATE usdt_quotes SET atomic_amount='1000500',amount='1.0005' WHERE order_id=?", other.OrderID); err != nil {
				t.Fatal(err)
			}
			ambiguous := save("999500", "ambiguous")
			if ambiguous.OrderID != "" || ambiguous.Status != "review" {
				t.Fatal("ambiguous evidence matched", ambiguous)
			}
			if _, _, err := s.SettleUSDTTransfer(ctx, ambiguous.ID, q.OrderID, admin.ID, "manual attempt"); !errors.Is(err, usdt.ErrConflict) {
				t.Fatal("manual settlement bypassed ambiguity", err)
			}
		})
	}
}
