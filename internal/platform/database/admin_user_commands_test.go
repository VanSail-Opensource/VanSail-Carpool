package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminCouponGrantAndDiscardAreIdempotentAndHistorical(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Now().UTC()
	actor, user := createTestUser(t, store, 48_201), createTestUser(t, store, 48_202)
	coupon, err := store.SaveCoupon(ctx, coupons.CouponInput{Code: "ADMIN25", Name: "Admin", Kind: coupons.KindPurchaseOnce,
		DiscountMode: coupons.DiscountFixed, ValueMinorOrBPS: 25, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	input := AdminCouponGrantInput{ActorUserID: actor.ID, UserID: user.ID, CouponID: coupon.ID, IdempotencyKey: "grant-key", Reason: "support adjustment"}
	first, err := store.GrantAdminCoupon(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.GrantAdminCoupon(ctx, input, now.Add(time.Minute))
	if err != nil || second.ID != first.ID {
		t.Fatalf("GrantAdminCoupon(replay) = (%+v, %v)", second, err)
	}
	if err = store.DiscardAdminCoupon(ctx, actor.ID, user.ID, first.ID, "discard-key", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = store.DiscardAdminCoupon(ctx, actor.ID, user.ID, first.ID, "discard-key", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("DiscardAdminCoupon(replay): %v", err)
	}
	wallet, err := store.ListCouponGrants(ctx, user.ID, now.Add(4*time.Minute))
	if err != nil || len(wallet) != 0 {
		t.Fatalf("wallet after discard = (%+v, %v)", wallet, err)
	}
	var discards, commands, audits int
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_grant_discards WHERE grant_id=?`, first.ID).Scan(&discards); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_coupon_discard_commands WHERE grant_id=?`, first.ID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='coupon.discard' AND target_id=?`, first.ID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if discards != 1 || commands != 1 || audits != 1 {
		t.Fatalf("discard history = (%d, %d, %d), want one each", discards, commands, audits)
	}
}

func TestAdminRefundBoundsAndReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Now().UTC()
	actor := createTestUser(t, store, 48_211)
	combo := saveTestCombo(t, store, "admin-refund-bounds", 900, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 48_212, combo, now)
	bad := AdminEntitlementRefundInput{ActorUserID: actor.ID, UserID: user.ID, PurchaseID: purchase.ID,
		IdempotencyKey: "refund-bad", RequestFingerprint: "refund-bad-fingerprint", Reason: "too much", AmountTXBMinor: purchase.PriceTXBMinor + 1}
	if _, err := store.RefundAdminEntitlement(ctx, bad, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("RefundAdminEntitlement(over-limit) = %v, want conflict", err)
	}
	partial := bad
	partial.IdempotencyKey, partial.RequestFingerprint, partial.AmountTXBMinor = "refund-partial", "refund-partial-fingerprint", 300
	before := adminWorkflowBalance(t, store, user.ID)
	first, err := store.RefundAdminEntitlement(ctx, partial, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.RefundAdminEntitlement(ctx, partial, now.Add(2*time.Minute))
	if err != nil || second.ID != first.ID || adminWorkflowBalance(t, store, user.ID) != before+partial.AmountTXBMinor {
		t.Fatalf("partial refund replay = (%+v, %v), balance=%d", second, err, adminWorkflowBalance(t, store, user.ID))
	}
	partial.AmountTXBMinor, partial.RequestFingerprint = 301, "refund-replay-conflict"
	if _, err = store.RefundAdminEntitlement(ctx, partial, now.Add(3*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed refund replay = %v, want conflict", err)
	}
}

func TestTemporaryBanRestorationWaitsForSuccessfulBanAndPreservesOverlap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Now().UTC()
	actor, user := createTestUser(t, store, 48_221), createTestUser(t, store, 48_222)
	ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: user.ID,
		IdempotencyKey: "ban-key", RequestFingerprint: "ban-command-fingerprint", Reason: "investigation", DurationMinutes: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: user.ID,
		IdempotencyKey: "other-ban", RequestFingerprint: "other-ban-command-fingerprint", Reason: "overlap", DurationMinutes: 1}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("overlapping ban = %v, want conflict", err)
	}
	if err = store.QueueExpiredAdminTemporaryBan(ctx, user.ID, now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("expired queued ban = %v, want conflict retry", err)
	}
	if _, err = store.BeginProviderOperationAttempt(ctx, ban.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CompleteProviderOperation(ctx, ban.ID, providerops.Completion{Status: providerops.StatusSucceeded}, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = store.QueueExpiredAdminTemporaryBan(ctx, user.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	active, err := store.ActiveAdminTemporaryBan(ctx, user.ID)
	if err != nil || active == nil || active.UnbanOperationID == "" {
		t.Fatalf("scheduled restoration = (%+v, %v)", active, err)
	}
	if err = store.CompleteAdminTemporaryUnban(ctx, user.ID, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if active, err = store.ActiveAdminTemporaryBan(ctx, user.ID); err != nil || active != nil {
		t.Fatalf("restored ban = (%+v, %v)", active, err)
	}
}
