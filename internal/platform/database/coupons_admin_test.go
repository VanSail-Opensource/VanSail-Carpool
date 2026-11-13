package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestAdminCouponAuditFailureRollsBackDefinition(t *testing.T) {
	for _, operation := range []string{"create", "update", "deactivate"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			actor := createTestUser(t, store, 89120)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			input := coupons.CouponInput{Code: "AUDIT_ONLY", Name: "Before", Kind: coupons.KindBalanceAdd, ValueMinorOrBPS: 100, Active: true}
			var expected time.Time
			if operation != "create" {
				previous, err := store.SaveCoupon(ctx, input, now)
				if err != nil {
					t.Fatal(err)
				}
				input.ID, expected = previous.ID, previous.UpdatedAt
				input.Name = "After"
				input.Active = operation != "deactivate"
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE FUNCTION reject_coupon_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `CREATE TRIGGER reject_coupon_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.target_type='coupon') EXECUTE FUNCTION reject_coupon_audit()`); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SaveAdminCoupon(ctx, actor.ID, input, expected, now); err == nil {
				t.Fatal("definition committed without its audit")
			}
			items, err := store.ListCoupons(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			if operation == "create" && len(items) != 0 || operation != "create" && (len(items) != 1 || items[0].Name != "Before" || !items[0].Active || !items[0].UpdatedAt.Equal(expected)) {
				t.Fatal("failed audit retained a definition change", items)
			}
			if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER reject_coupon_audit ON audit_events`); err != nil {
				t.Fatal(err)
			}
			committed, err := store.SaveAdminCoupon(ctx, actor.ID, input, expected, now)
			if err != nil || committed.Name != input.Name || committed.Active != input.Active {
				t.Fatal("retry did not return committed definition", committed, err)
			}
			var audits int
			var detail string
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(detail),'') FROM audit_events WHERE actor_user_id=$1 AND action='coupon.save' AND target_type='coupon' AND target_id=$2`, actor.ID, committed.ID).Scan(&audits, &detail); err != nil || audits != 1 || strings.Contains(detail, input.Code) {
				t.Fatal("retry did not create one redacted actor audit", audits, detail, err)
			}
		})
	}
}

func TestAdminCouponRejectsIneligibleActorAndPrivateReward(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestUser(t, store, 89121)
	now := time.Now().UTC()
	input := coupons.CouponInput{Code: "ACTOR_ONLY", Name: "Protected", Kind: coupons.KindBalanceAdd, ValueMinorOrBPS: 100, Active: true}
	for _, actorID := range []string{"", actor.ID} {
		if _, err := store.SaveAdminCoupon(ctx, actorID, input, time.Time{}, now); !errors.Is(err, ErrConflict) {
			t.Fatal("missing/non-admin actor accepted", err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	private, err := store.SaveCoupon(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE coupon_definitions SET admin_visible=0 WHERE id=$1`, private.ID); err != nil {
		t.Fatal(err)
	}
	input.ID, input.Name = private.ID, "Public overwrite"
	if _, err := store.SaveAdminCoupon(ctx, actor.ID, input, private.UpdatedAt, now); !errors.Is(err, ErrNotFound) {
		t.Fatal("private reward definition accepted by catalog mutation", err)
	}
	stored, err := couponByID(ctx, store.DB(), private.ID)
	if err != nil || stored.Name != private.Name {
		t.Fatal("private definition changed", stored, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='coupon.save' AND target_type='coupon'`, actor.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("rejected coupon mutations left audits", audits, err)
	}
}
