package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestMigrationClearsLegacySubscriptionCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20000)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET remna_subscription_url=? WHERE id=?`, "https://subscription.example/bearer", user.ID); err != nil {
		t.Fatalf("seed legacy subscription URL: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=?`, "010_clear_subscription_cache.sql"); err != nil {
		t.Fatalf("reset scrub migration: %v", err)
	}
	if err := migrate(ctx, store.DB()); err != nil {
		t.Fatalf("migrate(): %v", err)
	}
	var cached sql.NullString
	if err := store.DB().QueryRowContext(ctx, `SELECT remna_subscription_url FROM users WHERE id=?`, user.ID).Scan(&cached); err != nil {
		t.Fatalf("read scrubbed subscription URL: %v", err)
	}
	if cached.Valid {
		t.Fatalf("legacy subscription URL still persisted: %q", cached.String)
	}
}

func TestMigrationRestoresCompletedMissingRemnawaveAccount(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 20_099)
	now := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET username='restored',onboarding_state='agreement',
		accepted_agreement_revision=1,policy_accepted_at=NULL,recovery_reason='remnawave_user_missing',updated_at=? WHERE id=?`,
		stamp(now), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='050_restore_missing_remna_signup.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	restored, err := store.UserByID(ctx, user.ID)
	if err != nil || restored.OnboardingState != "complete" || restored.PolicyAcceptedAt == nil || restored.RecoveryReason != "" {
		t.Fatalf("restored account = %+v, err %v", restored, err)
	}
}

func TestLegacyReserveUsernameDoesNotStoreUsername(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20001)
	if err := store.ReserveUsername(ctx, user.ID, "river"); err != nil {
		t.Fatalf("ReserveUsername(first): %v", err)
	}
	if err := store.ReserveUsername(ctx, user.ID, "river"); err != nil {
		t.Fatalf("ReserveUsername(retry): %v", err)
	}
	if err := store.ReserveUsername(ctx, user.ID, "meadow"); err != nil {
		t.Fatalf("ReserveUsername(legacy caller): %v", err)
	}

	updated, err := store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserByID(): %v", err)
	}
	if updated.Username != nil {
		t.Fatalf("legacy reservation stored a username: %v", updated.Username)
	}
}

func TestQueueRemnawaveRepairPreservesCompletedSignup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20005)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET username='river',onboarding_state='complete',group_joined=1,channel_joined=1,
		policy_accepted_at=?,remna_user_id='remote-user',remna_subscription_url='https://subscription.example/token' WHERE id=?`, stamp(now), user.ID); err != nil {
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
			_, err := store.QueueRemnawaveRepair(ctx, user.ID, "remote-user", now.Add(time.Second))
			results <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("QueueRemnawaveRepair() error = %v", err)
		}
	}
	recovered, err := store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Username == nil || *recovered.Username != "river" || recovered.OnboardingState != "complete" ||
		recovered.RecoveryReason != "" || recovered.RemnaUserID != nil || recovered.PolicyAcceptedAt == nil ||
		!recovered.GroupJoined || !recovered.ChannelJoined {
		t.Fatalf("recovered user = %+v", recovered)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.Minor != "500" {
		t.Fatalf("preserved balance = (%+v, %v)", balance, err)
	}
	if _, err := store.QueueRemnawaveRepair(ctx, user.ID, "other-remote", now.Add(2*time.Second)); err != nil {
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
