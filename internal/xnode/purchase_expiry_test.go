package xnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurchaseExpiryRetiresStaleSnapshotWithoutClosingExtendedAllowance(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, at.Format(time.RFC3339Nano), period.PurchaseID); err != nil {
		t.Fatal(err)
	}
	captured, err := f.store.PurchaseByID(ctx, period.PurchaseID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=73 WHERE id=$1`, period.ID); err != nil {
		t.Fatal(err)
	}
	// Keep updated_at unchanged to prove the expiry check itself is required.
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, at.Add(time.Hour).Format(time.RFC3339Nano), captured.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.service.CloseExpiredPurchasePeriods(ctx, f.user, captured, at); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.XNodePeriod(ctx, period.ID)
	if err != nil || current.Status != "open" || current.ClosureGeneration != 0 || current.UsedBytes != 73 {
		t.Fatal("stale expiry changed extended allowance", current, err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1,updated_at=$2 WHERE id=$3`, at.Format(time.RFC3339Nano), at.Add(time.Nanosecond).Format(time.RFC3339Nano), captured.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.service.CloseExpiredPurchasePeriods(ctx, f.user, captured, at); err != nil {
		t.Fatal(err)
	}
	current, err = f.store.XNodePeriod(ctx, period.ID)
	if err != nil || current.Status != "open" {
		t.Fatal("old purchase revision admitted expiry", current, err)
	}
	fresh, err := f.store.PurchaseByID(ctx, captured.ID)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = f.service.CloseExpiredPurchasePeriods(ctx, f.user, fresh, at); err != nil {
			t.Fatal(err)
		}
	}
	current, err = f.store.XNodePeriod(ctx, period.ID)
	if err != nil || current.Status != "closed" || current.ClosureGeneration != 1 || current.UsedBytes != 73 {
		t.Fatal("current expiry lost accounting or replayed closure", current, err)
	}
}

func TestPurchaseExpiryRechecksExtensionAfterWaitingForPurchaseLock(t *testing.T) {
	f := controlFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, at.Format(time.RFC3339Nano), period.PurchaseID); err != nil {
		t.Fatal(err)
	}
	captured, err := f.store.PurchaseByID(ctx, period.PurchaseID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, at.Add(time.Hour).Format(time.RFC3339Nano), captured.ID); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		period   xnode.Period
		admitted bool
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		p, admitted, err := f.store.BeginXNodePurchaseExpiryClosure(ctx, f.user, captured, period, at)
		done <- outcome{p, admitted, err}
	}()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var blocked bool
		err = f.store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND strpos(query,'SELECT status,valid_until,updated_at FROM purchases')>0)`).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case early := <-done:
			t.Fatal("expiry bypassed the purchase lock", early)
		case <-ctx.Done():
			t.Fatal("expiry did not wait on the purchase lock", ctx.Err())
		case <-tick.C:
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.err != nil || result.admitted {
		t.Fatal("expiry admitted an extension committed while waiting", result)
	}
	current, err := f.store.XNodePeriod(ctx, period.ID)
	if err != nil || current.Status != "open" || current.ClosureGeneration != 0 {
		t.Fatal("extended period was closed", current, err)
	}
}
