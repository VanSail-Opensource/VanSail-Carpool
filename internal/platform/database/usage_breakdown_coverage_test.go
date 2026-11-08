package database

import (
	"context"
	"testing"
	"time"
)

func TestUsageBreakdownCoverageUsesAcceptedFloorAndOwnerNodes(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	ctx := context.Background()
	usageBreakdownRaw(t, s, u, "combo", "retained", "relevant", 1, 0, at)
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1)`, at.Add(-73*time.Hour))
	for _, node := range []string{"relevant", "other-owner"} {
		usageBreakdownExec(t, s, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES($1,'agent','stream',1,'digest','sample-capacity',$2,0,1)`, node, at)
	}
	value, err := s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at)
	if err != nil || value.Coverage.Since == nil || value.Coverage.GapCount != "1" || value.Coverage.Complete || value.Coverage.Truncated {
		t.Fatal(value.Coverage, err)
	}
	usageBreakdownExec(t, s, `DELETE FROM xnode_usage_coverage WHERE node_id='relevant'`)
	value, err = s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at)
	if err != nil || !value.Coverage.Complete || value.Coverage.GapCount != "0" {
		t.Fatal("foreign gaps poisoned owner coverage", value.Coverage, err)
	}
	usageBreakdownExec(t, s, `UPDATE xnode_usage_history_state SET started_at=$1`, at.Add(-time.Hour))
	value, err = s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at)
	if err != nil || value.Coverage.Complete || value.Coverage.Since == nil || !value.Coverage.Since.Equal(at.Add(-time.Hour)) {
		t.Fatal("writer coverage was backfilled", value.Coverage, err)
	}
}

func TestUsageBreakdownFirstLifecycleMinuteKeepsKnownRawAndOmitsUnsplitPAYG(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	ctx := context.Background()
	lifecycle := at.Add(20 * time.Second)
	usageBreakdownExec(t, s, `UPDATE users SET created_at=$2 WHERE id=$1`, u.ID, stamp(lifecycle))
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1)`, at.Add(-73*time.Hour))
	usageBreakdownRaw(t, s, u, "combo", "known-new", "first-minute", 3, 4, at.Add(25*time.Second))
	usageBreakdownRaw(t, s, u, "combo", "known-old", "old-minute", 999, 0, at.Add(10*time.Second))
	usageBreakdownExec(t, s, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes,download_bytes) VALUES($1,'partial',$2,999,0),($1,'complete',$3,5,6)`, u.ID, at, at.Add(time.Minute))
	a := usageBreakdownPAYGAccount(t, s, u, "cost-in-partial", lifecycle)
	usageBreakdownPAYGCharge(t, s, a, "precise-cost", 0, 1, 7, lifecycle.Add(time.Second))
	value, err := s.XNodeUsageBreakdown(ctx, u.ID, lifecycle, at.Add(2*time.Minute))
	if err != nil || len(value.Nodes) != 3 || value.Coverage.Complete || value.Coverage.GapCount != "0" || value.Coverage.Truncated {
		t.Fatal(value, err)
	}
	if value.Coverage.Since == nil || !value.Coverage.Since.Equal(at.Add(time.Minute)) {
		t.Fatal("missing partial bucket claimed complete", value.Coverage)
	}
	if node := usageBreakdownNode(t, value, "first-minute"); node.UploadBytes != "3" || node.DownloadBytes != "4" {
		t.Fatal("known observation bounds were discarded", node)
	}
	if node := usageBreakdownNode(t, value, "complete"); node.UploadBytes != "5" || node.DownloadBytes != "6" {
		t.Fatal(node)
	}
	if value.Cost.ActualMinor != "7" {
		t.Fatal("money was rounded to bucket boundaries", value.Cost)
	}
	value, err = s.XNodeUsageBreakdown(ctx, u.ID, at.Add(time.Minute), at.Add(2*time.Minute))
	if err != nil || !value.Coverage.Complete || len(value.Nodes) != 1 || value.Nodes[0].NodeID != "complete" {
		t.Fatal("full lifecycle minute did not recover coverage", value, err)
	}
}
