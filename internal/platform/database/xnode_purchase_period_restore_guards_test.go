package database

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurchasePeriodRestoreRespectsFinancialGateButAllowsAdminSync(t *testing.T) {
	for _, kind := range []string{"purchase_refund", "purchase_traffic_reset", providerops.KindAdminEntitlementEdit, providerops.KindAdminBulkExtension} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			f, ctx := newPurchasePeriodRestoreFixture(t, "closed"), context.Background()
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, f.identity.UserID); err != nil {
				t.Fatal(err)
			}
			_, _, err := f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.identity.UserID, OwnerUserID: f.identity.UserID, Kind: kind, IdempotencyKey: "retained-gate", RequestFingerprint: "retained-period-operation-gate",
				Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: f.purchase.ID}}}, f.now)
			if err != nil {
				t.Fatal(err)
			}
			err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, f.purchase, 0, f.now)
			blocked := kind == "purchase_refund" || kind == "purchase_traffic_reset"
			if blocked && !errors.Is(err, xnode.ErrAccessOperationPending) || !blocked && err != nil {
				t.Fatal("wrong operation gate", err)
			}
			period, readErr := f.store.XNodePeriod(ctx, f.period.ID)
			if readErr != nil || period.UsedBytes != 77 || (period.Status == "open") == blocked {
				t.Fatal(period, readErr)
			}
		})
	}
}

func TestPurchasePeriodRestoreClearsLiveProofAndRetainsRetiredDrain(t *testing.T) {
	t.Parallel()
	f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
	if err := f.store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "restore-config", Name: "Restore proof", DocumentCipher: "fixture"}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"live", "retired"} {
		if err := f.store.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: node, ConfigID: "restore-config", SettingsCipher: "fixture", Enabled: true, MultiplierFP: xnode.MultiplierScale}); err != nil {
			t.Fatal(err)
		}
		if err := f.store.TrackXNodePeriodNode(ctx, f.period.ID, node); err != nil {
			t.Fatal(err)
		}
	}
	period, err := f.store.BeginXNodePeriodClosure(ctx, f.period, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=7,flushed_stream_id='retained-stream',flushed_legacy_sequence=4 WHERE period_id=$1`, period.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,retired_at=$1 WHERE id='retired'`, stamp(f.now)); err != nil {
		t.Fatal(err)
	}
	if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, period, f.purchase, 0, f.now); err != nil {
		t.Fatal(err)
	}
	var liveCleared, retiredKept bool
	if err = f.store.DB().QueryRowContext(ctx, `SELECT flushed_sequence IS NULL AND flushed_stream_id='' AND flushed_legacy_sequence=0 FROM xnode_period_nodes WHERE period_id=$1 AND node_id='live'`, period.ID).Scan(&liveCleared); err != nil {
		t.Fatal(err)
	}
	if err = f.store.DB().QueryRowContext(ctx, `SELECT flushed_sequence=7 AND flushed_stream_id='retained-stream' AND flushed_legacy_sequence=4 FROM xnode_period_nodes WHERE period_id=$1 AND node_id='retired'`, period.ID).Scan(&retiredKept); err != nil {
		t.Fatal(err)
	}
	if !liveCleared || !retiredKept {
		t.Fatal("restore confused live admission with retained retirement proof", liveCleared, retiredKept)
	}
	if err = f.store.FinalizeXNodePeriod(ctx, period); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("old closure finalized reopened admission", err)
	}
}

func TestPurchasePeriodRestoreHonorsPersistedBanBeforeProviderRestriction(t *testing.T) {
	t.Parallel()
	f, ctx := newPurchasePeriodRestoreFixture(t, "closed"), context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, f.identity.UserID); err != nil {
		t.Fatal(err)
	}
	_, err := f.store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: f.identity.UserID, UserID: f.identity.UserID,
		IdempotencyKey: "restore-ban", RequestFingerprint: "restore-ban", DurationMinutes: 10, Reason: "fixture"}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, f.purchase, 0, f.now); !errors.Is(err, xnode.ErrAccessOperationPending) {
		t.Fatal("queued ban was bypassed by retained restoration", err)
	}
	period, err := f.store.XNodePeriod(ctx, f.period.ID)
	if err != nil || period.Status != "closed" || period.UsedBytes != 77 {
		t.Fatal(period, err)
	}
}
