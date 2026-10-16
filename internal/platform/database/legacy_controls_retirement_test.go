package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRetiredLegacyControlsCannotDispatchAfterUpgrade(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 88119)
	if _, err := store.db.ExecContext(ctx, `ALTER TABLE squad_product_overrides ADD COLUMN geocheck_disabled INTEGER NOT NULL DEFAULT 0`); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, kind := range []string{"admin_remna_relink", "host_remark_update"} {
		op, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: user.ID, Kind: kind, IdempotencyKey: kind, RequestFingerprint: "retired-controls-fixture", Items: []providerops.ItemInput{{Key: "target", TargetType: "legacy", TargetID: "target"}}}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, op.Receipt.ID)
	}
	raw, err := os.ReadFile("migrations/079_retire_legacy_controls.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		op, err := store.ProviderOperationByID(ctx, id)
		if err != nil || op.Receipt.Status != "failed" || op.Receipt.ErrorCode == nil || *op.Receipt.ErrorCode != "FEATURE_RETIRED" {
			t.Fatalf("retired operation still executable: %+v,%v", op, err)
		}
	}
	var pending int
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation' AND status IN ('pending','processing')`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("retired queue work remains: %d,%v", pending, err)
	}
}
