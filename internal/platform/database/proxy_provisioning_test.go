package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProvisioningConflictResolutionRemainsClosedAndPreservesPaidAccess(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 55_100)
	combo := saveTestCombo(t, store, "conflicted", 100, 30)
	now := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete',
		policy_accepted_at=$1,accepted_agreement_revision=1,group_joined=1,channel_joined=1 WHERE id=$2`, stamp(now), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "conflict-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	current, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "current"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, current.ID, true, now); err != nil {
		t.Fatal(err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "queued"}, now.Add(time.Minute))
	if err != nil || queued.Status != "queued" {
		t.Fatalf("queued purchase = %+v, %v", queued, err)
	}
	// A later paid add-on increases the purchase's charged total.
	if _, err := store.AdjustBalance(ctx, user.ID, -25, "addon-seed", "add-on", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET charged_txb_minor=charged_txb_minor+25 WHERE id=$1`, current.ID); err != nil {
		t.Fatal(err)
	}
	beforeBalance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ResolveProvisioningConflict(ctx, user.ID, user.AccountName(), now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("retired provisioning conflict resolution error = %v, want ErrConflict", err)
	}
	currentAfter, err := store.PurchaseByID(ctx, current.ID)
	if err != nil {
		t.Fatal(err)
	}
	queuedAfter, err := store.PurchaseByID(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := store.UserByID(ctx, user.ID)
	if err != nil || recovered.TelegramID != user.TelegramID || recovered.OnboardingState != "complete" ||
		recovered.ProxyUserID != nil || recovered.PolicyAcceptedAt == nil || !recovered.GroupJoined || !recovered.ChannelJoined {
		t.Fatalf("member access changed = %+v, err %v", recovered, err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance != beforeBalance {
		t.Fatalf("balance changed = (%+v, %v), before %+v", balance, err, beforeBalance)
	}
	if currentAfter.Status != "active" || queuedAfter.Status != "queued" {
		t.Fatalf("paid access changed: current=%s queued=%s", currentAfter.Status, queuedAfter.Status)
	}
	var refunds, notices int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='proxy_identity_conflict_refund'`, user.ID).Scan(&refunds); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE user_id=$1 AND kind='proxy_identity_conflict_refund'`, user.ID).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if refunds != 0 || notices != 0 {
		t.Fatalf("retired conflict path wrote refunds/notices = %d/%d", refunds, notices)
	}
}

func TestUnlinkedPaidBacklogQueuesOneRepair(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 55_101)
	combo := saveTestCombo(t, store, "repair", 100, 30)
	now := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete',
		policy_accepted_at=$1 WHERE id=$2`, stamp(now), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "repair-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "repair"}, now); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.EnqueueUnlinkedPaidUsers(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user'
		AND (CASE ((payload)::jsonb #> '{userId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{userId}' END)=$1 AND status='pending'`, user.ID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("repair jobs = %d, err %v", jobs, err)
	}
}

func TestProvisioningUsesExactPaidWindowAndActiveOwner(t *testing.T) {
	for _, test := range []struct {
		name   string
		until  time.Duration
		closed bool
		live   bool
	}{
		{name: "final nanosecond", until: time.Nanosecond, live: true},
		{name: "exact expiry"},
		{name: "closed owner", until: time.Hour, closed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			combo := saveTestCombo(t, store, "precise provisioning", 100, 30)
			user, purchase := createAdminWorkflowPurchase(t, store, 55_102, combo, now.Add(-time.Hour))
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(now.Add(test.until)), purchase.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=NULL,onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			if test.closed {
				// Close an already-onboarded owner; completing onboarding on a
				// closed owner is deliberately rejected by the lifecycle trigger.
				if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, user.ID); err != nil {
					t.Fatal(err)
				}
			}
			if live, err := store.HasProvisionablePurchase(ctx, user.ID, now); err != nil || live != test.live {
				t.Fatal("incorrect paid admission", live, err)
			}
			if err := store.EnqueueUnlinkedPaidUsers(ctx, now); err != nil {
				t.Fatal(err)
			}
			var jobs int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload::jsonb->>'userId'=$1`, user.ID).Scan(&jobs); err != nil || jobs != boolInt(test.live) {
				t.Fatal("incorrect repair admission", jobs, err)
			}
			err := store.LinkProvisionedProxyUser(ctx, user.ID, user.AccountName(), "", "90217", now)
			if test.live && err != nil || !test.live && !errors.Is(err, ErrConflict) {
				t.Fatal("incorrect identity link admission", err)
			}
		})
	}
}
