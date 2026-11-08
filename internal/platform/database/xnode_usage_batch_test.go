package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageHistoryBatchSavepointCannotLeakRejectedNeighbor(t *testing.T) {
	s, identity, period, p, at := usageHistoryFixture(t)
	invalid := usageHistoryReport(identity, period, p, at, 1)
	invalid.HourlySamples[0].Owner.ID = "wrong-owner"
	valid := usageHistoryReport(identity, period, p, at, 2)
	items := []xnode.TrafficCommit{
		{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: invalid.ID, Report: invalid},
		{NodeID: "node", AgentID: "agent", Sequence: 2, Digest: valid.ID, Report: valid},
	}
	results, err := s.RecordXNodeTrafficBatch(context.Background(), items)
	if err != nil || len(results) != 2 || !errors.Is(results[0], xnode.ErrInvalid) || results[1] != nil {
		t.Fatal("per-report history isolation failed", results, err)
	}
	rows, _, err := s.XNodeUsageHistory(context.Background(), identity.UserID, at.Add(-time.Minute), at.Add(time.Second))
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if row.BillingMode == "combo" && row.UploadBytes != "3" || row.BillingMode == "hourly" && row.UploadBytes != "7" {
			t.Fatal("rejected neighbor leaked into accepted history", row)
		}
	}
	value, err := s.XNodePeriod(context.Background(), period.ID)
	if err != nil || value.UsedBytes != 8 {
		t.Fatal("savepoint leaked native weighted usage", value, err)
	}
}
