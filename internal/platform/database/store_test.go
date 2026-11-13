package database

import (
	"context"

	"errors"
	"testing"
	"time"
)

func TestBeginAgreementIsIdempotentAndPreservesCompletedSignup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20001)
	for range 2 {
		if err := store.BeginAgreement(ctx, user.ID); err != nil {
			t.Fatal("agreement progression failed", err)
		}
	}
	updated, err := store.UserByID(ctx, user.ID)
	if err != nil || updated.Username != nil || updated.OnboardingState != "agreement" {
		t.Fatal("agreement progression changed username or state", updated, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete',
		group_joined=1,channel_joined=1,policy_accepted_at=$1 WHERE id=$2`, stamp(time.Now()), user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginAgreement(ctx, user.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("completed signup was rewound", err)
	}
	updated, err = store.UserByID(ctx, user.ID)
	if err != nil || updated.OnboardingState != "complete" || updated.Username != nil {
		t.Fatal("completed signup did not remain intact", updated, err)
	}
}

func TestQueueProxyRepairPreservesCompletedSignup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20005)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete',group_joined=1,channel_joined=1,
		policy_accepted_at=$1,proxy_user_id='remote-user' WHERE id=$2`, stamp(now), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "recovery-balance", "preserved", now); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := store.QueueProxyRepair(ctx, user.ID, "remote-user", now.Add(time.Second))
			results <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("QueueProxyRepair() error = %v", err)
		}
	}
	recovered, err := store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.TelegramID != int64(20005) || recovered.OnboardingState != "complete" ||
		recovered.RecoveryReason != "" || recovered.ProxyUserID != nil || recovered.PolicyAcceptedAt == nil ||
		!recovered.GroupJoined || !recovered.ChannelJoined {
		t.Fatalf("recovered user = %+v", recovered)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.Minor != "500" {
		t.Fatalf("preserved balance = (%+v, %v)", balance, err)
	}
	if _, err := store.QueueProxyRepair(ctx, user.ID, "other-remote", now.Add(2*time.Second)); err != nil {
		t.Fatalf("second repair should leave newer identity unchanged: %v", err)
	}
}

func TestPurchaseTrafficResetPhasesAreDurableAndIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20002)
	combo := saveTestCombo(t, store, "reset-once", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "reset-seed", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "reset-phase"}, time.Now())
	if err != nil {
		t.Fatalf("CreatePurchase(): %v", err)
	}
	phase, err := store.PurchaseTrafficResetPhase(ctx, purchase.ID)
	if err != nil || phase != "pending" {
		t.Fatalf("PurchaseTrafficResetPhase(initial) = %q, %v", phase, err)
	}
	if err := store.AdvancePurchaseTrafficReset(ctx, purchase.ID, "pending", "quiesced", time.Now()); err != nil {
		t.Fatalf("AdvancePurchaseTrafficReset(quiesced): %v", err)
	}
	if err := store.AdvancePurchaseTrafficReset(ctx, purchase.ID, "pending", "quiesced", time.Now().Add(time.Second)); err != nil {
		t.Fatalf("AdvancePurchaseTrafficReset(replay): %v", err)
	}
	if err := store.AdvancePurchaseTrafficReset(ctx, purchase.ID, "quiesced", "reset", time.Now().Add(2*time.Second)); err != nil {
		t.Fatalf("AdvancePurchaseTrafficReset(reset): %v", err)
	}
	phase, err = store.PurchaseTrafficResetPhase(ctx, purchase.ID)
	if err != nil || phase != "reset" {
		t.Fatalf("PurchaseTrafficResetPhase(final) = %q, %v", phase, err)
	}
	if err := store.AdvancePurchaseTrafficReset(ctx, purchase.ID, "pending", "reset", time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("AdvancePurchaseTrafficReset(invalid) = %v, want ErrConflict", err)
	}
}
