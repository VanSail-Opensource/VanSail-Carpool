package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownHourlyGroupsMergeEnrollmentsRetainIdleAndAvoidFanout(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	ctx := context.Background()
	for _, group := range []xnode.Group{{ID: "paid-squad", Name: "Named paid squad", Enabled: true}, {ID: "paid-idle", Name: "Named idle squad", Enabled: false}} {
		if err := s.SaveXNodeGroup(ctx, group); err != nil {
			t.Fatal(err)
		}
	}
	first := hourlyBreakdownEnrollment(t, s, u, "first-activation", "paid-squad", at)
	hourlyBreakdownReceipt(t, s, first, "first-hour", 11, at)
	hourlyBreakdownRaw(t, s, first, "shared", 10, 20, at)
	hourlyBreakdownRaw(t, s, first, "another", 30, 40, at)
	usageBreakdownExec(t, s, `UPDATE hourly_enrollments SET status='closed',disabled_at=$2 WHERE id=$1`, first.EnrollmentID, at.Add(time.Minute))
	second := hourlyBreakdownEnrollment(t, s, u, "next-activation", "paid-squad", at.Add(time.Minute))
	hourlyBreakdownReceipt(t, s, second, "next-hour", 5, at.Add(time.Minute))
	hourlyBreakdownRaw(t, s, second, "shared", 3, 4, at.Add(time.Minute))
	// An independent idle settlement and shared extra purchase already have
	// their own immutable facts; no traffic allocation is invented for either.
	usageBreakdownHourlyCharges(t, s, u, "paid-idle", at)
	a := usageBreakdownPAYGAccount(t, s, u, "shared", at)
	usageBreakdownPAYGCharge(t, s, a, "payg-window", 0, 1, 9, at)
	usageBreakdownRaw(t, s, u, "combo", "historical", "shared", 5, 6, at)
	usageBreakdownExec(t, s, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes,download_bytes) VALUES($1,'shared',$2,2,3)`, u.ID, at)
	usageBreakdownExec(t, s, `INSERT INTO hourly_offers(group_id,enabled,hourly_price_minor,revision,created_at,updated_at) VALUES('paid-squad',1,999999,1,$1,$1)`, at)
	var before string
	if err := s.DB().QueryRow(`SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id)::text FROM hourly_settlements s`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	through := at.Add(2 * time.Minute)
	value, err := s.XNodeUsageBreakdown(ctx, u.ID, through.Add(-72*time.Hour), through)
	if err != nil || len(value.Nodes) != 1 || len(value.HourlySquads) != 2 {
		t.Fatal(value, err)
	}
	node := value.Nodes[0]
	if node.NodeID != "shared" || node.UploadBytes != "7" || node.DownloadBytes != "9" || node.CostMinor.Numerator != "9" {
		t.Fatal("hourly bytes leaked into Combo/PAYG node slice", node)
	}
	paid, idle := hourlyBreakdownSquad(t, value, "paid-squad"), hourlyBreakdownSquad(t, value, "paid-idle")
	if paid.Name != "Named paid squad" || paid.UploadBytes != "43" || paid.DownloadBytes != "64" || paid.CostMinor != (model.RationalMinor{Numerator: "16", Denominator: "1"}) {
		t.Fatal("receipt/minute fanout, repricing or closed enrollment loss", paid)
	}
	if idle.Name != "Named idle squad" || idle.UploadBytes != "0" || idle.DownloadBytes != "0" || idle.CostMinor.Numerator != "11" {
		t.Fatal("paid idle squad disappeared", idle)
	}
	if value.Cost.ActualMinor != "61" || value.Cost.UnallocatedMinor.Numerator != "25" || value.PositiveNodes.Traffic != "1" || value.PositiveNodes.Cost != "1" || value.PositiveEntries.Traffic != "2" || value.PositiveEntries.Cost != "3" {
		t.Fatal("actual/unallocated or combined entry counts changed", value)
	}
	var after string
	if err = s.DB().QueryRow(`SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id)::text FROM hourly_settlements s`).Scan(&after); err != nil || after != before {
		t.Fatal("read changed recorded financial evidence", err)
	}
	assertUsageBreakdownConservation(t, value)
}
