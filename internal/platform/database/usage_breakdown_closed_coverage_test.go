package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownClosedCoverageOnlyParticipantsRetainLoss(t *testing.T) {
	for _, mode := range []string{"combo", "payg"} {
		t.Run(mode, func(t *testing.T) {
			s, u, at := usageBreakdownFixture(t)
			ctx := context.Background()
			a := usageBreakdownPAYGAccount(t, s, u, "closed-history", at.Add(-time.Hour))
			if mode == "combo" {
				// Use a primary-member period instead of a PAYG service association.
				i, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: u.ID, CredentialsCipher: "opaque"})
				if err != nil {
					t.Fatal(err)
				}
				p := xnode.Period{ID: "closed-primary", ProxyUserID: i.ProxyUserID, StartedAt: at.Add(-time.Hour), Strategy: "NO_RESET", Status: "open"}
				if err = s.SaveXNodePeriod(ctx, p); err != nil {
					t.Fatal(err)
				}
				if err = s.TrackXNodePeriodNode(ctx, p.ID, a.NodeID); err != nil {
					t.Fatal(err)
				}
				usageBreakdownExec(t, s, `DELETE FROM xnode_payg_accounts WHERE proxy_user_id=$1`, a.ProxyUserID)
			}
			usageBreakdownExec(t, s, `UPDATE xnode_periods SET status='closed',ended_at=$1`, stamp(at.Add(time.Minute)))
			usageBreakdownExec(t, s, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1)`, at.Add(-73*time.Hour))
			usageBreakdownExec(t, s, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES($1,'agent','stream',1,'digest','sample-capacity',$2,1,0)`, a.NodeID, at)
			through := at.Add(2 * time.Minute)
			value, err := s.XNodeUsageBreakdown(ctx, u.ID, through.Add(-72*time.Hour), through)
			if err != nil || len(value.Nodes) != 0 || value.Coverage.GapCount != "1" || value.Coverage.Complete {
				t.Fatal("closed participant erased a loss-only report", value, err)
			}
			// Old-lifecycle participants alone cannot import their shared-node gaps.
			usageBreakdownExec(t, s, `UPDATE users SET created_at=$2 WHERE id=$1`, u.ID, stamp(at.Add(90*time.Second)))
			value, err = s.XNodeUsageBreakdown(ctx, u.ID, through.Add(-72*time.Hour), through)
			if err != nil || value.Coverage.GapCount != "0" {
				t.Fatal("closed pre-lifecycle period imported a gap", value.Coverage, err)
			}
		})
	}
}
