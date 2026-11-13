package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTrafficItemHistoryFailurePreservesReplayableCursorAndCharges(t *testing.T) {
	for _, stream := range []string{"", "11111111111111111111111111111111"} {
		name := "legacy"
		if stream != "" {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			s, parent, combo, account := paygBillingFixture(t)
			ctx := context.Background()
			id := "agent:1"
			if stream != "" {
				if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", stream); err != nil {
					t.Fatal(err)
				}
				id = "agent:" + stream + ":1"
			}
			at := time.Now().UTC()
			item := xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: 1, Digest: id,
				Report: wire.TrafficReport{ID: id, StreamID: stream, Samples: []wire.TrafficSample{
					{UserID: int(account.ProxyUserID), AccountingPeriod: account.PeriodID, From: at, Through: at, UploadBytes: uint64(xnode.PAYGStartBytes)},
					{UserID: int(parent.ProxyUserID), AccountingPeriod: combo.ID, From: at, Through: at, UploadBytes: 1},
				}}}
			if _, err := s.DB().ExecContext(ctx, `CREATE FUNCTION reject_history_start() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test history start failure' USING ERRCODE='23514'; END $$`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().ExecContext(ctx, `CREATE TRIGGER reject_history_start BEFORE INSERT ON xnode_usage_history_state FOR EACH ROW EXECUTE FUNCTION reject_history_start()`); err != nil {
				t.Fatal(err)
			}
			outcomes, err := s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{item})
			if err != nil || len(outcomes) != 1 || outcomes[0] == nil {
				t.Fatal("history failure did not remain an isolated failed item", outcomes, err)
			}
			var sequence uint64
			if stream == "" {
				sequence, err = s.XNodeReportSequence(ctx, "agent")
			} else {
				sequence, _, err = s.XNodeStreamSequence(ctx, "agent", stream)
			}
			if err != nil || sequence != 0 {
				t.Fatal("rolled-back report advanced its durable cursor", sequence, err)
			}
			current, err := s.XNodePAYGAccount(ctx, account.ProxyUserID)
			if err != nil || current.UsedBytes != 0 || current.PurchasedBytes != 0 || current.BalanceMinor != 1000 {
				t.Fatal("rolled-back report retained financial mutation", current, err)
			}
			period, err := s.XNodePeriod(ctx, combo.ID)
			if err != nil || period.UsedBytes != 0 {
				t.Fatal("rolled-back report retained staged quota", period, err)
			}
			if _, err = s.DB().ExecContext(ctx, `DROP TRIGGER reject_history_start ON xnode_usage_history_state`); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				outcomes, err = s.RecordXNodeTrafficBatch(ctx, []xnode.TrafficCommit{item})
				if err != nil || len(outcomes) != 1 || outcomes[0] != nil {
					t.Fatal("replay after storage recovery failed", outcomes, err)
				}
			}
			current, err = s.XNodePAYGAccount(ctx, account.ProxyUserID)
			if err != nil || current.UsedBytes != xnode.PAYGStartBytes || current.PurchasedBytes != xnode.PAYGGB || current.BalanceMinor != 900 {
				t.Fatal("replay lost or repeated the recovered financial fact", current, err)
			}
			period, err = s.XNodePeriod(ctx, combo.ID)
			if err != nil || period.UsedBytes != 1 {
				t.Fatal("replay lost or repeated the recovered quota", period, err)
			}
			var charges int
			if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='payg_gb' AND user_id=$1`, parent.UserID).Scan(&charges); err != nil || charges != 1 {
				t.Fatal("recovery did not retain exactly one debit", charges, err)
			}
		})
	}
}
