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

func TestBootStreamsSeparateReplayRestartGapsAndLegacyAccounting(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	first, second := "11111111111111111111111111111111", "22222222222222222222222222222222"
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", first); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	report := func(stream string, sequence uint64, bytes uint64) wire.TrafficReport {
		id := fmt.Sprintf("agent:%s:%d", stream, sequence)
		if stream == "" {
			id = fmt.Sprintf("agent:%d", sequence)
		}
		return wire.TrafficReport{ID: id, StreamID: stream, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: bytes}}}
	}
	commit := func(batch wire.TrafficReport, digest string) {
		t.Helper()
		sequence, err := wire.TrafficSequence("agent", batch)
		if err == nil {
			err = s.RecordXNodeTraffic(ctx, "node", "agent", sequence, digest, batch)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	commit(report(first, 1, 4), "first")
	commit(report(first, 1, 100), "conflicting")
	commit(report(first, 3, 4), "gap")
	commit(report(first, 2, 100), "late")
	loss := report(first, 4, 0)
	loss.DroppedSamples = 12
	commit(loss, "loss")
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", second); err != nil {
		t.Fatal(err)
	}
	commit(report(first, 5, 100), "closed")
	commit(report(second, 1, 4), "new-boot")
	commit(report("", 1, 4), "legacy")
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", first); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("replaced boot reopened", err)
	}
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", second); err != nil {
		t.Fatal("reconnect lost current stream", err)
	}
	if err := s.SetXNodeStreamRate(ctx, "node", "agent", second, 1, 2*xnode.MultiplierScale); err != nil {
		t.Fatal(err)
	}
	commit(report(second, 2, 4), "new-rate")
	commit(report(second, 1, 4), "new-boot")
	value, err := s.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 20 {
		t.Fatal("replay/restart repriced or fabricated usage", value.UsedBytes, err)
	}
	sequence, closed, err := s.XNodeStreamSequence(ctx, "agent", first)
	if err != nil || sequence != 4 || !closed {
		t.Fatal("closed cursor changed", sequence, closed, err)
	}
	sequence, closed, err = s.XNodeStreamSequence(ctx, "agent", second)
	if err != nil || sequence != 2 || closed {
		t.Fatal("boot sequence collided", sequence, closed, err)
	}
	legacy, err := s.XNodeReportSequence(ctx, "agent")
	if err != nil || legacy != 1 {
		t.Fatal("legacy identity lost", legacy, err)
	}
	var gaps bool
	if err := s.db.QueryRowContext(ctx, `SELECT has_gaps FROM xnode_traffic_streams WHERE agent_id='agent' AND stream_id=$1`, first).Scan(&gaps); err != nil || !gaps {
		t.Fatal("loss evidence missing", err)
	}
	third := "33333333333333333333333333333333"
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", third); err != nil {
		t.Fatal(err)
	}
	if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
		t.Fatal(err)
	}
	period, err = s.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	barrier := wire.TrafficBarrier{AgentID: "agent", StreamID: second, Sequence: 99, LegacySequence: 1, AccountingPeriod: period.ID, Quiesced: true}
	if err := s.SealXNodeClosingPeriodNode(ctx, period, "node", barrier); err != nil {
		t.Fatal(err)
	}
	participants, err := s.XNodePeriodNodes(ctx, period.ID)
	if err != nil || len(participants) != 1 || participants[0].Sequence != 2 {
		t.Fatal("settlement invented acknowledgement of lost reports", participants, err)
	}
}

func TestStreamCursorAndUsageRollbackTogether(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	stream := "11111111111111111111111111111111"
	if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", stream); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	batch := wire.TrafficReport{ID: "agent:" + stream + ":1", StreamID: stream, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 16}}}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage ADD CONSTRAINT test_storage_failure CHECK(upload_bytes<10)`); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordXNodeTraffic(ctx, "node", "agent", 1, "same", batch); err == nil {
		t.Fatal("storage failure accepted")
	}
	sequence, _, err := s.XNodeStreamSequence(ctx, "agent", stream)
	value, usageErr := s.XNodePeriod(ctx, period.ID)
	if err != nil || usageErr != nil || sequence != 0 || value.UsedBytes != 0 {
		t.Fatal("failed commit changed accounting", sequence, value.UsedBytes, err, usageErr)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_daily_usage DROP CONSTRAINT test_storage_failure`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.RecordXNodeTraffic(ctx, "node", "agent", 1, "same", batch); err != nil {
			t.Fatal(err)
		}
	}
	value, err = s.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 16 {
		t.Fatal("retried transaction duplicated usage", value.UsedBytes, err)
	}
}
