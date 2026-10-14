package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

const testRolloverMultiplierFP int64 = 1_000_000

func TestRolloverUsageSnapshotAccountsForMoreThanTwentyNodes(t *testing.T) {
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	series := make([]remnawave.NodeUsageSeries, 0, 21)
	cache := newNodeMultiplierCache()
	for index := 0; index < 21; index++ {
		uuid := "node-" + string(rune('a'+index))
		series = append(series, remnawave.NodeUsageSeries{UUID: uuid, Data: []int64{1}})
		cache.set(uuid, testRolloverMultiplierFP)
	}
	client := &rolloverStatsClient{stats: &remnawave.UserStats{
		Categories: []string{start.Format(time.DateOnly)}, SparklineData: []int64{21}, Series: series,
	}}
	adapter := newRolloverStatsAdapter(t, client, cache)

	snapshot, err := adapter.UsageSnapshotForRollover(context.Background(), "7", start, start)
	if err != nil {
		t.Fatalf("UsageSnapshotForRollover(): %v", err)
	}
	if client.limit != rolloverStatsTopNodesLimit || snapshot.WeightedUsedBytes != 21 {
		t.Fatalf("snapshot = %+v, topNodesLimit=%d", snapshot, client.limit)
	}
}

func TestRolloverUsageSnapshotRejectsMalformedStatistics(t *testing.T) {
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	valid := rolloverStatistics(start)
	tests := []struct {
		name  string
		user  *remnawave.User
		stats *remnawave.UserStats
	}{
		{name: "aggregate mismatch", stats: &remnawave.UserStats{Categories: valid.Categories, SparklineData: []int64{6}, Series: valid.Series}},
		{name: "duplicate category", stats: &remnawave.UserStats{Categories: []string{valid.Categories[0], valid.Categories[0]}, SparklineData: []int64{5, 0}, Series: []remnawave.NodeUsageSeries{{UUID: "node-a", Data: []int64{5, 0}}}}},
		{name: "negative bucket", stats: &remnawave.UserStats{Categories: valid.Categories, SparklineData: []int64{0}, Series: []remnawave.NodeUsageSeries{{UUID: "node-a", Data: []int64{-1}}}}},
		{name: "invalid strategy", user: &remnawave.User{TrafficLimitStrategy: "UNKNOWN"}, stats: valid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := newNodeMultiplierCache()
			cache.set("node-a", testRolloverMultiplierFP)
			client := &rolloverStatsClient{user: test.user, stats: test.stats}
			adapter := newRolloverStatsAdapter(t, client, cache)
			_, err := adapter.UsageSnapshotForRollover(context.Background(), "7", start, start)
			if !errors.Is(err, rollover.ErrPerNodeUsageUnavailable) {
				t.Fatalf("UsageSnapshotForRollover() error = %v", err)
			}
		})
	}
}

func TestRolloverUsageSnapshotRejectsZeroMultiplier(t *testing.T) {
	start := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	cache := newNodeMultiplierCache()
	cache.set("node-a", 0)
	adapter := newRolloverStatsAdapter(t, &rolloverStatsClient{stats: rolloverStatistics(start)}, cache)

	_, err := adapter.UsageSnapshotForRollover(context.Background(), "7", start, start)
	if !errors.Is(err, rollover.ErrPerNodeUsageUnavailable) {
		t.Fatalf("UsageSnapshotForRollover() error = %v", err)
	}
}

func rolloverStatistics(day time.Time) *remnawave.UserStats {
	return &remnawave.UserStats{Categories: []string{day.Format(time.DateOnly)}, SparklineData: []int64{5},
		Series: []remnawave.NodeUsageSeries{{UUID: "node-a", Data: []int64{5}}}}
}

func newRolloverStatsAdapter(t *testing.T, client *rolloverStatsClient, cache *nodeMultiplierCache) remnaAdapter {
	t.Helper()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "remna-rollover-validation", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Shutdown(context.Background()) })
	return remnaAdapter{queue: queue, multipliers: cache, clientFactory: func(context.Context) (remnaClient, error) { return client, nil }}
}
