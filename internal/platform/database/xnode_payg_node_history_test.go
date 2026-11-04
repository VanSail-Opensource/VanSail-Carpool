package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestPAYGNodeHistorySharesSavepointsAndReplayDeduplication(t *testing.T) {
	s, owner, _, account := paygBillingFixture(t)
	ctx, at := context.Background(), time.Now().UTC()
	sample := wire.TrafficSample{UserID: int(account.ProxyUserID), AccountingPeriod: account.PeriodID, From: at, Through: at, UploadBytes: 11}
	bad := wire.TrafficSample{UserID: int(owner.ProxyUserID), AccountingPeriod: "missing-history-period", From: at, Through: at, UploadBytes: 1}
	first := xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: "one", Report: wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{sample, bad}}}
	sample.UploadBytes, sample.DownloadBytes = 17, 19
	second := xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: 2, Digest: "two", Report: wire.TrafficReport{ID: "agent:2", Samples: []wire.TrafficSample{sample}}}
	for range 2 {
		outcomes, err := s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{first, second})
		if err != nil || len(outcomes) != 2 || outcomes[0] != nil || outcomes[1] != nil {
			t.Fatal(outcomes, err)
		}
	}
	rows, truncated, err := s.XNodePAYGNodeHistory(ctx, owner.UserID, at.Add(-72*time.Hour), time.Now().UTC())
	if err != nil || truncated || len(rows) != 1 || rows[0].UploadBytes != "17" || rows[0].DownloadBytes != "19" {
		t.Fatal("discarded savepoint or replay leaked directional samples", rows, truncated, err)
	}
	other, _, err := s.XNodePAYGNodeHistory(ctx, "another-owner", at.Add(-72*time.Hour), time.Now().UTC())
	if err != nil || len(other) != 0 {
		t.Fatal("history escaped owner scope", other, err)
	}
}

func TestPAYGHistoryFlushFailureDoesNotCommitChargesOrCursor(t *testing.T) {
	s, owner, _, a := paygBillingFixture(t)
	ctx, at := context.Background(), time.Now().UTC()
	item := xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: "one", Report: wire.TrafficReport{ID: "agent:1", Samples: []wire.TrafficSample{{UserID: int(a.ProxyUserID), AccountingPeriod: a.PeriodID, From: at, Through: at, UploadBytes: uint64(xnode.PAYGStartBytes)}}}}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE xnode_payg_node_minutes ADD CONSTRAINT test_history_failure CHECK(upload_bytes=0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{item}); err == nil {
		t.Fatal("failed history flush was acknowledged")
	}
	if cursor, err := s.XNodeReportSequence(ctx, "agent"); err != nil || cursor != 0 {
		t.Fatal("failed flush advanced the cursor", cursor, err)
	}
	account, err := s.XNodePAYGAccount(ctx, a.ProxyUserID)
	if err != nil || account.BalanceMinor != 1000 || account.UsedBytes != 0 || account.PurchasedBytes != 0 {
		t.Fatal("failed history flush retained financial changes", owner, account, err)
	}
}

func TestPAYGNodeHistoryRetainsAcceptedFutureCaptureMinutes(t *testing.T) {
	s, owner, _, _ := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	sample := wire.TrafficSample{Through: now.Add(2 * time.Minute), UploadBytes: 3, DownloadBytes: 4}
	if err := recordPAYGNodeSampleTx(ctx, tx, owner.UserID, "node", sample, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, _, err := s.XNodePAYGNodeHistory(ctx, owner.UserID, now.Add(-72*time.Hour), now)
	if err != nil || len(rows) != 0 {
		t.Fatal("future capture was shown before its minute", rows, err)
	}
	later := now.Add(3 * time.Minute)
	rows, _, err = s.XNodePAYGNodeHistory(ctx, owner.UserID, later.Add(-72*time.Hour), later)
	if err != nil || len(rows) != 1 || rows[0].UploadBytes != "3" || rows[0].DownloadBytes != "4" {
		t.Fatal("accepted clock skew discarded committed directional usage", rows, err)
	}
}

func TestPAYGNodeHistoryRetainsHistoricalNodesAndPrunesOnlyExpiredSummaries(t *testing.T) {
	s, owner, _, _ := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC().Truncate(time.Minute)
	for _, entry := range []struct {
		node, upload string
		at           time.Time
	}{{"removed-node", "9007199254740993", now}, {"removed-node", "9007199254740993", now.Add(-time.Minute)}, {"expired-node", "5", now.Add(-73 * time.Hour)}} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes,download_bytes) VALUES($1,$2,$3,$4::numeric,7)`, owner.UserID, entry.node, entry.at, entry.upload); err != nil {
			t.Fatal(err)
		}
	}
	rows, truncated, err := s.XNodePAYGNodeHistory(ctx, owner.UserID, now.Add(-72*time.Hour), now)
	if err != nil || truncated || len(rows) < 1 || rows[0].Name != "removed-node" {
		t.Fatal("historical node was omitted", rows, err)
	}
	if _, err := s.PrunePAYGTransientRecords(ctx, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM xnode_payg_node_minutes`).Scan(&count); err != nil || count != 2 {
		t.Fatal("retention removed surviving samples or retained old rows", count, err)
	}
	var nodes []string
	for i := range xnode.MaxPAYGHistoryNodes + 1 {
		nodes = append(nodes, fmt.Sprintf("bounded-node-%03d", i))
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes) SELECT $1,node_id,$2,1 FROM unnest($3::text[]) AS v(node_id)`, owner.UserID, now, nodes); err != nil {
		t.Fatal(err)
	}
	rows, truncated, err = s.XNodePAYGNodeHistory(ctx, owner.UserID, now.Add(-72*time.Hour), now)
	if err != nil || !truncated || len(rows) != xnode.MaxPAYGHistoryNodes {
		t.Fatal("history read exceeded its explicit bound", len(rows), truncated, err)
	}
}
