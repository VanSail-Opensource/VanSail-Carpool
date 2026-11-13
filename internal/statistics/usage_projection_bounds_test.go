package statistics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

func TestUsageProjectionOverflowPreservesLastGoodRemotePartition(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	repository := &statisticsRepositoryStub{members: []model.StatisticsUsageMember{{
		Purchase:     model.Purchase{ID: "purchase", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
		RemoteUserID: "native-owner",
	}}}
	provider := &statisticsProviderStub{digest: Digest{CreatedUsers: 9}, snapshots: map[string]usageaccounting.UsageSnapshot{
		"native-owner": {LimitBytes: 1, Strategy: "NO_RESET", WeightedUsedBytes: math.MaxInt64, NodeSeriesAvailable: true},
	}}
	service := NewService(repository, provider)
	previous := now.Add(-time.Minute)
	service.snapshot.RemoteGeneratedAt = previous
	service.snapshot.Remote.WeeklyUserIncrease = 4
	if err := service.Refresh(context.Background(), now); err == nil {
		t.Fatal("unrepresentable average was published")
	}
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || snapshot.Remote.WeeklyUserIncrease != 4 || !snapshot.RemoteGeneratedAt.Equal(previous) || len(snapshot.StalePartitions) != 1 || snapshot.StalePartitions[0] != remotePartition || !snapshot.DatabaseGeneratedAt.Equal(now) {
		t.Fatalf("last-good partition was not preserved: %+v, %v", snapshot, err)
	}
}

func TestUsageProjectionPreservesRepresentableOverage(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	repository := &statisticsRepositoryStub{members: []model.StatisticsUsageMember{{
		Purchase:     model.Purchase{ID: "purchase", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)},
		RemoteUserID: "native-owner",
	}}}
	provider := &statisticsProviderStub{snapshots: map[string]usageaccounting.UsageSnapshot{
		"native-owner": {LimitBytes: 1000, Strategy: "NO_RESET", WeightedUsedBytes: 2000, NodeSeriesAvailable: true},
	}}
	value, err := NewService(repository, provider).refreshRemote(context.Background(), now)
	if err != nil || value.MonthlyAverageUsageBPS != 20000 || value.MonthlyAverageUsage != 200 {
		t.Fatalf("representable overage changed: %+v, %v", value, err)
	}
}
