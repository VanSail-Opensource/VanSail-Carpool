package database

import (
	"context"
	"testing"
	"time"
)

func TestPanelEntryVerificationAndOnboardingCleanup(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 31910)
	check := func(want bool) {
		t.Helper()
		got, err := store.PanelEntryQualified(ctx, user.TelegramID)
		if err != nil || got != want {
			t.Fatalf("qualified=%v, %v", got, err)
		}
	}
	check(false)
	if err := store.RegisterPanelEntry(ctx, user.TelegramID, true); err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := store.VerifyPanelEntry(ctx, user.TelegramID); err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := store.RegisterPanelEntry(ctx, user.TelegramID, true); err != nil {
		t.Fatal(err)
	}
	check(true)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET username='verified',onboarding_state='agreement' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	revision, ids, err := store.CurrentAgreementContract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteOnboardingRevision(ctx, user.ID, revision, ids, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM panel_entry_verification WHERE telegram_id=$1`, user.TelegramID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rows=%d, %v", count, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='agreement' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET policy_accepted_at=NULL WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterPanelEntry(ctx, user.TelegramID, true); err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM panel_entry_verification`).Scan(&count); err != nil || count != 0 {
		t.Fatal("previously onboarded row recreated")
	}
}

func TestDisabledCaptchaQualifiesOnlyAuthenticatedPanelEntry(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 31911)
	if err := store.VerifyPanelEntry(ctx, user.TelegramID); err != nil {
		t.Fatal(err)
	}
	if qualified, _ := store.PanelEntryQualified(ctx, user.TelegramID); qualified {
		t.Fatal("verification without entry qualified")
	}
	if err := store.RegisterPanelEntry(ctx, user.TelegramID, false); err != nil {
		t.Fatal(err)
	}
	if qualified, err := store.PanelEntryQualified(ctx, user.TelegramID); !qualified || err != nil {
		t.Fatalf("disabled entry=%v,%v", qualified, err)
	}
	if err := store.RegisterPanelEntry(ctx, 999999, false); err != nil {
		t.Fatal(err)
	}
	if qualified, _ := store.PanelEntryQualified(ctx, 999999); qualified {
		t.Fatal("unrecognized identity qualified")
	}
	var columns int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_attribute WHERE attrelid=to_regclass('panel_entry_verification') AND attnum>0 AND NOT attisdropped`).Scan(&columns); err != nil || columns != 2 {
		t.Fatalf("columns=%d,%v", columns, err)
	}
}
