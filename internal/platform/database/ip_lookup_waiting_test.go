package database

import (
	"context"
	"testing"
	"time"
)

func TestIPLookupQueuedWaitSurvivesReceiptRetention(t *testing.T) {
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	op := submitIP(t, service, user, "long-wait", false)
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET created_at=? WHERE id=?`, stamp(now.Add(-48*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneProviderOperationsTx(ctx, tx, now.Add(-24*time.Hour), now, map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.Operation.Status != "queued" || check.Refunded || check.Progress.Phase != "queued" {
		t.Fatalf("queued report expired or attempted: %+v %v", check, err)
	}
	if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "admitted", now); err != nil || !claimed {
		t.Fatalf("long-wait report not recoverable: %v %v", claimed, err)
	}
}
