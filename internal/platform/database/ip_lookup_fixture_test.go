package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func ipLookupFixture(t *testing.T, quota int) (*Store, *iplookup.Service, string, string, time.Time) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 96001)
	now := time.Now().UTC()
	combo := saveTestCombo(t, store, "IP check combo", 100, 30)
	c := iplookup.DefaultConfig()
	c.Enabled = true
	c.LookupFeeTXB = "2.50"
	c.RefreshFeeTXB = "3.50"
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "ipapi"
	}
	if err := store.SaveIPLookupSettings(ctx, user.ID, c, map[string]string{"ipapi": "test-vault-value"}, map[string]*int{combo.ID: &quota}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 10000, "ip-seed:"+user.ID, "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "ip-purchase"}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	return store, iplookup.NewService(store, []byte(strings.Repeat("k", 32))), user.ID, purchase.ID, now
}

func submitIP(t *testing.T, service *iplookup.Service, user, key string, refresh bool) model.OperationReceipt {
	t.Helper()
	ctx := context.Background()
	q, err := service.Quote(ctx, user, "8.8.8.8", refresh)
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.Submit(ctx, user, key, q)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func finishIP(t *testing.T, store *Store, operationID string, failed bool) iplookup.Report {
	t.Helper()
	ctx := context.Background()
	r, _, err := store.IPLookupRun(ctx, operationID)
	if err != nil {
		t.Fatal(err)
	}
	r.Status, r.Verdict = "succeeded", "suitable"
	if failed {
		r.Status, r.Verdict = "failed", "inconclusive"
	}
	r.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	stage, claimed, err := store.ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		details := iplookup.LiveDetails{IP: r.IP, FetchedAt: r.CheckedAt, Topology: iplookup.BGPTopology{LiveMeta: iplookup.LiveMeta{Status: "empty"}}}
		stage.SnapshotID, err = store.SaveIPDetails(ctx, r.ID, details, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		stage.Status = "succeeded"
		if err := store.FinishIPLookupLiveStage(ctx, r.ID, stage); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.FinishIPLookupRun(ctx, r, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return r
}
