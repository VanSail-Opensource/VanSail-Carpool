package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownHourlyCoverageRetainsClosedLossAndRejectsUnrelatedGaps(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	ctx := context.Background()
	current := hourlyBreakdownEnrollment(t, s, u, "closed-in-window", "current-group", at)
	old := hourlyBreakdownEnrollment(t, s, u, "closed-before-window", "old-group", at.Add(-100*time.Hour))
	for _, item := range []struct {
		p     xnode.HourlyPrincipal
		node  string
		ended time.Time
	}{{current, "retained-gap", at.Add(time.Minute)}, {old, "old-gap", at.Add(-80 * time.Hour)}} {
		if err := s.SaveXNodeNode(ctx, xnode.Node{ID: item.node, Name: item.node, ConfigID: "breakdown-config", SettingsCipher: "opaque"}); err != nil {
			t.Fatal(err)
		}
		if err := s.TrackXNodePeriodNode(ctx, item.p.PeriodID, item.node); err != nil {
			t.Fatal(err)
		}
		usageBreakdownExec(t, s, `UPDATE hourly_enrollments SET status='closed',disabled_at=$2 WHERE id=$1`, item.p.EnrollmentID, item.ended)
		usageBreakdownExec(t, s, `UPDATE xnode_periods SET status='closed',ended_at=$2 WHERE id=$1`, item.p.PeriodID, stamp(item.ended))
	}
	other := createTestUser(t, s, 91002)
	foreign := hourlyBreakdownEnrollment(t, s, other, "foreign-coverage", "foreign-group", at)
	for _, item := range []struct {
		p         xnode.HourlyPrincipal
		node      string
		principal int64
	}{{foreign, "forged-owner-gap", foreign.ProxyUserID}, {current, "forged-principal-gap", current.ProxyUserID + 1}} {
		usageBreakdownExec(t, s, `INSERT INTO xnode_usage_minutes(owner_id,account_generation,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,0,'hourly',$2,$3,$4,$5,$6,$6,$6,1,0)`, u.ID, item.p.EnrollmentID, item.principal, item.p.PeriodID, item.node, at)
	}
	for _, node := range []string{"retained-gap", "old-gap", "forged-owner-gap", "forged-principal-gap"} {
		usageBreakdownExec(t, s, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES($1,'agent','stream',1,'digest','sample-capacity',$2,0,1)`, node, at)
	}
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1)`, at.Add(-73*time.Hour))
	through := at.Add(2 * time.Minute)
	value, err := s.XNodeUsageBreakdown(ctx, u.ID, through.Add(-72*time.Hour), through)
	if err != nil || len(value.Nodes) != 0 || len(value.HourlySquads) != 0 || value.Coverage.GapCount != "1" || value.Coverage.Complete {
		t.Fatal("closed hourly loss-only evidence or owner scope changed", value, err)
	}
	usageBreakdownExec(t, s, `DELETE FROM xnode_usage_coverage WHERE node_id='retained-gap'`)
	value, err = s.XNodeUsageBreakdown(ctx, u.ID, through.Add(-72*time.Hour), through)
	if err != nil || value.Coverage.GapCount != "0" || !value.Coverage.Complete {
		t.Fatal("old or malformed hourly references poisoned current coverage", value.Coverage, err)
	}
}
