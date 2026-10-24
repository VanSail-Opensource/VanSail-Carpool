package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestOnboardingQueuesCredentialsWithoutComboPurchase(t *testing.T) {
	t.Parallel()
	for _, payg := range []bool{false, true} {
		t.Run(map[bool]string{false: "no PAYG", true: "PAYG enabled"}[payg], func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 31920)
			if payg {
				if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: "payg", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.BeginAgreement(ctx, user.ID); err != nil {
				t.Fatal(err)
			}
			revision, ids, err := store.CurrentAgreementContract(ctx)
			if err != nil {
				t.Fatal(err)
			}
			completed, err := store.CompleteOnboardingRevision(ctx, user.ID, revision, ids, time.Now().UTC())
			if err != nil || completed.OnboardingState != "complete" {
				t.Fatalf("onboarding = %+v, %v", completed, err)
			}
			for kind, want := range map[string]int{xnode.AccessRepairJob: 1, xnode.PAYGJob: boolInt(payg), "xnode_sync_user": 0} {
				var count int
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.userId')=? AND status='pending'`, kind, user.ID).Scan(&count); err != nil || count != want {
					t.Fatalf("%s jobs = %d, want %d: %v", kind, count, want, err)
				}
			}
			balance, err := store.Balance(ctx, user.ID)
			if err != nil || balance.MinorInt64() != 0 {
				t.Fatal("onboarding changed wallet", balance, err)
			}
		})
	}
}

func TestAccessRepairBackfillsOnlyCompletedMembersWithoutPurchases(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	completed := createTestUser(t, store, 31921)
	incomplete := createTestUser(t, store, 31922)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=?`, completed.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.EnqueueXNodeAccessRepairs(ctx, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	for user, want := range map[string]int{completed.ID: 1, incomplete.ID: 0} {
		var count int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND json_extract(payload,'$.userId')=? AND status='pending'`, xnode.AccessRepairJob, user).Scan(&count); err != nil || count != want {
			t.Fatalf("member %s repair jobs = %d, want %d: %v", user, count, want, err)
		}
	}
}
