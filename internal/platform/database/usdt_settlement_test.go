package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func TestUSDTSettlementRecoversExpiredOrdersAndClaimsTransfersOnce(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	user := createTestUser(t, s, 88003)
	ctx := context.Background()
	order := createUSDTTestOrder(t, s, user.ID, "1.000")
	q := reserveUSDTTestQuote(t, s, order)
	created := time.Now().UTC().Add(-time.Hour)
	expires := created.Add(30 * time.Minute)
	if _, err := s.db.Exec("UPDATE payment_orders SET created_at=?,expires_at=?,status='expired' WHERE id=?", stamp(created), stamp(expires), order.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE usdt_quotes SET expires_at=? WHERE order_id=?", stamp(expires), order.ID); err != nil {
		t.Fatal(err)
	}
	transfer := usdtTestTransfer(q, "in-window", created.Add(time.Minute))
	id := saveUSDTTestTransfer(t, s, transfer)
	if err := s.SaveUSDTBatch(ctx, q.Network, usdt.Batch{Transfers: []usdt.Transfer{transfer}, Cursor: usdt.Cursor{Value: "101"}}); err != nil {
		t.Fatal(err)
	}
	ready, err := s.USDTReady(ctx, 100)
	if err != nil || len(ready) != 1 {
		t.Fatalf("ready=%v %v", ready, err)
	}
	paid, changed, err := s.SettleUSDTTransfer(ctx, id, order.ID, "", "")
	if err != nil || !changed || paid.Status != "paid" {
		t.Fatalf("settle=%+v %t %v", paid, changed, err)
	}
	if _, changed, err := s.SettleUSDTTransfer(ctx, id, order.ID, "", ""); err != nil || changed {
		t.Fatalf("replay=%t %v", changed, err)
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "1000" {
		t.Fatalf("balance=%+v %v", balance, err)
	}
	otherID := saveUSDTTestTransfer(t, s, usdtTestTransfer(q, "second-payment", created.Add(2*time.Minute)))
	if _, changed, err := s.SettleUSDTTransfer(ctx, otherID, order.ID, "", ""); err == nil && changed {
		t.Fatal("double deposit credited an already-paid order")
	}
	transfer.AtomicAmount = "1000001"
	transfer.Amount = "1.000001"
	if err := s.SaveUSDTBatch(ctx, q.Network, usdt.Batch{Transfers: []usdt.Transfer{transfer}, Cursor: usdt.Cursor{Value: "999"}}); !errors.Is(err, usdt.ErrConflict) {
		t.Fatalf("modified proof accepted: %v", err)
	}
	cursor, err := s.USDTCursor(ctx, q.Network)
	if err != nil || cursor.Value == "999" {
		t.Fatal("failed batch advanced cursor")
	}
}

func TestUSDTReviewEnforcesRoleAmountAndAudit(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	user := createTestUser(t, s, 88004)
	admin := createTestUser(t, s, 88005)
	ctx := context.Background()
	if _, err := s.db.Exec("UPDATE users SET role='admin' WHERE id=?", admin.ID); err != nil {
		t.Fatal(err)
	}
	order := createUSDTTestOrder(t, s, user.ID, "1.000")
	q := reserveUSDTTestQuote(t, s, order)
	if _, err := s.db.Exec("UPDATE payment_orders SET cancelled_at=? WHERE id=?", stamp(order.CreatedAt), order.ID); err != nil {
		t.Fatal(err)
	}
	id := saveUSDTTestTransfer(t, s, usdtTestTransfer(q, "late", order.CreatedAt.Add(time.Second)))
	if _, _, err := s.SettleUSDTTransfer(ctx, id, order.ID, "", ""); !errors.Is(err, usdt.ErrConflict) {
		t.Fatalf("late auto-credit=%v", err)
	}
	if _, _, err := s.SettleUSDTTransfer(ctx, id, order.ID, user.ID, "accept late transfer"); !errors.Is(err, usdt.ErrConflict) {
		t.Fatalf("member credit=%v", err)
	}
	if _, _, err := s.SettleUSDTTransfer(ctx, id, order.ID, admin.ID, " "); !errors.Is(err, usdt.ErrConflict) {
		t.Fatal("blank reason accepted")
	}
	page, err := s.USDTReview(ctx, "", 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Reason != "USDT_LATE_PAYMENT" {
		t.Fatalf("review=%+v %v", page, err)
	}
	if _, changed, err := s.SettleUSDTTransfer(ctx, id, order.ID, admin.ID, "verified late payment"); err != nil || !changed {
		t.Fatalf("review credit=%t %v", changed, err)
	}
	if _, changed, err := s.SettleUSDTTransfer(ctx, id, order.ID, admin.ID, "retry"); err != nil || changed {
		t.Fatalf("review replay=%t %v", changed, err)
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='usdt.transfer.credit'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("audits=%d %v", count, err)
	}
	mismatch := usdtTestTransfer(q, "underpaid", order.CreatedAt.Add(time.Second))
	mismatch.AtomicAmount = "999000"
	mismatch.Amount = "0.999000"
	wrongID := saveUSDTTestTransfer(t, s, mismatch)
	if _, _, err := s.SettleUSDTTransfer(ctx, wrongID, order.ID, admin.ID, "forgive difference"); !errors.Is(err, usdt.ErrConflict) {
		t.Fatal("amount mismatch credited")
	}
}
