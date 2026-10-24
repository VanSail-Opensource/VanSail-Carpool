package datamigrate

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func seedAccounting(t *testing.T, ctx context.Context, store *database.Store, vault *secret.Vault, user string, at time.Time) {
	t.Helper()
	seal := func(context, value string) string {
		cipher, err := vault.Encrypt(context, value)
		if err != nil {
			t.Fatal(err)
		}
		return cipher
	}
	if err := store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "recovery", Name: "Recovery", DocumentCipher: seal("xnode.config.recovery", `{"inbounds":[]}`)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveXNodeNode(ctx, xnode.Node{ID: "recovery-node", Name: "Recovery", ConfigID: "recovery", Enabled: true, MultiplierFP: 1250000, SettingsCipher: seal("xnode.node.recovery-node", `{}`)}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindXNodeAgent(ctx, "recovery-node", "recovery-agent"); err != nil {
		t.Fatal(err)
	}
	identity, err := store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user, CredentialsCipher: seal("xnode.identity.user."+user, `{"uuid":"recovery"}`), SubscriptionCipher: seal("xnode.subscription.user."+user, "recovery-token")})
	if err != nil {
		t.Fatal(err)
	}
	period := xnode.Period{ID: "recovery-period", ProxyUserID: identity.ProxyUserID, StartedAt: at, LimitBytes: 100000, Strategy: "NO_RESET", Status: "open"}
	if err = store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	report := wire.TrafficReport{ID: "recovery-agent:1", Traffic: wire.Traffic{int(identity.ProxyUserID): {1, 2}}, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1, DownloadBytes: 2}}}
	if err = store.RecordXNodeTraffic(ctx, "recovery-node", "recovery-agent", 1, "recovery-digest", report); err != nil {
		t.Fatal(err)
	}
	value, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 3 {
		t.Fatalf("accounting fixture: %+v %v", value, err)
	}
}
