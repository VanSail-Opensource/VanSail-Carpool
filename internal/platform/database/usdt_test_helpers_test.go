package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

const usdtTestWallet = "0x1111111111111111111111111111111111111111"

func createUSDTTestOrder(t *testing.T, s *Store, userID, base string) model.PaymentOrder {
	t.Helper()
	order, err := s.CreatePaymentOrder(context.Background(), model.PaymentOrder{UserID: userID, Provider: usdt.Provider, MethodID: "builtin_usdt:usdt.erc20", ProviderRail: "usdt.erc20", TXBMinor: 1000, PayableAmount: base, PayableCurrency: "USDT", RateSnapshot: "10", RateDirection: "txb_per_currency", ExpiresAt: time.Now().UTC().Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	return order
}
func reserveUSDTTestQuote(t *testing.T, s *Store, order model.PaymentOrder) usdt.Quote {
	t.Helper()
	spec, _ := usdt.Network("ethereum")
	q, err := s.ReserveUSDTQuote(context.Background(), order.ID, spec, []string{usdtTestWallet}, order.PayableAmount)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func usdtTestTransfer(q usdt.Quote, hash string, occurred time.Time) usdt.Transfer {
	amount, _ := usdt.TransferAmount(q.AtomicAmount, 6)
	return usdt.Transfer{Network: q.Network, Token: q.Token, Recipient: q.Recipient, Amount: amount, AtomicAmount: q.AtomicAmount, TxHash: hash, EventID: "0", OccurredAt: occurred, Verified: true, Finalized: true}
}
func saveUSDTTestTransfer(t *testing.T, s *Store, transfer usdt.Transfer) string {
	t.Helper()
	if err := s.SaveUSDTBatch(context.Background(), transfer.Network, usdt.Batch{Transfers: []usdt.Transfer{transfer}, Cursor: usdt.Cursor{Value: "100", Hash: "block"}, CaughtUp: true}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := s.db.QueryRow("SELECT id FROM usdt_transfers WHERE network=? AND tx_hash=? AND event_id=?", transfer.Network, transfer.TxHash, transfer.EventID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
