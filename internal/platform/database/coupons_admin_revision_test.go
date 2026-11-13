package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestAdminCouponStaleMergeCannotUndoDeactivation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89122)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	initial, err := store.SaveAdminCoupon(ctx, actor.ID, coupons.CouponInput{Code: "DEACTIVATE", Name: "Before", Kind: coupons.KindBalanceAdd, ValueMinorOrBPS: 100, Active: true}, time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.RedeemCoupon(ctx, actor.ID, initial.Code, "original-redemption", now)
	if err != nil {
		t.Fatal(err)
	}
	disabled := initial.CouponInput
	disabled.Active = false
	current, err := store.SaveAdminCoupon(ctx, actor.ID, disabled, initial.UpdatedAt, now)
	if err != nil || !current.UpdatedAt.After(initial.UpdatedAt) {
		t.Fatal("same-clock mutation did not advance revision", current, err)
	}
	stale := initial.CouponInput
	stale.Name = "Stale edit"
	if _, err := store.SaveAdminCoupon(ctx, actor.ID, stale, initial.UpdatedAt, now.Add(time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale partial merge overwrote deactivation", err)
	}
	for _, revision := range []time.Time{{}, initial.UpdatedAt} {
		if _, err := store.SaveAdminCoupon(ctx, actor.ID, stale, revision, now); !errors.Is(err, ErrConflict) {
			t.Fatal("unbound/stale revision accepted", err)
		}
	}
	edit := current.CouponInput
	edit.Name = "Current edit"
	latest, err := store.SaveAdminCoupon(ctx, actor.ID, edit, current.UpdatedAt, now.Add(-time.Hour))
	if err != nil || latest.Active || !latest.UpdatedAt.After(current.UpdatedAt) {
		t.Fatal("current merge lost inactive state or revision on clock rollback", latest, err)
	}
	replayed, err := store.RedeemCoupon(ctx, actor.ID, initial.Code, "original-redemption", now.Add(time.Hour))
	if err != nil || !replayed.Replayed || replayed.ID != receipt.ID || replayed.BalanceAfterMinor != receipt.BalanceAfterMinor {
		t.Fatal("definition edits corrupted settled redemption evidence", replayed, err)
	}
	var redemptions, credits int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM coupon_redemptions),(SELECT COUNT(*) FROM ledger_entries WHERE kind='coupon_balance_add')`).Scan(&redemptions, &credits); err != nil || redemptions != 1 || credits != 1 {
		t.Fatal("definition retry duplicated financial effects", redemptions, credits, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='coupon.save' AND target_type='coupon' AND target_id=$2`, actor.ID, initial.ID).Scan(&audits); err != nil || audits != 3 {
		t.Fatal("stale writes added audits or committed writes lacked them", audits, err)
	}
}

func TestConcurrentAdminCouponEditsRequireTheirOriginalRevision(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89123)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	initial, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "COMPARE_SWAP", Name: "Before", Kind: coupons.KindBalanceAdd, ValueMinorOrBPS: 100, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	start, outcomes := make(chan struct{}), make(chan error, 2)
	for _, name := range []string{"First edit", "Second edit"} {
		go func() {
			<-start
			input := initial.CouponInput
			input.Name = name
			_, err := store.SaveAdminCoupon(ctx, actor.ID, input, initial.UpdatedAt, now)
			outcomes <- err
		}()
	}
	close(start)
	succeeded, conflicted := 0, 0
	for range 2 {
		err := <-outcomes
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrConflict) {
			conflicted++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatal("concurrent edits did not preserve original revision precondition", succeeded, conflicted)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='coupon.save' AND target_type='coupon' AND target_id=$2`, actor.ID, initial.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("concurrent loser left an audit", audits, err)
	}
}
