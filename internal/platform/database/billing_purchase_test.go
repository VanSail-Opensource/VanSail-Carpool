package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCreatePurchaseSnapshotsAndRenewsAtCurrentTermEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10001)
	included := saveTestSquad(t, store, "included-squad", 900, true)
	addon := saveTestSquad(t, store, "addon-squad", 300, true)
	combo := saveTestCombo(t, store, "monthly", 1_000, 30, included.ID)
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "seed-credit", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}

	now := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	first, err := store.CreatePurchase(ctx, PurchaseInput{
		UserID:         user.ID,
		ComboID:        combo.ID,
		AddonSquadIDs:  []string{addon.ID, addon.ID},
		IdempotencyKey: "purchase-snapshot-first",
	}, now)
	if err != nil {
		t.Fatalf("CreatePurchase(first): %v", err)
	}
	if first.Status != "activating" {
		t.Fatalf("first status = %q, want activating", first.Status)
	}
	if !first.ValidFrom.Equal(now) {
		t.Fatalf("first validFrom = %s, want %s", first.ValidFrom, now)
	}
	if want := now.AddDate(0, 0, 30); !first.ValidUntil.Equal(want) {
		t.Fatalf("first validUntil = %s, want %s", first.ValidUntil, want)
	}
	if first.PriceTXBMinor != 1_300 {
		t.Fatalf("first price = %d, want 1300", first.PriceTXBMinor)
	}
	if first.CoreGrossTXBMinor != 1_000 {
		t.Fatalf("first core gross = %d, want 1000", first.CoreGrossTXBMinor)
	}
	if got, want := first.SquadUUIDs, []string{"addon-squad", "included-squad"}; !equalStrings(got, want) {
		t.Fatalf("first squads = %v, want %v", got, want)
	}

	second, err := store.CreatePurchase(ctx, PurchaseInput{
		UserID:         user.ID,
		ComboID:        combo.ID,
		AddonSquadIDs:  []string{addon.ID},
		IdempotencyKey: "purchase-snapshot-second",
	}, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("CreatePurchase(renewal): %v", err)
	}
	if second.Status != "queued" {
		t.Fatalf("renewal status = %q, want queued", second.Status)
	}
	var provisioningJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_sync_user'
		AND json_extract(payload,'$.userId')=?`, user.ID).Scan(&provisioningJobs); err != nil || provisioningJobs != 1 {
		t.Fatalf("queued purchase provisioning jobs = %d, err %v", provisioningJobs, err)
	}
	if second.CoreGrossTXBMinor != 1_000 {
		t.Fatalf("queued core gross = %d, want 1000", second.CoreGrossTXBMinor)
	}
	if !second.ValidFrom.Equal(first.ValidUntil) {
		t.Fatalf("renewal validFrom = %s, want prior validUntil %s", second.ValidFrom, first.ValidUntil)
	}
	if want := first.ValidUntil.AddDate(0, 0, 30); !second.ValidUntil.Equal(want) {
		t.Fatalf("renewal validUntil = %s, want %s", second.ValidUntil, want)
	}

	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "2400" {
		t.Fatalf("balance = %s, want 2400", balance.Minor)
	}
	entries, err := store.ListLedger(ctx, user.ID, 20)
	if err != nil {
		t.Fatalf("ListLedger(): %v", err)
	}
	if got := countLedgerKind(entries, "purchase_debit"); got != 2 {
		t.Fatalf("purchase debit count = %d, want 2", got)
	}
	var outboxCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_apply_entitlement'`).Scan(&outboxCount); err != nil {
		t.Fatalf("count entitlement outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("entitlement outbox count = %d, want 1 before renewal becomes due", outboxCount)
	}
}

func TestCreatePurchaseInsufficientBalanceIsAtomic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10002)
	combo := saveTestCombo(t, store, "too-expensive", 1_000, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 999, "almost-enough", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}

	_, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "purchase-insufficient"}, time.Now())
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("CreatePurchase() error = %v, want ErrInsufficientBalance", err)
	}

	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "999" {
		t.Fatalf("balance = %s, want 999", balance.Minor)
	}
	assertRowCount(t, store, "purchases", 0)
	assertRowCount(t, store, "outbox_jobs", 0)
	var purchaseDebits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='purchase_debit'`).Scan(&purchaseDebits); err != nil {
		t.Fatalf("count purchase debits: %v", err)
	}
	if purchaseDebits != 0 {
		t.Fatalf("purchase debit count = %d, want 0", purchaseDebits)
	}
}

func TestCreatePurchaseIdempotencyReplaysAndRejectsFingerprintReuse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10004)
	addon := saveTestSquad(t, store, "idempotent-addon", 300, true)
	combo := saveTestCombo(t, store, "idempotent-combo", 1_000, 30)
	otherCombo := saveTestCombo(t, store, "different-combo", 500, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "idempotency-seed", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, AddonSquadIDs: []string{" " + addon.ID + " ", addon.ID}, IdempotencyKey: " purchase-attempt "}

	const callers = 12
	start := make(chan struct{})
	results := make(chan model.Purchase, callers)
	errorsCh := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			purchase, err := store.CreatePurchase(ctx, input, time.Now())
			results <- purchase
			errorsCh <- err
		}()
	}
	close(start)
	var purchaseID string
	for range callers {
		purchase := <-results
		if err := <-errorsCh; err != nil {
			t.Fatalf("CreatePurchase(retry): %v", err)
		}
		if purchaseID == "" {
			purchaseID = purchase.ID
		} else if purchase.ID != purchaseID {
			t.Fatalf("replay purchase ID = %q, want %q", purchase.ID, purchaseID)
		}
	}
	conflict := input
	conflict.ComboID = otherCombo.ID
	if _, err := store.CreatePurchase(ctx, conflict, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("CreatePurchase(fingerprint conflict) error = %v, want ErrConflict", err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID}, time.Now()); err == nil {
		t.Fatal("CreatePurchase() accepted a missing idempotency key")
	}
	var purchases, debits, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE user_id=?`, user.ID).Scan(&purchases); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='purchase_debit'`, user.ID).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_apply_entitlement' AND payload=?`, `{"purchaseId":"`+purchaseID+`"}`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if purchases != 1 || debits != 1 || jobs != 1 {
		t.Fatalf("idempotent effects purchases=%d debits=%d jobs=%d, want 1/1/1", purchases, debits, jobs)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "3700" {
		t.Fatalf("idempotent balance = (%+v, %v), want 3700", balance, err)
	}
}
