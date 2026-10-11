package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

func TestIPLookupCacheCoalescingQuotaAndPaidRefresh(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, purchase, now := ipLookupFixture(t, 2)
	q, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Submit(ctx, user, "first", q)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Submit(ctx, user, "first", q)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err := service.Submit(ctx, user, "stale", q); iplookup.ErrorCode(err) != "IP_LOOKUP_BUSY" {
		t.Fatalf("stale quote accepted: %v", err)
	}
	r1, _, _ := store.IPLookupRun(ctx, first.ID)
	if err := store.MarkPurchaseSyncResult(ctx, purchase, true, now); err != nil {
		t.Fatal(err)
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 1 {
		t.Fatalf("sync refilled quota: %+v %v", state, err)
	}
	finishIP(t, store, first.ID, false)
	third := submitIP(t, service, user, "cached-paid", false)
	check, err := service.Check(ctx, user, third.ID)
	if err != nil || !check.Cached || check.Charge.Minor != "0" || check.Report.ID != r1.ID {
		t.Fatalf("cached check=%+v %v", check, err)
	}
	refreshQ, err := service.Quote(ctx, user, "8.8.8.8", true)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := service.Submit(ctx, user, "refresh", refreshQ)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(ctx, user, "refresh-joined", refreshQ); iplookup.ErrorCode(err) != "IP_LOOKUP_BUSY" {
		t.Fatalf("duplicate refresh charged: %v", err)
	}
	newReport, _, _ := store.IPLookupRun(ctx, refresh.ID)
	if newReport.ID == r1.ID {
		t.Fatal("refresh was cached or duplicated")
	}
	finishIP(t, store, refresh.ID, false)
	late, err := service.Submit(ctx, user, "refresh-late", refreshQ)
	if err != nil {
		t.Fatal(err)
	}
	lateCheck, err := service.Check(ctx, user, late.ID)
	if err != nil || lateCheck.Report.ID != newReport.ID || lateCheck.Charge.Minor != "0" {
		t.Fatalf("late refresh=%+v %v", lateCheck, err)
	}
	if _, err := service.Check(ctx, "another-owner", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner report access allowed")
	}
}

func TestIPLookupUnattemptedFailureRestoresCostExactlyOnce(t *testing.T) {
	t.Parallel()
	for _, quota := range []int{0, 1} {
		t.Run(model.TXBMoney(int64(quota)).Minor, func(t *testing.T) {
			ctx := context.Background()
			store, service, user, _, now := ipLookupFixture(t, quota)
			before, err := store.Balance(ctx, user)
			if err != nil {
				t.Fatal(err)
			}
			op := submitIP(t, service, user, "failed", false)
			r := finishIP(t, store, op.ID, true)
			if err := store.FinishIPLookupRun(ctx, r, now); err != nil {
				t.Fatal(err)
			}
			after, err := store.Balance(ctx, user)
			if err != nil || before.Minor != after.Minor {
				t.Fatalf("balance %v -> %v err=%v", before, after, err)
			}
			state, err := service.State(ctx, user)
			if err != nil || state.Allowance.Remaining != quota {
				t.Fatalf("quota not restored: %+v %v", state, err)
			}
			check, err := service.Check(ctx, user, op.ID)
			if err != nil || !check.Refunded || check.Charge.Minor != "0" {
				t.Fatalf("check=%+v %v", check, err)
			}
			q, err := service.Quote(ctx, user, "8.8.8.8", false)
			if err != nil || q.CacheReportID != "" {
				t.Fatalf("failed report cached: %+v %v", q, err)
			}
		})
	}
}

func TestIPLookupInfrastructureFailureRetainsImmutableCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 0)
	original := finishIP(t, store, submitIP(t, service, user, "original", false).ID, false)
	refresh := submitIP(t, service, user, "failed-refresh", true)
	finishIP(t, store, refresh.ID, true)
	c := iplookup.DefaultConfig()
	c.Enabled = true
	c.LookupFeeTXB = "2.50"
	c.RefreshFeeTXB = "3.50"
	c.Providers[0].Enabled = true
	if err := store.SaveIPLookupSettings(ctx, user, c, map[string]string{"abuseipdb": "different-secret"}, nil); err != nil {
		t.Fatal(err)
	}
	op := submitIP(t, service, user, "old-cache", false)
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.Report.ID != original.ID || check.Report.PolicyVersion != original.PolicyVersion {
		t.Fatalf("cached snapshot changed: %+v %v", check, err)
	}
}

func TestIPLookupNewActivationAndExpiredAllowance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, purchase, now := ipLookupFixture(t, 2)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_allowances SET used=2 WHERE purchase_id=?`, purchase); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='activating',valid_until=? WHERE id=?`, stamp(now.Add(-time.Minute)), purchase); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase, true, now); err != nil {
		t.Fatal(err)
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance != nil {
		t.Fatalf("expired quota eligible: %+v %v", state, err)
	}
	var combo string
	if err := store.DB().QueryRowContext(ctx, `SELECT combo_id FROM purchases WHERE id=?`, purchase).Scan(&combo); err != nil {
		t.Fatal(err)
	}
	next, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user, ComboID: combo, IdempotencyKey: "next"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, next.ID, true, now); err != nil {
		t.Fatal(err)
	}
	state, err = service.State(ctx, user)
	if err != nil || state.Allowance.PurchaseID != next.ID || state.Allowance.Remaining != 2 {
		t.Fatalf("new term quota=%+v %v", state, err)
	}
}
