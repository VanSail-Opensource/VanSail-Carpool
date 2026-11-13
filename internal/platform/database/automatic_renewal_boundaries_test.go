package database

import (
	"context"
	"testing"
	"time"
)

func TestAutomaticRenewalDueAndFailureRespectExactExpiry(t *testing.T) {
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name         string
		expires, now time.Time
		due          bool
	}{
		{"fractional expiry before whole-second scan", base.Add(500 * time.Millisecond), base, false},
		{"whole-second expiry before fractional scan", base, base.Add(500 * time.Millisecond), true},
		{"inclusive fractional expiry", base.Add(500 * time.Millisecond), base.Add(500 * time.Millisecond), true},
		{"nanosecond before expiry", base.Add(500 * time.Millisecond), base.Add(500*time.Millisecond - time.Nanosecond), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 950001)
			combo := saveTestCombo(t, store, "boundary renewal", 0, 1)
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "boundary"}, tc.expires.Add(-24*time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, purchase.ValidFrom); err != nil {
				t.Fatal(err)
			}
			due, err := store.DueAutoRenewals(ctx, tc.now)
			if err != nil || (len(due) == 1) != tc.due {
				t.Fatalf("due = %+v, %v; want due %v", due, err, tc.due)
			}
			if err := store.MarkAutoRenewalFailed(ctx, purchase.ID, AutoRenewalReasonComboUnavailable, tc.now); err != nil {
				t.Fatal(err)
			}
			updated, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil {
				t.Fatal(err)
			}
			var notices int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=$1`, "auto-renewal-failed:"+purchase.ID).Scan(&notices); err != nil {
				t.Fatal(err)
			}
			if tc.due {
				if updated.AutoRenewEnabled || updated.Status != "expired" || notices != 1 {
					t.Fatalf("due failure = %+v, notices %d", updated, notices)
				}
			} else if !updated.AutoRenewEnabled || updated.Status != purchase.Status || notices != 0 {
				t.Fatalf("early failure changed paid term: %+v, notices %d", updated, notices)
			}
		})
	}
}

func TestAutomaticRenewalOtherTermUsesExactBoundary(t *testing.T) {
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		otherEnd, now time.Time
		blocked       bool
	}{
		{"fractional successor remains live", base.Add(500 * time.Millisecond), base, true},
		{"whole-second successor has ended", base, base.Add(500 * time.Millisecond), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 950002)
			combo := saveTestCombo(t, store, "replacement boundary", 0, 1)
			old, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "old"}, base.Add(-48*time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			next, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "next"}, base.Add(-24*time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='queued',valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(base.Add(-time.Hour)), stamp(tc.otherEnd), next.ID); err != nil {
				t.Fatal(err)
			}
			plan, err := store.AutoRenewalPlan(ctx, user.ID, old.ID, tc.now)
			if err != nil || (plan.IneligibleReason == AutoRenewalReasonQueuedPurchase) != tc.blocked {
				t.Fatalf("plan = %+v, %v; want blocked %v", plan, err, tc.blocked)
			}
		})
	}
}

func TestAutomaticRenewalFractionalSuccessorSuppressesFailure(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 950003)
	combo := saveTestCombo(t, store, "later fractional successor", 0, 1)
	old, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "old"}, base.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "next"}, base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='queued',valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(base), stamp(base.Add(500*time.Millisecond)), next.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.SetAutoRenewal(ctx, user.ID, old.ID, true, base.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkAutoRenewalFailed(ctx, old.ID, AutoRenewalReasonQueuedPurchase, base); err != nil {
		t.Fatal(err)
	}
	var enabled, notices int
	var reason string
	if err = store.DB().QueryRowContext(ctx, `SELECT auto_renew_enabled,auto_renew_failure_reason FROM purchases WHERE id=$1`, old.ID).Scan(&enabled, &reason); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=$1`, "auto-renewal-failed:"+old.ID).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 || reason != "" || notices != 0 {
		t.Fatalf("superseded failure = enabled %d, reason %q, notices %d", enabled, reason, notices)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET auto_renew_failure_reason=$1,auto_renew_failed_at=$2 WHERE id=$3`, AutoRenewalReasonQueuedPurchase, stamp(base), old.ID); err != nil {
		t.Fatal(err)
	}
	if failure, err := store.AutoRenewalFailure(ctx, user.ID); err != nil || failure != nil {
		t.Fatalf("superseded failure = %+v, %v", failure, err)
	}
}
