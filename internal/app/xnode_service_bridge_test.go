package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestStandaloneBridgeSharesOneCombinedAllowance(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		limit, combined, source, expected int64
		blocked                           bool
	}{
		{"partly used", 1000, 990, 90, 100, false},
		{"exhausted", 1000, 1000, 90, 0, true},
		{"unlimited", 0, 10000, 90, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := database.Open(ctx, filepath.Join(t.TempDir(), "standalone.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			store := database.NewStore(db)
			vault, err := secret.NewVault(make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			native := xnode.NewService(store, nil, vault, nil, nil)
			access := xnode.ServiceAccess{LimitBytes: tc.limit, Strategy: "MONTH_ROLLING", StartedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour), GroupIDs: []string{"existing-group"}}
			identity, err := native.ImportServiceIdentity(ctx, "standalone", 929, "00000000-0000-4000-8000-000000000929", "trojan", "chacha", access)
			if err != nil {
				t.Fatal(err)
			}
			if err = native.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: tc.combined, LifetimeBytes: tc.combined, SyncedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			identity, err = store.XNodeIdentity(ctx, identity.ProxyUserID)
			if err != nil {
				t.Fatal(err)
			}
			policy, err := (proxyProvider{store: store, native: native}).bridgeServicePolicy(ctx, identity, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if tc.blocked {
				if policy != nil {
					t.Fatal("exhausted source was reauthorized")
				}
				return
			}
			if policy == nil || policy.ID != "" || policy.UserID != "" || policy.TrafficLimitBytes != tc.expected || policy.ResetStrategy != access.Strategy || !policy.ValidUntil.Equal(access.ExpiresAt) {
				t.Fatal("source received another full quota or a synthetic purchase identity")
			}
		})
	}
}
