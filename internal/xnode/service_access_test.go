package xnode_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func standaloneFixture(t *testing.T, limit, used int64, strategy string) (fixture, xnode.Identity, xnode.ServiceAccess) {
	t.Helper()
	f := controlFixture(t, false)
	ctx := context.Background()
	access := xnode.ServiceAccess{LimitBytes: limit, Strategy: strategy, StartedAt: time.Now().UTC().Add(-31 * 24 * time.Hour), ExpiresAt: time.Now().UTC().AddDate(1, 0, 0), GroupIDs: []string{f.group.ID}}
	identity, err := f.service.ImportServiceIdentity(ctx, "standalone", 376, "00000000-0000-4000-8000-000000000376", "existing-trojan", "existing-chacha", access)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.SaveXNodeMigration(ctx, xnode.MigrationState{Phase: "active"}); err != nil {
		t.Fatal(err)
	}
	if err = f.service.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: used, LifetimeBytes: 10000, SyncedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	identity, err = f.store.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	return f, identity, access
}

func TestStandaloneImportPreservesQuotaCredentialsAndPrivateSubscription(t *testing.T) {
	f, identity, access := standaloneFixture(t, 1000, 100, "MONTH_ROLLING")
	ctx := context.Background()
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Standalone host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	metadata, err := f.service.ServiceSubscription(ctx, "376")
	if err != nil || !metadata.Active || metadata.LimitBytes != "1000" || metadata.UsedBytes != "100" || metadata.ExpiresAt == nil || !metadata.ExpiresAt.Equal(access.ExpiresAt) {
		t.Fatalf("standalone metadata: active=%v, used=%s, limit=%s, error=%v", metadata.Active, metadata.UsedBytes, metadata.LimitBytes, err)
	}
	token := metadata.URL[strings.LastIndex(metadata.URL, "/")+1:]
	raw, _, err := f.service.BearerSubscription(ctx, token, "xray")
	if err != nil || !strings.Contains(string(raw), "00000000-0000-4000-8000-000000000376") {
		t.Fatalf("compatible credential missing from subscription: %v", err)
	}
	resumed, err := f.service.ImportServiceIdentity(ctx, "standalone", 376, "00000000-0000-4000-8000-000000000376", "existing-trojan", "existing-chacha", access)
	if err != nil || resumed.CredentialsCipher != identity.CredentialsCipher || resumed.SubscriptionCipher != identity.SubscriptionCipher || resumed.CurrentPeriodID != identity.CurrentPeriodID {
		t.Fatalf("resume changed identity or accounting: %v", err)
	}
	access.LimitBytes++
	if _, err = f.service.ImportServiceIdentity(ctx, "standalone", 376, "00000000-0000-4000-8000-000000000376", "existing-trojan", "existing-chacha", access); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("resume silently changed quota: %v", err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil || period.PurchaseID != "" || period.UsedBytes != 100 || period.LimitBytes != 1000 {
		t.Fatalf("standalone quota or purchase ownership changed: %+v, %v", period, err)
	}
}

func TestStandaloneNativeTrafficExhaustsTheSharedAllowance(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "NO_RESET")
	ctx := context.Background()
	if err := f.store.BindXNodeAgent(ctx, f.node.ID, "standalone-agent"); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: "standalone-agent:1", Samples: []wire.TrafficSample{{UserID: 376, AccountingPeriod: identity.CurrentPeriodID, From: at, Through: at, UploadBytes: 900}}}
	if err := f.store.RecordXNodeTraffic(ctx, f.node.ID, "standalone-agent", 1, "standalone-digest", report); err != nil {
		t.Fatal(err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 0 {
		t.Fatalf("exhausted identity remains authorized: %d, %v", len(compiled.Users), err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil || period.UsedBytes != 1000 {
		t.Fatalf("combined allowance was not exact: %+v, %v", period, err)
	}
}

func TestStandaloneCadenceWaitsForFinalSourceAccounting(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "MONTH_ROLLING")
	ctx := context.Background()
	if err := f.service.ResetCadences(ctx); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.XNodeIdentity(ctx, 376)
	if err != nil || current.CurrentPeriodID != identity.CurrentPeriodID {
		t.Fatal("native reset advanced ahead of the active source")
	}
	if err = f.store.SaveXNodeMigration(ctx, xnode.MigrationState{Phase: "complete"}); err != nil {
		t.Fatal(err)
	}
	if err = f.service.ResetCadences(ctx); err != nil {
		t.Fatal(err)
	}
	current, err = f.store.XNodeIdentity(ctx, 376)
	if err != nil || current.CurrentPeriodID == identity.CurrentPeriodID {
		t.Fatal("rolling standalone allowance did not reset after final accounting")
	}
	period, err := f.store.XNodePeriod(ctx, current.CurrentPeriodID)
	if err != nil || period.UsedBytes != 0 || period.LimitBytes != 1000 || period.Strategy != "MONTH_ROLLING" {
		t.Fatalf("incorrect successor period: %+v, %v", period, err)
	}
	old, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil || old.UsedBytes != 100 || old.Status != "closed" {
		t.Fatalf("reset erased previous usage: %+v, %v", old, err)
	}
}

func TestStandaloneSourceResetRetainsLateTailAndRejectsStaleImports(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "MONTH_ROLLING")
	ctx := context.Background()
	at := time.Now().UTC().Add(time.Second)
	if err := f.service.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: 7, LifetimeBytes: 10020, LastResetAt: &at, SyncedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := f.service.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: 99, LifetimeBytes: 9999, SyncedAt: at.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	current, err := f.store.XNodeIdentity(ctx, 376)
	if err != nil || current.CurrentPeriodID == identity.CurrentPeriodID {
		t.Fatal("source reset failed to open a standalone period")
	}
	period, err := f.store.XNodePeriod(ctx, current.CurrentPeriodID)
	if err != nil || period.UsedBytes != 7 {
		t.Fatalf("new reset counter changed: %+v, %v", period, err)
	}
	old, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil || old.UsedBytes != 113 || old.Status != "closing" {
		t.Fatalf("late source tail lost: %+v, %v", old, err)
	}
}
