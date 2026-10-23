package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestLuckyDrawRequiresWorstCaseCoverageAndReplays(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_200)
	if _, err := store.AdjustBalance(ctx, user.ID, 349, "draw-seed", "seed", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Exact coverage", Kind: "instant", Enabled: true,
		FeeMinor: 100, ExpectedParticipation: 1,
		Prizes: []activity.PrizeInput{{Name: "Minus", ProbabilityBPS: 10_000, Reward: activity.Reward{Kind: activity.RewardTXBDelta, TXBDeltaMinor: -250}}}}, time.Now())
	if err != nil {
		t.Fatalf("SaveLuckyDraw(): %v", err)
	}
	if _, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "draw-key", fixedActivityRandom{}, time.Now()); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("PlayLuckyDraw(under-covered) = %v, want insufficient balance", err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 1, "draw-coverage", "coverage", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(coverage): %v", err)
	}
	result, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "draw-key", fixedActivityRandom{}, time.Now())
	if err != nil || result.BalanceAfterMinor != 0 {
		t.Fatalf("PlayLuckyDraw() = (%+v, %v), want zero balance", result, err)
	}
	replayed, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "draw-key", fixedActivityRandom{}, time.Now())
	if err != nil || !replayed.Replayed || replayed.ID != result.ID {
		t.Fatalf("PlayLuckyDraw(replay) = (%+v, %v)", replayed, err)
	}
	var results int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_draw_results`).Scan(&results); err != nil || results != 1 {
		t.Fatalf("draw result count = %d, %v, want 1", results, err)
	}
}

func TestActivityDescriptionsPersistAndEnterResultSnapshots(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_300)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 1_000, "activity-description-seed", "seed", now); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	game, err := store.SaveActivityGame(ctx, activity.GameInput{Name: "Coin", Icon: "coin", Description: "  Clear game terms  ", Enabled: true,
		WinChanceBPS: 10_000, MinimumStakeMinor: 100, MaximumStakeMinor: 100, ReturnMultiplierBPS: 10_000}, now)
	if err != nil {
		t.Fatalf("SaveActivityGame(): %v", err)
	}
	if game.Description != "Clear game terms" {
		t.Fatalf("game description = %q", game.Description)
	}
	bet, err := store.PlaceActivityBet(ctx, user.ID, game.ID, 100, "description-bet", fixedActivityRandom{}, now)
	if err != nil {
		t.Fatalf("PlaceActivityBet(): %v", err)
	}
	if !strings.Contains(bet.ConfigurationSnapshot, `"description":"Clear game terms"`) {
		t.Fatalf("game snapshot omits description: %s", bet.ConfigurationSnapshot)
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Draw", Description: "  Clear draw terms  ", Kind: "instant",
		Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
		Prizes: []activity.PrizeInput{{Name: "Nothing", ProbabilityBPS: 10_000, Reward: activity.Reward{Kind: activity.RewardNone}}}}, now)
	if err != nil {
		t.Fatalf("SaveLuckyDraw(): %v", err)
	}
	if draw.Description != "Clear draw terms" {
		t.Fatalf("draw description = %q", draw.Description)
	}
	result, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "description-draw", fixedActivityRandom{}, now)
	if err != nil {
		t.Fatalf("PlayLuckyDraw(): %v", err)
	}
	if !strings.Contains(result.ConfigurationSnapshot, `"description":"Clear draw terms"`) {
		t.Fatalf("draw snapshot omits description: %s", result.ConfigurationSnapshot)
	}
}

func TestStoredExtensionAppliesToExactNextActivationAndDelaysQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_400)
	base := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 10_000, "extension-seed", "seed", base); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Extension term", 100, 1)
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "extension-first"}, base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "extension-second"}, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE id=?`, first.ID); err != nil {
		t.Fatal(err)
	}
	// Draw extension rewards now require a live term, so stored credits arrive via affiliate awards.
	creditTx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySubscriptionExtensionTx(ctx, creditTx, user.ID, 3, "affiliate_tier", "extension-credit", base.Add(time.Minute)); err != nil {
		_ = creditTx.Rollback()
		t.Fatal(err)
	}
	if err := creditTx.Commit(); err != nil {
		t.Fatal(err)
	}
	third, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "extension-third"}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var consumedBefore int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_extension_credits WHERE consumed_at IS NOT NULL`).Scan(&consumedBefore); err != nil || consumedBefore != 0 {
		t.Fatalf("credits consumed by later queued purchase = %d, %v", consumedBefore, err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, second.ValidFrom); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(): %v", err)
	}
	activated, err := store.PurchaseByID(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	delayed, err := store.PurchaseByID(ctx, third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activated.Status != "activating" || !activated.ValidUntil.Equal(second.ValidUntil.AddDate(0, 0, 3)) {
		t.Fatalf("activated term = status %q until %s, want activating/%s", activated.Status, activated.ValidUntil, second.ValidUntil.AddDate(0, 0, 3))
	}
	if !delayed.ValidFrom.Equal(third.ValidFrom.AddDate(0, 0, 3)) || !delayed.ValidUntil.Equal(third.ValidUntil.AddDate(0, 0, 3)) {
		t.Fatalf("delayed term = %s..%s, want %s..%s", delayed.ValidFrom, delayed.ValidUntil, third.ValidFrom.AddDate(0, 0, 3), third.ValidUntil.AddDate(0, 0, 3))
	}
	var consumedBy string
	if err := store.DB().QueryRowContext(ctx, `SELECT consumed_by_purchase_id FROM activity_extension_credits`).Scan(&consumedBy); err != nil || consumedBy != second.ID {
		t.Fatalf("extension consumed by %q, %v, want %q", consumedBy, err, second.ID)
	}
}

func TestActiveExtensionQueuesProxySync(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_401)
	base := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 10_000, "active-extension-seed", "seed", base); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Active extension term", 100, 30)
	active, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "active-extension-first"}, base)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "active-extension-second"}, base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Active extension", Kind: "instant", Enabled: true,
		FeeMinor: 100, ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{
			Name: "Three days", ProbabilityBPS: 10_000, Reward: activity.Reward{Kind: activity.RewardSubscriptionExtension, ExtensionDays: 3},
		}}}, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "active-extension-draw", fixedActivityRandom{}, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	updatedActive, err := store.PurchaseByID(ctx, active.ID)
	if err != nil {
		t.Fatal(err)
	}
	updatedQueued, err := store.PurchaseByID(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !updatedActive.ValidUntil.Equal(active.ValidUntil.AddDate(0, 0, 3)) {
		t.Fatalf("active expiry = %s, want %s", updatedActive.ValidUntil, active.ValidUntil.AddDate(0, 0, 3))
	}
	if !updatedQueued.ValidFrom.Equal(queued.ValidFrom.AddDate(0, 0, 3)) || !updatedQueued.ValidUntil.Equal(queued.ValidUntil.AddDate(0, 0, 3)) {
		t.Fatalf("queued dates = %s..%s, want %s..%s", updatedQueued.ValidFrom, updatedQueued.ValidUntil, queued.ValidFrom.AddDate(0, 0, 3), queued.ValidUntil.AddDate(0, 0, 3))
	}
	var syncJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload=?`, `{"userId":"`+user.ID+`"}`).Scan(&syncJobs); err != nil {
		t.Fatal(err)
	}
	if syncJobs != 1 {
		t.Fatalf("Proxy sync jobs = %d, want 1", syncJobs)
	}
}
