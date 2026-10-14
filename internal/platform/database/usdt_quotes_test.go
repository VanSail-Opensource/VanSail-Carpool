package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func TestUSDTQuoteReservationIsConcurrentPermanentAndExact(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	user := createTestUser(t, s, 88001)
	spec, _ := usdt.Network("ethereum")
	ctx := context.Background()
	ids := []string{}
	for range 16 {
		ids = append(ids, createUSDTTestOrder(t, s, user.ID, "1.000").ID)
	}
	results := make(chan usdt.Quote, 16)
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, err := s.ReserveUSDTQuote(ctx, id, spec, []string{usdtTestWallet}, "1.000")
			results <- q
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for q := range results {
		if seen[q.Amount] {
			t.Fatalf("duplicate amount %s", q.Amount)
		}
		seen[q.Amount] = true
		if len(q.Amount) != 5 || q.AtomicAmount == "" {
			t.Fatalf("quote=%+v", q)
		}
		replay, err := s.ReserveUSDTQuote(ctx, q.OrderID, spec, []string{"0x2222222222222222222222222222222222222222"}, "2.000")
		if err != nil || replay != q {
			t.Fatalf("immutable replay=%+v %v", replay, err)
		}
	}
	order := createUSDTTestOrder(t, s, user.ID, "1.000")
	q := reserveUSDTTestQuote(t, s, order)
	if _, err := s.db.Exec("DELETE FROM payment_orders WHERE id=?", order.ID); err != nil {
		t.Fatal(err)
	}
	retained, err := s.USDTQuote(ctx, order.ID)
	if err != nil || retained != q {
		t.Fatalf("lost permanent reservation: %+v %v", retained, err)
	}
	if _, err := s.USDTQuote(ctx, "missing"); !errors.Is(err, usdt.ErrQuoteNotFound) {
		t.Fatal(err)
	}
}

func TestUSDTQuoteCapacityAndAtomicBSCAmount(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	user := createTestUser(t, s, 88002)
	ctx := context.Background()
	for range 100 {
		order := createUSDTTestOrder(t, s, user.ID, "1.000")
		reserveUSDTTestQuote(t, s, order)
	}
	order := createUSDTTestOrder(t, s, user.ID, "1.000")
	spec, _ := usdt.Network("ethereum")
	if _, err := s.ReserveUSDTQuote(ctx, order.ID, spec, []string{usdtTestWallet}, "1.000"); !errors.Is(err, usdt.ErrCapacity) {
		t.Fatal(err)
	}
	q, err := s.ReserveUSDTQuote(ctx, order.ID, spec, []string{"0x2222222222222222222222222222222222222222"}, "1.000")
	if err != nil || q.Amount != "1.000" {
		t.Fatalf("new wallet=%+v %v", q, err)
	}
	bsc, _ := usdt.Network("bsc")
	next := createUSDTTestOrder(t, s, user.ID, "1.000")
	if _, err := s.db.Exec("UPDATE payment_orders SET provider_rail=?,method_id=? WHERE id=?", bsc.Rail, usdt.Provider+":"+bsc.Rail, next.ID); err != nil {
		t.Fatal(err)
	}
	quote, err := s.ReserveUSDTQuote(ctx, next.ID, bsc, []string{usdtTestWallet}, "1.000")
	if err != nil || quote.AtomicAmount != "1000000000000000000" {
		t.Fatalf("BSC amount=%+v %v", quote, err)
	}
	if err := s.ExpireStalePaymentOrders(ctx, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	addresses, err := s.USDTWatchAddresses(ctx, "ethereum")
	if err != nil || len(addresses) != 2 {
		t.Fatalf("watch=%v %v", addresses, err)
	}
}
