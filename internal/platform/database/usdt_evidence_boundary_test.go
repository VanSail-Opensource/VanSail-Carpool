package database

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

func TestUSDTMatchingEvidenceUsesExactCreationBoundary(t *testing.T) {
	t.Parallel()
	base := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for _, test := range []struct {
		name              string
		created, occurred time.Duration
		want              bool
	}{
		{name: "whole-second-inclusive", want: true},
		{name: "fraction-after-whole-second", occurred: 500 * time.Millisecond, want: true},
		{name: "whole-second-before-fraction", created: 500 * time.Millisecond},
		{name: "nanosecond-before", created: 500 * time.Millisecond, occurred: 500*time.Millisecond - time.Nanosecond},
		{name: "fraction-inclusive", created: 500 * time.Millisecond, occurred: 500 * time.Millisecond, want: true},
		{name: "nanosecond-after", created: 500 * time.Millisecond, occurred: 500*time.Millisecond + time.Nanosecond, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newTestStore(t)
			owner := createTestUser(t, s, 88101)
			order := createUSDTTestOrder(t, s, owner.ID, "1.000")
			quote := reserveUSDTTestQuote(t, s, order)
			if _, err := s.db.ExecContext(context.Background(), `UPDATE payment_orders SET created_at=$1 WHERE id=$2`, stamp(base.Add(test.created)), order.ID); err != nil {
				t.Fatal(err)
			}
			id := saveUSDTTestTransfer(t, s, usdtTestTransfer(quote, test.name, base.Add(test.occurred)))
			page, err := s.USDTMatchingEvidence(context.Background(), order.ID)
			if err != nil || (len(page.Items) == 1) != test.want || len(page.Items) > 1 {
				t.Fatalf("matching evidence = %+v, %v; want match %t", page, err, test.want)
			}
			if test.want && page.Items[0].ID != id {
				t.Fatal("matched a different transfer", page.Items[0].ID)
			}
		})
	}
}

func TestUSDTMatchingEvidenceFiltersCreationBeforePageLimit(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 88102)
	order := createUSDTTestOrder(t, store, owner.ID, "1.000")
	quote := reserveUSDTTestQuote(t, store, order)
	base := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET created_at=$1 WHERE id=$2`, stamp(base.Add(250*time.Millisecond)), order.ID); err != nil {
		t.Fatal(err)
	}
	transfers := make([]usdt.Transfer, 0, 101)
	for index := range 100 {
		transfers = append(transfers, usdtTestTransfer(quote, "before-order-"+strconv.Itoa(index), base))
	}
	transfers = append(transfers, usdtTestTransfer(quote, "valid-after-order", base.Add(500*time.Millisecond)))
	if err := store.SaveUSDTBatch(ctx, quote.Network, usdt.Batch{Transfers: transfers, Cursor: usdt.Cursor{Value: "200", Hash: "block"}, CaughtUp: true}); err != nil {
		t.Fatal(err)
	}
	page, err := store.USDTMatchingEvidence(ctx, order.ID)
	if err != nil || len(page.Items) != 1 || page.Items[0].TxHash != "valid-after-order" {
		t.Fatal("pre-creation records consumed the review page", page, err)
	}
}
