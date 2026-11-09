package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTrafficBatchIsolatesDiscardedAndMalformedReports(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	stream := "11111111111111111111111111111111"
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", stream); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	item := func(sequence uint64, upload uint64) xnode.TrafficCommit {
		id := fmt.Sprintf("agent:%s:%d", stream, sequence)
		return xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: sequence, Digest: id, Report: wire.TrafficReport{ID: id, StreamID: stream, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: upload}}}}
	}
	conflict := item(2, 100)
	conflict.Digest = "different"
	discard := item(3, 100)
	discard.Report.Samples = append(discard.Report.Samples, wire.TrafficSample{UserID: int(identity.ProxyUserID) + 1, AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 1})
	malformed := item(6, 1)
	malformed.Report.Samples[0].UserID = -1
	// Sorting preserves the first receipt for equal sequences. Report 3 must not
	// leave its first sample in the shared Go buffer.
	items := []xnode.TrafficCommit{item(5, 1), item(1, 1), item(2, 1), conflict, discard, item(4, 1), malformed, item(7, 1)}
	outcomes, err := s.RecordXNodeTrafficBatch(ctx, items)
	if err != nil || len(outcomes) != len(items) {
		t.Fatal(outcomes, err)
	}
	for i, outcome := range outcomes {
		if i == 6 {
			if !errors.Is(outcome, xnode.ErrInvalid) {
				t.Fatal("malformed report accepted", outcome)
			}
		} else if outcome != nil {
			t.Fatal(i, outcome)
		}
	}
	value, err := s.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 5 {
		t.Fatal("failed or duplicate report changed usage", value.UsedBytes, err)
	}
	var upload, weighted, remainder int64
	if err := s.db.QueryRowContext(ctx, `SELECT upload_bytes,weighted_bytes,weight_remainder FROM xnode_daily_usage WHERE period_id=$1`, period.ID).Scan(&upload, &weighted, &remainder); err != nil || upload != 5 || weighted != 5 || remainder != 0 {
		t.Fatal("combined write changed raw accounting", upload, weighted, remainder, err)
	}
	sequence, closed, err := s.XNodeStreamSequence(ctx, "agent", stream)
	if err != nil || closed || sequence != 7 {
		t.Fatal("committed cursor disagrees with usage", sequence, closed, err)
	}
	if _, err := s.RecordXNodeTrafficBatch(ctx, items); err != nil {
		t.Fatal(err)
	}
	replayed, err := s.XNodePeriod(ctx, period.ID)
	if err != nil || replayed.UsedBytes != value.UsedBytes {
		t.Fatal("batch replay charged again", replayed.UsedBytes, err)
	}
}

func TestCombinedTrafficWriteFailureRollsBackEveryCursor(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	var items []xnode.TrafficCommit
	for i, bytes := range []uint64{1, 16} {
		id := fmt.Sprintf("agent:%d", i+1)
		items = append(items, xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: uint64(i + 1), Digest: id, Report: wire.TrafficReport{ID: id, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: bytes}}}})
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage ADD CONSTRAINT test_batch_failure CHECK(upload_bytes<10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordXNodeTrafficBatch(ctx, items); err == nil {
		t.Fatal("combined storage failure committed")
	}
	sequence, err := s.XNodeReportSequence(ctx, "agent")
	value, usageErr := s.XNodePeriod(ctx, period.ID)
	if err != nil || usageErr != nil || sequence != 0 || value.UsedBytes != 0 {
		t.Fatal("failed write left partial accounting", sequence, value.UsedBytes, err, usageErr)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage DROP CONSTRAINT test_batch_failure`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		outcomes, err := s.RecordXNodeTrafficBatch(ctx, items)
		if err != nil || len(outcomes) != 2 || outcomes[0] != nil || outcomes[1] != nil {
			t.Fatal(outcomes, err)
		}
	}
	value, err = s.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 17 {
		t.Fatal("retry lost or duplicated usage", value.UsedBytes, err)
	}
}
