package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestMalformedAffiliateTiersFailClosedAndRemainRepairable(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	actor, inviter := createTestAdmin(t, store, 711), createTestUser(t, store, 712)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 713, inviter.TelegramID, now); err != nil || !accepted {
		t.Fatal(accepted, err)
	}
	invitee, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 713, FirstName: "Invitee"}, false)
	if err != nil {
		t.Fatal(err)
	}
	order := createTestPaymentOrder(t, store, invitee.ID, "ezpay", 1000, now)
	if _, err := store.DB().ExecContext(ctx, `UPDATE affiliate_tiers SET enabled=0`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AffiliateOverview(ctx, inviter.ID, ""); !errors.Is(err, affiliates.ErrInvalidInput) {
		t.Fatal("malformed tiers were presented as a valid overview", err)
	}
	if _, changed, err := store.SettlePayment(ctx, "ezpay", "recover-affiliate", "hash", order.ID, "trade-affiliate", "", now); !errors.Is(err, affiliates.ErrInvalidInput) || changed {
		t.Fatal("malformed tiers partially settled a payment", changed, err)
	}
	if balance := adminWorkflowBalance(t, store, invitee.ID); balance != 0 {
		t.Fatal("rejected settlement credited invitee", balance)
	}
	// The admin configuration read remains available so malformed old terms can
	// be replaced through the normal revision-checked repair path.
	current, err := store.AffiliateConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveAffiliateConfig(ctx, actor.ID, affiliates.ConfigInput{ExpectedVersion: current.Version, Tiers: []affiliates.Tier{
		{Name: "Recovered starter", Threshold: 0, Enabled: true, CommissionEnabled: true, CommissionBPS: 1000, Reward: affiliates.Reward{Kind: "none"}},
	}}, now); err != nil {
		t.Fatal("normal administrator repair failed", err)
	}
	if _, changed, err := store.SettlePayment(ctx, "ezpay", "recover-affiliate", "hash", order.ID, "trade-affiliate", "", now); err != nil || !changed {
		t.Fatal("original payment could not settle after repair", changed, err)
	}
	if balance := adminWorkflowBalance(t, store, inviter.ID); balance != 100 {
		t.Fatal("recovered commission differs from exact accepted amount", balance)
	}
	// A repeated original settlement must leave a usable transaction. Catching
	// a PostgreSQL unique violation alone leaves that transaction aborted.
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := settleAffiliateTx(ctx, tx, order.ID, invitee.ID, 1000, now); err != nil {
		t.Fatal("original affiliate receipt did not replay", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal("affiliate replay left an aborted transaction", err)
	}
	if balance := adminWorkflowBalance(t, store, inviter.ID); balance != 100 {
		t.Fatal("affiliate replay duplicated commission", balance)
	}
}
