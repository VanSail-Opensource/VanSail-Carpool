package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestOfflineTrafficAfterRetirementPreservesHistoricalWeightsAndMoney(t *testing.T) {
	for _, batched := range []bool{false, true} {
		t.Run(map[bool]string{false: "individual", true: "batch"}[batched], func(t *testing.T) {
			s, identity, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			at := time.Now().UTC().Add(-48 * time.Hour)
			// Three historical bytes at 1.25x plus eighty free bytes. New offline
			// reports must add raw bytes without recalculating this committed base.
			if _, err := s.db.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=3 WHERE id=$1`, period.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes,weight_remainder) VALUES($1,'node',$2,$3,83,0,3,750000)`, identity.ProxyUserID, period.ID, at.Format(time.DateOnly)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AdjustBalance(ctx, identity.UserID, 1234, "retirement-funding", "seed", at); err != nil {
				t.Fatal(err)
			}
			ledger := func() string {
				t.Helper()
				var value string
				if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(l) ORDER BY l.id),'[]'::jsonb)::text FROM ledger_entries l WHERE user_id=$1`, identity.UserID).Scan(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			before := ledger()
			item := func(stream string, sequence uint64, up, down uint64) xnode.TrafficCommit {
				id := fmt.Sprintf("agent:%d", sequence)
				if stream != "" {
					id = fmt.Sprintf("agent:%s:%d", stream, sequence)
				}
				return xnode.TrafficCommit{NodeID: "node", AgentID: "agent", Sequence: sequence, Digest: id, Report: wire.TrafficReport{ID: id, StreamID: stream, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: up, DownloadBytes: down}}}}
			}
			first := item("", 1, 2, 1)
			if err := s.RecordXNodeTraffic(ctx, first.NodeID, first.AgentID, first.Sequence, first.Digest, first.Report); err != nil {
				t.Fatal(err)
			}
			stream := "11111111111111111111111111111111"
			if err := s.RegisterXNodeTrafficStream(ctx, "node", "agent", stream); err != nil {
				t.Fatal(err)
			}
			items := []xnode.TrafficCommit{item("", 2, 4, 2), item(stream, 1, 1, 1)}
			for range 2 {
				if batched {
					outcomes, err := s.RecordXNodeTrafficBatch(ctx, items)
					if err != nil || len(outcomes) != len(items) {
						t.Fatal(outcomes, err)
					}
					for _, err := range outcomes {
						if err != nil {
							t.Fatal(err)
						}
					}
				} else {
					for _, entry := range items {
						if err := s.RecordXNodeTraffic(ctx, entry.NodeID, entry.AgentID, entry.Sequence, entry.Digest, entry.Report); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if err := s.RecordXNodeTraffic(ctx, first.NodeID, first.AgentID, first.Sequence, first.Digest, first.Report); err != nil {
				t.Fatal(err)
			}
			value, err := s.XNodePeriod(ctx, period.ID)
			if err != nil || value.UsedBytes != 14 {
				t.Fatal("offline traffic repriced committed usage", value, err)
			}
			var got nativeUsageValue
			if err := s.db.QueryRowContext(ctx, `SELECT upload_bytes,download_bytes,weighted_bytes,weight_remainder FROM xnode_daily_usage WHERE period_id=$1`, period.ID).Scan(&got.upload, &got.download, &got.weighted, &got.remainder); err != nil || got != (nativeUsageValue{90, 4, 14, 750000}) {
				t.Fatal("historical weight or remainder changed", got, err)
			}
			balance, err := s.Balance(ctx, identity.UserID)
			if err != nil || balance.MinorInt64() != 1234 || ledger() != before {
				t.Fatal("traffic changed financial evidence", balance, err)
			}
		})
	}
}
