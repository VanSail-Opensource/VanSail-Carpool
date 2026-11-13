package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHourlyActivationBlocksAcceptedZeroPriceLegacySuccessor(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	combo := saveTestCombo(t, s, "zero-price-grandfathered", 0, 30)
	legacy, err := s.CreatePurchase(ctx, PurchaseInput{UserID: input.OwnerID, ComboID: combo.ID, IdempotencyKey: "zero-legacy"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE purchases SET status='queued' WHERE id=$1`, legacy.ID); err != nil {
		t.Fatal(err)
	}
	before := hourlyBillingBalance(t, s, input.OwnerID)
	if _, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now()); !errors.Is(err, ErrHourlyLegacyConflict) {
		t.Fatalf("zero-price queued purchased benefit did not block hourly admission: %v", err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM hourly_enrollments`).Scan(&count); err != nil || count != 0 || hourlyBillingBalance(t, s, input.OwnerID) != before {
		t.Fatalf("rejected activation mutated account: %d %v", count, err)
	}
}

func TestHourlyPreparationBlocksNewLegacyPurchasesButPreservesTheirReplays(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	combo := saveTestCombo(t, s, "legacy-reciprocal", 100, 30)
	request := PurchaseInput{UserID: input.OwnerID, ComboID: combo.ID, IdempotencyKey: "legacy-before-hourly"}
	legacy, err := s.CreatePurchase(ctx, request, time.Now().Add(-31*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Preserve an accepted receipt whose fixed term has already elapsed.
	prepared := prepareHourlyBillingFixture(t, s, input, policy)
	replay, err := s.CreatePurchase(ctx, request, time.Now())
	if err != nil || replay.ID != legacy.ID {
		t.Fatalf("hourly intent hid prior legacy receipt: %+v %v", replay, err)
	}
	request.IdempotencyKey = "legacy-after-hourly"
	before := hourlyBillingBalance(t, s, input.OwnerID)
	if _, err := s.CreatePurchase(ctx, request, time.Now()); !errors.Is(err, ErrHourlyLegacyConflict) {
		t.Fatalf("legacy checkout ignored hourly intent: %v", err)
	}
	if _, err := s.Renew(ctx, RenewalInput{UserID: input.OwnerID, PurchaseID: legacy.ID, IdempotencyKey: "renew-after-hourly", TermCount: 1}, time.Now()); !errors.Is(err, ErrHourlyLegacyConflict) {
		t.Fatalf("manual renewal ignored hourly intent: %v", err)
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != before {
		t.Fatal("rejected legacy purchase changed wallet")
	}
	if _, err := s.DisableHourlyEnrollment(ctx, HourlyDisableInput{OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration,
		EnrollmentID: prepared.EnrollmentID, Voluntary: true}, policy, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePurchase(ctx, request, time.Now()); err != nil {
		t.Fatalf("cancelled unpaid preparation blocked future legacy checkout: %v", err)
	}
}

func TestNewLegacyTermsRetainPurchasedDurationWithoutBackfill(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, 89711)
	addon := saveTestSquad(t, s, "legacy-duration-addon", 10, true)
	combo := saveTestCombo(t, s, "legacy-duration", 100, 30)
	if _, err := s.AdjustBalance(ctx, u.ID, 10000, "duration-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.GroupID}, IdempotencyKey: "duration-initial"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := s.Renew(ctx, RenewalInput{UserID: u.ID, PurchaseID: p.ID, IdempotencyKey: "duration-renew", TermCount: 2}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{p.ID, batch.Purchases[0].ID, batch.Purchases[1].ID} {
		var seconds int64
		if err := s.DB().QueryRow(`SELECT purchased_term_seconds FROM purchases WHERE id=$1`, id).Scan(&seconds); err != nil || seconds != 30*24*60*60 {
			t.Fatalf("purchased duration = %d, %v", seconds, err)
		}
	}
	if _, err := s.DB().Exec(`UPDATE purchases SET purchased_term_seconds=purchased_term_seconds+1 WHERE id=$1`, p.ID); err == nil {
		t.Fatal("accepted purchased duration rewrite")
	}
	// Construct a pre-snapshot historical row; readers must not backfill it.
	if _, err := s.DB().Exec(`INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,gross_price_txb_minor,valid_from,valid_until,status,created_at,updated_at)
		SELECT 'historical-no-duration',user_id,combo_id,charged_txb_minor,gross_price_txb_minor,valid_from,valid_until,'expired',created_at,updated_at
		FROM purchases WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PurchaseByID(ctx, "historical-no-duration"); err != nil {
		t.Fatal(err)
	}
	var missing bool
	if err := s.DB().QueryRow(`SELECT purchased_term_seconds IS NULL FROM purchases WHERE id='historical-no-duration'`).Scan(&missing); err != nil || !missing {
		t.Fatalf("reading old term invented a snapshot: %v %v", missing, err)
	}
	if _, err := s.DB().Exec(`UPDATE purchases SET purchased_term_seconds=2592000 WHERE id='historical-no-duration'`); err == nil {
		t.Fatal("accepted historical purchased duration backfill")
	}
}

func TestHourlyLegacyExclusionPreservesNanosecondExpiry(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "Legacy boundary", 100, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 89712, combo, now)
	for _, remaining := range []time.Duration{time.Nanosecond, 0, -time.Nanosecond} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(now.Add(remaining)), purchase.ID); err != nil {
			t.Fatal(err)
		}
		blocked, err := store.HourlyLegacyBlocked(ctx, user.ID, now)
		if err != nil || blocked != (remaining > 0) {
			t.Fatalf("legacy view at %v = %v, %v", remaining, blocked, err)
		}
		tx, err := store.DB().BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = rejectLegacyHourlyActivationTx(ctx, tx, user.ID, now)
		_ = tx.Rollback()
		if remaining > 0 && !errors.Is(err, ErrHourlyLegacyConflict) || remaining <= 0 && err != nil {
			t.Fatalf("legacy admission at %v = %v", remaining, err)
		}
	}
}
