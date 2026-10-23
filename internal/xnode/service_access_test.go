package xnode_test

import (
	"context"

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
	saved, err := f.service.SaveNamedService(ctx, "", "standalone", access)
	if err != nil {
		t.Fatal(err)
	}
	identity := saved.Identity
	if identity.ProxyUserID < 1<<52 || identity.ProxyUserID >= 1<<53 {
		t.Fatal("invalid service namespace", identity.ProxyUserID)
	}
	if _, err = f.store.DB().ExecContext(ctx, `INSERT INTO xnode_imported_usage(proxy_user_id,period_id,used_bytes,lifetime_bytes,synced_at) VALUES(?,?,?,?,?)`, identity.ProxyUserID, identity.CurrentPeriodID, used, 10000, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=?`, used, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	return f, identity, access
}

func TestStandaloneNativeTrafficExhaustsTheSharedAllowance(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "NO_RESET")
	ctx := context.Background()
	if err := f.store.BindXNodeAgent(ctx, f.node.ID, "standalone-agent"); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	report := wire.TrafficReport{ID: "standalone-agent:1", Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: identity.CurrentPeriodID, From: at, Through: at, UploadBytes: 900}}}
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

func TestStandaloneCadencePreservesHistoricalBaseline(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "MONTH_ROLLING")
	ctx := context.Background()
	var current xnode.Identity
	var err error
	if err = f.service.ResetCadences(ctx); err != nil {
		t.Fatal(err)
	}
	current, err = f.store.XNodeIdentity(ctx, identity.ProxyUserID)
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
