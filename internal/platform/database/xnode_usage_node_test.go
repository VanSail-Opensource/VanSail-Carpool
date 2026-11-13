package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNodeHistoryRetainsRawOnlyAndCoverageOnlyLabels(t *testing.T) {
	for _, raw := range []bool{false, true} {
		s, identity, _ := xnodeTrafficFixture(t)
		ctx := context.Background()
		// Isolate the explicit raw/coverage guard from enrollment/rate evidence.
		if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "history-only", Name: "Retained label", ConfigID: "config", Enabled: true, SettingsCipher: "encrypted", MultiplierFP: xnode.MultiplierScale}); err != nil {
			t.Fatal(err)
		}
		before, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		hadHistory, err := nodeHasHistory(ctx, before, "history-only")
		_ = before.Rollback()
		if err != nil || hadHistory {
			t.Fatal("fixture already had unrelated history", hadHistory, err)
		}
		at := time.Now().UTC()
		if raw {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes) VALUES($1,0,'hourly','old-enrollment',1,'old-period','history-only',$2,$3,$3,1,0)`, identity.UserID, at.Truncate(time.Minute), at); err != nil {
				t.Fatal(err)
			}
		} else if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES('history-only','old-agent','old-stream',0,'known','history-loss',$1,0,1)`, at); err != nil {
			t.Fatal(err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		retained, err := nodeHasHistory(ctx, tx, "history-only")
		_ = tx.Rollback()
		if err != nil || !retained {
			t.Fatal("raw-only history lost its native node label", raw, retained, err)
		}
	}
}

func TestUsageHistoryAndCoverageDoNotWidenFractionalBounds(t *testing.T) {
	s, owner, _ := xnodeTrafficFixture(t)
	ctx, minute := context.Background(), time.Now().UTC().Truncate(time.Minute)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,0,'hourly','retained-enrollment',1,'retained-period','node',$2,$2,$2,17,0)`, owner.UserID, minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly)
 VALUES('node','agent','retained-stream',0,'known','history-loss',$1,0,1)`, minute); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{minute.Add(-time.Nanosecond), minute.Add(time.Nanosecond)} {
		rows, coverage, err := s.XNodeUsageHistory(ctx, owner.UserID, at, at)
		if err != nil || len(rows) != 0 || coverage.GapCount != "0" {
			t.Fatal("parameter rounding widened raw history or gap coverage", at, rows, coverage, err)
		}
	}
	rows, coverage, err := s.XNodeUsageHistory(ctx, owner.UserID, minute, minute)
	if err != nil || len(rows) != 1 || rows[0].UploadBytes != "17" || coverage.GapCount != "1" {
		t.Fatal("exact history/coverage instant was excluded", rows, coverage, err)
	}
}
