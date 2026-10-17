package xnode_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNamedServicePreservesUsageAndScopesRouting(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	access := xnode.ServiceAccess{LimitBytes: 1000, Strategy: "NO_RESET", StartedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour), GroupIDs: []string{f.group.ID}}
	item, err := f.service.SaveNamedService(ctx, "", "routing-service", access)
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(item.Identity.ProxyUserID, 10)
	if item.Identity.UserID != "" || item.Identity.CurrentPeriodID == "" {
		t.Fatal("service created a member or lacks accounting")
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=400 WHERE id=?`, item.Identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	access.LimitBytes = 2000
	if _, err = f.service.SaveNamedService(ctx, id, "routing-service", access); err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, item.Identity.CurrentPeriodID)
	if err != nil || period.UsedBytes != 400 || period.LimitBytes != 2000 {
		t.Fatal("access edit reset usage", period, err)
	}
	custom, err := f.service.SaveOwnedOutbounds(ctx, "native."+id, f.node.ID, 0, json.RawMessage(`[{"tag":"exit","protocol":"socks","settings":{"servers":[{"address":"8.8.8.8","port":1080}]}}]`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.service.SaveOwnedRouting(ctx, "native."+id, f.node.ID, custom.Revision, json.RawMessage(`[{"type":"field","domain":["example.com"],"outboundTag":"exit"}]`))
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 1 {
		t.Fatal("standalone admission failed", compiled.Users, err)
	}
	access.Disabled = true
	if _, err = f.service.SaveNamedService(ctx, id, "routing-service", access); err != nil {
		t.Fatal(err)
	}
	compiled, err = f.service.Compile(ctx, f.node.ID)
	if err != nil || len(compiled.Users) != 0 {
		t.Fatal("disabled service remains authorized", err)
	}
	if _, err = f.service.OwnedCustomization(ctx, "native."+strconv.FormatInt(f.identity.ProxyUserID, 10), f.node.ID); err == nil {
		t.Fatal("member accessed through service scope")
	}
}
