package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTrafficDiscardRollsBackWholeBatchAndCountsNextBatch(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	valid := wire.TrafficSample{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}
	wrong := valid
	wrong.UserID++
	report := wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{valid, wrong}}
	for range 2 {
		if err := store.RecordXNodeTraffic(ctx, "node", "agent", 1, "first-digest", report); err != nil {
			t.Fatal(err)
		}
	}
	before, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || before.UsedBytes != 0 {
		t.Fatal("discard retained a partial charge", before, err)
	}
	var count int
	var detail string
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MAX(detail) FROM audit_events WHERE action='xnode_traffic_discard'`).Scan(&count, &detail); err != nil || count != 1 {
		t.Fatal("discard audit is not idempotent", count, err)
	}
	var evidence struct {
		Reason    string `json:"reason"`
		Discarded bool   `json:"discardedReport"`
	}
	if err = json.Unmarshal([]byte(detail), &evidence); err != nil || evidence.Reason != "owner_mismatch" || !evidence.Discarded {
		t.Fatal("missing discard evidence", evidence, err)
	}
	report.ID, report.Samples = "agent:2", []wire.TrafficSample{valid}
	if err = store.RecordXNodeTraffic(ctx, "node", "agent", 2, "second-digest", report); err != nil {
		t.Fatal(err)
	}
	after, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || after.UsedBytes != 5 {
		t.Fatal("future valid traffic was not charged", after, err)
	}
}

func TestTrafficGapAuditsMissingRangeWithoutLosingValidReport(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	sample := wire.TrafficSample{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: 4}
	report := wire.TrafficReport{ID: "agent:42", Samples: []wire.TrafficSample{sample}}
	for range 2 {
		if err := store.RecordXNodeTraffic(ctx, "node", "agent", 42, "digest", report); err != nil {
			t.Fatal(err)
		}
	}
	var detail string
	if err := store.DB().QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='xnode_traffic_discard'`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Reason, MissingFrom, MissingThrough string
		DiscardedReport                     bool
	}
	if err := json.Unmarshal([]byte(detail), &evidence); err != nil || evidence.Reason != "sequence_gap" || evidence.MissingFrom != "1" || evidence.MissingThrough != "41" || evidence.DiscardedReport {
		t.Fatal("gap audit does not match the missing range", evidence, err)
	}
	got, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || got.UsedBytes != 5 {
		t.Fatal("valid report was lost or double charged", got, err)
	}
}
