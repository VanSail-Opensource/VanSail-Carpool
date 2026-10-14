package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type rolloverStatsClient struct {
	remnaClient
	start, end time.Time
	limit      int
	user       *remnawave.User
	stats      *remnawave.UserStats
}

func (c *rolloverStatsClient) GetUserByID(context.Context, int64) (*remnawave.User, error) {
	if c.user != nil {
		return c.user, nil
	}
	return &remnawave.User{TrafficLimitStrategy: remnawave.TrafficNoReset}, nil
}

func (c *rolloverStatsClient) GetUserStats(_ context.Context, _ int64, start, end time.Time, limit int) (*remnawave.UserStats, error) {
	if start.After(end) {
		return nil, errors.New("stats range is invalid")
	}
	c.start, c.end, c.limit = start, end, limit
	if c.stats != nil {
		return c.stats, nil
	}
	return &remnawave.UserStats{Categories: []string{}, SparklineData: []int64{}, Series: []remnawave.NodeUsageSeries{}}, nil
}

func TestRolloverUsageSnapshotKeepsInclusiveFinalDate(t *testing.T) {
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "remna-rollover-stats-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = queue.Shutdown(context.Background()) }()

	client := &rolloverStatsClient{}
	adapter := remnaAdapter{queue: queue, clientFactory: func(context.Context) (remnaClient, error) { return client, nil }}
	moment := time.Date(2026, 8, 19, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	snapshot, err := adapter.UsageSnapshotForRollover(context.Background(), "7", moment, moment)
	if err != nil {
		t.Fatalf("UsageSnapshotForRollover(): %v", err)
	}
	if !snapshot.NodeSeriesAvailable || len(snapshot.Daily) != 0 {
		t.Fatalf("empty per-node series = %+v, want an available zero-usage series", snapshot)
	}
	if !client.start.Equal(moment.UTC()) || !client.end.Equal(moment.UTC()) {
		t.Fatalf("stats range = %s to %s, want %s to %s", client.start, client.end, moment.UTC(), moment.UTC())
	}
	if client.limit != rolloverStatsTopNodesLimit {
		t.Fatalf("topNodesLimit = %d, want %d", client.limit, rolloverStatsTopNodesLimit)
	}
}
