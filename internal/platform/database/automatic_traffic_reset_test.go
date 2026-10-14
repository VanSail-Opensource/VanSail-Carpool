package database

import (
	"context"
	"strconv"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAutomaticTrafficResetDebitIsIdempotentAndSuccessGated(t *testing.T) {
	t.Parallel()
	store, userID, _, now := automaticResetFixture(t, 88_100, 5_000)
	ctx := context.Background()

	result, err := store.ProcessAutomaticTrafficResetObservation(ctx, "88100", 1_000, 1_000, "DAY", nil, now)
	if err != nil || !result.Handled || !result.EventCreated {
		t.Fatalf("ProcessAutomaticTrafficResetObservation() = (%+v, %v)", result, err)
	}
	replay, err := store.ProcessAutomaticTrafficResetObservation(ctx, "88100", 1_000, 1_000, "DAY", nil, now.Add(time.Minute))
	if err != nil || !replay.Handled || replay.EventCreated {
		t.Fatalf("automatic reset replay = (%+v, %v)", replay, err)
	}
	assertAutomaticResetCounts(t, store, userID, 1, 1)
	assertNotificationCounts(t, store, 2, 1)

	var operationID string
	if err := store.DB().QueryRow(`SELECT id FROM provider_operations WHERE kind='purchase_traffic_reset'`).Scan(&operationID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operationID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operationID, "purchase", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	completedAt := now.Add(2 * time.Minute)
	if _, err := store.CompleteProviderOperationItemAndReleaseNotifications(ctx, operationID, "purchase",
		providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: "{}"}, completedAt); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 2, 2)
	var completion string
	if err := store.DB().QueryRow(`SELECT json_extract(payload,'$.facts.time') FROM outbox_jobs
		WHERE kind=? AND json_extract(payload,'$.kind')=?`, jobpayload.UserNotificationKind,
		jobpayload.UserEventAutomaticReset).Scan(&completion); err != nil || completion != completedAt.Format(time.RFC3339Nano) {
		t.Fatalf("completion time = %q, %v", completion, err)
	}
}

func TestAutomaticTrafficResetInsufficientBalanceDisablesPreference(t *testing.T) {
	t.Parallel()
	store, userID, _, now := automaticResetFixture(t, 88_101, 301)
	ctx := context.Background()

	result, err := store.ProcessAutomaticTrafficResetObservation(ctx, "88101", 991, 1_000, "NO_RESET", nil, now)
	if err != nil || !result.Handled || !result.EventCreated {
		t.Fatalf("insufficient automatic reset = (%+v, %v)", result, err)
	}
	setting, err := store.TrafficResetAutomation(ctx, userID)
	if err != nil || setting.Enabled {
		t.Fatalf("automation setting = (%+v, %v), want disabled", setting, err)
	}
	assertAutomaticResetCounts(t, store, userID, 0, 0)
	assertNotificationCounts(t, store, 2, 2)
	if replay, err := store.ProcessAutomaticTrafficResetObservation(ctx, "88101", 999, 1_000, "NO_RESET", nil, now.Add(time.Minute)); err != nil || replay.Handled {
		t.Fatalf("disabled replay = (%+v, %v)", replay, err)
	}
}

func TestAutomaticTrafficResetFailureCompensatesAndNotifies(t *testing.T) {
	t.Parallel()
	store, userID, _, now := automaticResetFixture(t, 88_102, 5_000)
	ctx := context.Background()
	if _, err := store.ProcessAutomaticTrafficResetObservation(ctx, "88102", 1_000, 1_000, "DAY", nil, now); err != nil {
		t.Fatal(err)
	}
	var operationID string
	if err := store.DB().QueryRow(`SELECT id FROM provider_operations WHERE kind='purchase_traffic_reset'`).Scan(&operationID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operationID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operationID, "purchase", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.CompensateTrafficReset(ctx, operationID, "RESET_REJECTED", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertAutomaticResetCounts(t, store, userID, 1, 1)
	assertNotificationCounts(t, store, 3, 2)
}

func automaticResetFixture(t *testing.T, telegramID, seed int64) (*Store, string, string, time.Time) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, telegramID)
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, seed, "automatic-reset-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Automatic reset", 301, 30)
	if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET reset_strategy='DAY' WHERE id=?`, combo.ID); err != nil {
		t.Fatal(err)
	}
	purchase := createMemberOperationPurchase(t, store, user.ID, combo.ID, "automatic-reset-purchase", now.Add(-time.Hour))
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET remna_user_id=? WHERE id=?`, strconv.FormatInt(telegramID, 10), user.ID); err != nil {
		t.Fatal(err)
	}
	setting, err := store.TrafficResetAutomation(ctx, user.ID)
	if err != nil || !setting.Enabled {
		t.Fatalf("TrafficResetAutomation(default) = (%+v, %v), want enabled", setting, err)
	}
	return store, user.ID, purchase.ID, now
}

func assertAutomaticResetCounts(t *testing.T, store *Store, userID string, operations, debits int) {
	t.Helper()
	var operationCount, debitCount int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM provider_operations WHERE kind='purchase_traffic_reset'`).Scan(&operationCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='traffic_reset_debit'`, userID).Scan(&debitCount); err != nil {
		t.Fatal(err)
	}
	if operationCount != operations || debitCount != debits {
		t.Fatalf("automatic reset counts = operations %d debits %d, want %d/%d", operationCount, debitCount, operations, debits)
	}
}
