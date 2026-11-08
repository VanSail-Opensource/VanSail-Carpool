package database

import (
	"context"
	"testing"
	"time"
)

func TestDroppedHourlyOnlyReportPersistsCoverageBeforeAcknowledgment(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	report := usageHistoryReport(identity, period, p, at, 1)
	report.Samples, report.HourlySamples = nil, nil
	report.DroppedHourlySamples = 3
	for range 2 {
		if err := commitUsageHistoryFixture(t, s, report, true); err != nil {
			t.Fatal(err)
		}
	}
	rows, coverage, err := s.XNodeUsageHistory(context.Background(), identity.UserID, at.Add(-time.Minute), time.Now().Add(time.Second))
	if err != nil || len(rows) != 0 || coverage.Since == nil || coverage.Complete || coverage.GapCount != "1" {
		t.Fatal("drop-only coverage missing, doubled or fabricated traffic", rows, coverage, err)
	}
	var dropped string
	if err = s.db.QueryRow(`SELECT dropped_hourly::text FROM xnode_usage_coverage`).Scan(&dropped); err != nil || dropped != "3" {
		t.Fatal("original drop count was not retained", dropped, err)
	}
	var gaps bool
	if err = s.db.QueryRow(`SELECT has_gaps FROM xnode_traffic_streams`).Scan(&gaps); err != nil || !gaps {
		t.Fatal("hourly loss did not mark journal coverage", gaps, err)
	}
}

func TestUsageCoverageCanRetainAnEmptyReplacedBoot(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	for _, boot := range []string{usageHistoryStream, "22222222222222222222222222222222"} {
		if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", boot); err != nil {
			t.Fatal("zero-cursor boot closure failed", err)
		}
	}
	var sequence int64
	if err := s.db.QueryRow(`SELECT sequence FROM xnode_usage_coverage`).Scan(&sequence); err != nil || sequence != 0 {
		t.Fatal("coverage fabricated a positive report sequence", sequence, err)
	}
}
