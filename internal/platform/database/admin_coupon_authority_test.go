package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestAdminCouponMutationsRequireCurrentAuthorityAndPreserveReplays(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	actor, owner := createTestAdmin(t, store, 48231), createTestUser(t, store, 48232)
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "CURRENTADMIN", Name: "Current authority",
		Kind: coupons.KindPurchaseOnce, DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 25, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	input := AdminCouponGrantInput{ActorUserID: actor.ID, UserID: owner.ID, CouponID: coupon.ID,
		IdempotencyKey: "accepted-grant", Reason: "support adjustment"}
	accepted, err := store.GrantAdminCoupon(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	spareInput := input
	spareInput.IdempotencyKey = "retained-active-grant"
	spare, err := store.GrantAdminCoupon(ctx, spareInput, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.DiscardAdminCoupon(ctx, actor.ID, owner.ID, accepted.ID, "accepted-discard", now); err != nil {
		t.Fatal(err)
	}
	assertReplay := func() {
		t.Helper()
		replay, err := store.GrantAdminCoupon(ctx, input, now.Add(time.Minute))
		if err != nil || replay.ID != accepted.ID {
			t.Fatal("accepted grant depended on later authority", replay, err)
		}
		if err = store.DiscardAdminCoupon(ctx, actor.ID, owner.ID, accepted.ID, "accepted-discard", now.Add(time.Minute)); err != nil {
			t.Fatal("accepted discard depended on later authority", err)
		}
	}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`,
			state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		fresh := input
		fresh.IdempotencyKey = "fresh-" + state.name
		if _, err = store.GrantAdminCoupon(ctx, fresh, now); !errors.Is(err, ErrConflict) {
			t.Fatal("ineligible administrator granted coupon", state.name, err)
		}
		if err = store.DiscardAdminCoupon(ctx, actor.ID, owner.ID, spare.ID, "fresh-"+state.name, now); !errors.Is(err, ErrConflict) {
			t.Fatal("ineligible administrator discarded coupon", state.name, err)
		}
		assertReplay()
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	fresh := input
	fresh.IdempotencyKey = "purging-owner"
	if _, err = store.GrantAdminCoupon(ctx, fresh, now); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("fresh grant ignored owner lifecycle", err)
	}
	if err = store.DiscardAdminCoupon(ctx, actor.ID, owner.ID, spare.ID, "purging-owner", now); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("fresh discard ignored owner lifecycle", err)
	}
	assertReplay()
	var grants, discards, commands, audits int
	if err = store.DB().QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM coupon_grants WHERE user_id=$1),
		(SELECT COUNT(*) FROM coupon_grant_discards WHERE user_id=$1),
		(SELECT COUNT(*) FROM admin_coupon_discard_commands WHERE actor_user_id=$2),
		(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$2 AND action IN ('coupon.grant','coupon.discard'))`,
		owner.ID, actor.ID).Scan(&grants, &discards, &commands, &audits); err != nil || grants != 2 || discards != 1 || commands != 1 || audits != 3 {
		t.Fatal("rejected or replayed coupon command changed history", grants, discards, commands, audits, err)
	}
	var status string
	if err = store.DB().QueryRowContext(ctx, `SELECT status FROM coupon_grants WHERE id=$1`, spare.ID).Scan(&status); err != nil || status != "active" {
		t.Fatal("refused discard consumed retained grant", status, err)
	}
}
