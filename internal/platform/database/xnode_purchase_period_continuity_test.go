package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestRetainedPeriodRequiresExactPaidContinuityForExtendedProjection(t *testing.T) {
	t.Parallel()
	f, ctx := newPurchasePeriodRestoreFixture(t, "closed"), context.Background()
	next, err := f.store.CreatePurchase(ctx, PurchaseInput{UserID: f.identity.UserID, ComboID: f.purchase.ComboID, IdempotencyKey: "period-continuity"}, f.now.Add(time.Minute))
	if err != nil || next.Status != "queued" {
		t.Fatal(next, err)
	}
	projection, err := f.store.ContinuityEntitlement(ctx, next.ID, next.ValidFrom)
	if err != nil || projection == nil {
		t.Fatal(projection, err)
	}
	if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, *projection, 0, next.ValidFrom); err != nil {
		t.Fatal("paid boundary continuity lost", err)
	}
	period, err := f.store.XNodePeriod(ctx, f.period.ID)
	if err != nil || period.Status != "open" || period.UsedBytes != 77 || !period.StartedAt.Equal(f.period.StartedAt) {
		t.Fatal("continuity reset retained accounting", period, err)
	}
	purchase, err := f.store.PurchaseByID(ctx, f.purchase.ID)
	if err != nil || !purchase.ValidUntil.Equal(f.purchase.ValidUntil) {
		t.Fatal("continuity rewrote original paid term", purchase, err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=$1`, next.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, period, *projection, 0, next.ValidFrom); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("cancelled lineage allowed extension", err)
	}
}
