package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type commerceBridgeRepository struct {
	xnode.Repository
	synced bool
}

func (*commerceBridgeRepository) XNodeIdentity(context.Context, int64) (xnode.Identity, error) {
	return xnode.Identity{ProxyUserID: 900, UserID: "member", LegacyUserID: "7", CurrentPeriodID: "period"}, nil
}
func (*commerceBridgeRepository) XNodePeriod(context.Context, string) (xnode.Period, error) {
	return xnode.Period{ID: "period", PurchaseID: "purchase", Status: "open", UsedBytes: 42}, nil
}
func (*commerceBridgeRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return &model.Purchase{ID: "purchase", UserID: "member"}, nil
}
func (*commerceBridgeRepository) XNodeLegacyUsage(context.Context, int64) (xnode.LegacyUsage, error) {
	return xnode.LegacyUsage{PeriodID: "period", UsedBytes: 10, LifetimeBytes: 10, SyncedAt: time.Now().Add(-time.Hour)}, nil
}
func (r *commerceBridgeRepository) SyncXNodeLegacyUsage(context.Context, xnode.LegacyUsage) error {
	r.synced = true
	return nil
}

func TestMixedProviderResetPrecheckDoesNotRequireFinancialFinalization(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "commerce.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	if err = store.SaveXNodeMigration(ctx, xnode.MigrationState{Phase: "active", SourceCipher: "fixture"}); err != nil {
		t.Fatal(err)
	}
	remote := &remnawave.User{ID: 7}
	remote.UserTraffic.UsedTrafficBytes = 10
	remote.UserTraffic.LifetimeUsedTrafficBytes = 10
	adapter := newRolloverStatsAdapter(t, &rolloverStatsClient{user: remote}, newNodeMultiplierCache())
	repo := &commerceBridgeRepository{}
	provider := proxyProvider{remnaAdapter: adapter, native: xnode.NewService(repo, nil, nil, nil, nil), store: store}
	state, err := provider.MemberOperationState(ctx, "900")
	if err != nil || state.UsedTrafficBytes != 42 || !repo.synced {
		t.Fatalf("reset precheck blocked or uncombined: %+v,%v,synced=%v", state, err, repo.synced)
	}
	if _, err = provider.MemberRefundState(ctx, "900", "purchase"); !errors.Is(err, xnode.ErrUnavailable) {
		t.Fatalf("unsettled refund accepted: %v", err)
	}
}
