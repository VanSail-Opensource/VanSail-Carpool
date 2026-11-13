package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestPaymentExpiryUsesExactStoredDeadline(t *testing.T) {
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, path := range []string{"scanner", "checkout transaction"} {
		for _, tc := range []struct {
			name         string
			expires, now time.Time
			expired      bool
		}{
			{"live fractional expiry", base.Add(500 * time.Millisecond), base, false},
			{"past whole expiry", base, base.Add(500 * time.Millisecond), true},
			{"inclusive fractional expiry", base.Add(500 * time.Millisecond), base.Add(500 * time.Millisecond), true},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				ctx, store := context.Background(), newTestStore(t)
				user := createTestUser(t, store, 950010)
				order := createTestPaymentOrder(t, store, user.ID, "ezpay", 100, time.Now().UTC())
				if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET expires_at=$1,payment_url='https://pay.example/live' WHERE id=$2`, stamp(tc.expires), order.ID); err != nil {
					t.Fatal(err)
				}
				var err error
				if path == "scanner" {
					err = store.ExpireStalePaymentOrders(ctx, tc.now)
				} else {
					_, err = hourlyCreditTransaction(ctx, store, func(tx *sql.Tx) (struct{}, error) { return struct{}{}, expireStalePaymentsTx(ctx, tx, tc.now) })
				}
				if err != nil {
					t.Fatal(err)
				}
				current, err := store.PaymentOrderByID(ctx, order.ID)
				if err != nil {
					t.Fatal(err)
				}
				if (current.Status == "expired") != tc.expired || (current.PaymentURL == nil) != tc.expired {
					t.Fatalf("expiry = %+v, want expired %v", current, tc.expired)
				}
			})
		}
	}
}

func TestLatestValidPaymentParsesExpiryAndCreationOrder(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 950011)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	first := createTestPaymentOrder(t, store, user.ID, "ezpay", 100, time.Now().UTC())
	second := createTestPaymentOrder(t, store, user.ID, "ezpay", 100, time.Now().UTC())
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET created_at=$1,expires_at=$2 WHERE id=$3`, stamp(base), stamp(base.Add(time.Second)), first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET created_at=$1,expires_at=$2 WHERE id=$3`, stamp(base.Add(100*time.Millisecond)), stamp(base.Add(500*time.Millisecond)), second.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		now  time.Time
		want string
	}{
		{base, second.ID},
		{base.Add(500 * time.Millisecond), first.ID},
		{base.Add(time.Second), ""},
	} {
		got, err := store.LatestValidUnpaidPaymentOrder(ctx, user.ID, tc.now)
		if tc.want == "" {
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("expired checkout = %+v, %v", got, err)
			}
		} else if err != nil || got.ID != tc.want {
			t.Fatalf("checkout = %+v, %v; want %s", got, err, tc.want)
		}
	}
}
