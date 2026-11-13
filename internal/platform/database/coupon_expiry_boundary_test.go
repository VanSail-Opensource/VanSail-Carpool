package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestCouponWalletExpiresAtExactInstant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 32_205)
	now := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	grants := make([]coupons.Grant, 0, 2)
	for _, input := range []struct {
		code    string
		expires time.Time
	}{
		{"WHOLE_EXPIRY", now},
		{"NANO_EXPIRY", now.Add(900 * time.Nanosecond)},
	} {
		coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: input.code, Name: input.code,
			Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed,
			ValueMinorOrBPS: 20, ExpiresAt: &input.expires, Active: true}, now.Add(-time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		grant, err := store.GrantCoupon(ctx, user.ID, coupon.ID, "test", input.code, now.Add(-time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		grants = append(grants, grant)
	}
	wallet, err := store.ListCouponGrants(ctx, user.ID, now)
	if err != nil || len(wallet) != 1 || wallet[0].ID != grants[1].ID {
		t.Fatalf("wallet at whole second = %+v, %v", wallet, err)
	}
	for index, want := range []string{"expired", "active"} {
		var status string
		if err := store.DB().QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=$1`, grants[index].ID).Scan(&status); err != nil || status != want {
			t.Fatalf("grant %d status = %q, %v; want %q", index, status, err, want)
		}
	}
	quote, err := store.QuotePurchaseCoupon(ctx, coupons.PurchaseContext{UserID: user.ID,
		GrantID: grants[1].ID, ComboID: "test", GrossPriceMinor: 100}, now)
	if err != nil || quote.DiscountMinor != 20 {
		t.Fatalf("live coupon quote = %+v, %v", quote, err)
	}
	wallet, err = store.ListCouponGrants(ctx, user.ID, now.Add(900*time.Nanosecond))
	if err != nil || len(wallet) != 0 {
		t.Fatalf("wallet at exact expiry = %+v, %v", wallet, err)
	}
	var status string
	if err := store.DB().QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=$1`, grants[1].ID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("expired grant status = %q, %v", status, err)
	}
}

func TestCouponWalletWaitsForConcurrentExpiryExtension(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store := newTestStore(t)
	user := createTestUser(t, store, 32_206)
	now := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	expires := now.Add(-time.Second)
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "EXTEND_EXPIRY", Name: "Extend expiry",
		Kind: coupons.KindPurchaseRecurring, DiscountMode: coupons.DiscountFixed,
		ValueMinorOrBPS: 20, ExpiresAt: &expires, Active: true}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := store.GrantCoupon(ctx, user.ID, coupon.ID, "test", "extension", now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE coupon_definitions SET expires_at=$1 WHERE id=$2`, stamp(now.Add(time.Hour)), coupon.ID); err != nil {
		t.Fatal(err)
	}
	type walletResult struct {
		grants []coupons.Grant
		err    error
	}
	done := make(chan walletResult, 1)
	go func() {
		grants, err := store.ListCouponGrants(ctx, user.ID, now)
		done <- walletResult{grants: grants, err: err}
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'FOR UPDATE OF g,c')>0)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case result := <-done:
			t.Fatalf("wallet did not wait for coupon extension: %+v, %v", result.grants, result.err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.err != nil || len(result.grants) != 1 || result.grants[0].ID != grant.ID {
		t.Fatalf("extended coupon wallet = %+v, %v", result.grants, result.err)
	}
	var status string
	if err := store.DB().QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=$1`, grant.ID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("extended coupon status = %q, %v", status, err)
	}
}
